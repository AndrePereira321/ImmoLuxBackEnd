package server

// End-to-end tests of the real server — middleware, routes and stores —
// against PostgreSQL. Skipped unless IMMOLUX_TEST_DATABASE_URL names a
// dedicated database whose name contains the word "test"; every test empties
// all of its tables (see internal/database/testdb_test.go).

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"immo-lux/internal/config"
	"immo-lux/internal/database/ent/client/migrate"
	"immo-lux/internal/models"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v3"
)

const (
	testDatabaseEnv = "IMMOLUX_TEST_DATABASE_URL"
	// testDatabaseLockKey serialises tests on the shared test database across
	// packages; internal/database tests use the same key.
	testDatabaseLockKey = 72746855

	testAdminEmail    = "admin@test.local"
	testAdminPassword = "correct-horse-battery"
)

var testNamePattern = regexp.MustCompile(`(?i)(^|[^a-z])test([^a-z]|$)`)

type integrationServer struct {
	baseURL string
	sqlDB   *sql.DB
}

func newIntegrationServer(t *testing.T) *integrationServer {
	t.Helper()
	dsn := os.Getenv(testDatabaseEnv)
	if dsn == "" {
		t.Skipf("%s is not set; skipping PostgreSQL integration test", testDatabaseEnv)
	}
	dbURL, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("invalid %s: %v", testDatabaseEnv, err)
	}
	dbName := strings.TrimPrefix(dbURL.Path, "/")
	if !testNamePattern.MatchString(dbName) {
		t.Skipf("HTTP integration tests need a dedicated database named with the word \"test\", got %q", dbName)
	}
	ctx := context.Background()

	// Held until every other cleanup (server shutdown, DB close) has run.
	lockDB, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	lockConn, err := lockDB.Conn(ctx)
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	if _, err := lockConn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", testDatabaseLockKey); err != nil {
		t.Fatalf("failed to lock test database: %v", err)
	}
	t.Cleanup(func() {
		_, _ = lockConn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", testDatabaseLockKey)
		_ = lockConn.Close()
		_ = lockDB.Close()
	})

	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	port := 5432
	if p := dbURL.Port(); p != "" {
		if port, err = strconv.Atoi(p); err != nil {
			t.Fatalf("invalid port in %s: %v", testDatabaseEnv, err)
		}
	}
	password, _ := dbURL.User.Password()
	sslMode := dbURL.Query().Get("sslmode")
	if sslMode == "" {
		sslMode = "disable"
	}

	usersFile := filepath.Join(t.TempDir(), "users.json")
	users := fmt.Sprintf(`[{"firstName":"Test","lastName":"Admin","email":%q,"password":%q}]`, testAdminEmail, testAdminPassword)
	if err := os.WriteFile(usersFile, []byte(users), 0600); err != nil {
		t.Fatalf("failed to write users file: %v", err)
	}

	serverConfig, err := config.GetServerConfig([]byte(fmt.Sprintf(`
[app]
name="ImmoLux"
version="1.0.0"
[server]
host="127.0.0.1"
origin=%q
proxy_header="X-Forwarded-For"
[logging]
server_log_level="disabled"
database_log_level="disabled"
[security]
jwt_secret="integration-secret-that-is-at-least-32-chars"
[database]
host=%q
port=%d
username=%q
password=%q
db_name=%q
ssl_mode=%q
user_file_path=%q
`, testOrigin, dbURL.Hostname(), port, dbURL.User.Username(), password, dbName, sslMode, filepath.ToSlash(usersFile))))
	if err != nil {
		t.Fatalf("invalid test config: %v", err)
	}

	srv, err := New(serverConfig) // migrates, seeds the admin and starts maintenance
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })

	// Start every test from empty tables, then seed the admin again.
	tableNames := make([]string, 0, len(migrate.Tables))
	for _, table := range migrate.Tables {
		tableNames = append(tableNames, `"`+table.Name+`"`)
	}
	if _, err := sqlDB.ExecContext(ctx, "TRUNCATE "+strings.Join(tableNames, ", ")+" RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("failed to truncate test database: %v", err)
	}
	if err := srv.db.Init(); err != nil {
		t.Fatalf("failed to seed users: %v", err)
	}

	srv.RegisterRoutes()
	return &integrationServer{baseURL: startLoopbackServer(t, srv.fiber), sqlDB: sqlDB}
}

type apiResult struct {
	status        int
	sessionCookie string
	success       bool
	data          json.RawMessage
	errorCode     string
}

func (s *integrationServer) do(t *testing.T, method, path, contentType string, body io.Reader, clientIP, sessionCookie string) apiResult {
	t.Helper()
	req, err := http.NewRequest(method, s.baseURL+path, body)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if clientIP != "" {
		req.Header.Set("X-Forwarded-For", clientIP)
	}
	if sessionCookie != "" {
		req.AddCookie(&http.Cookie{Name: "session_token", Value: sessionCookie})
	}
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("%s %s failed: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	result := apiResult{status: resp.StatusCode}
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "session_token" {
			result.sessionCookie = cookie.Value
		}
	}
	var envelope struct {
		Success bool                   `json:"success"`
		Data    json.RawMessage        `json:"data"`
		Error   *models.ServerAPIError `json:"error"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("%s %s: response is not the API envelope (%d): %q", method, path, resp.StatusCode, raw)
	}
	result.success, result.data = envelope.Success, envelope.Data
	if envelope.Error != nil {
		result.errorCode = envelope.Error.Code
	}
	return result
}

func (s *integrationServer) login(t *testing.T, clientIP, password string) apiResult {
	t.Helper()
	body := fmt.Sprintf(`{"email":%q,"password":%q}`, testAdminEmail, password)
	return s.do(t, fiber.MethodPost, ApiPrefix+"/login", "application/json", strings.NewReader(body), clientIP, "")
}

func requireResult(t *testing.T, what string, got apiResult, status int, errorCode string) {
	t.Helper()
	if got.status != status || got.errorCode != errorCode {
		t.Fatalf("%s: expected %d %q, got %d %q", what, status, errorCode, got.status, got.errorCode)
	}
}

func TestLoginFlowEndToEnd(t *testing.T) {
	s := newIntegrationServer(t)
	const attacker, user = "203.0.113.50", "198.51.100.7"

	for i := 1; i <= 5; i++ {
		requireResult(t, fmt.Sprintf("wrong password %d", i), s.login(t, attacker, "wrong-password"), 401, "INVALID_CREDENTIALS")
	}
	// The database limit now blocks this client for the account...
	requireResult(t, "6th attempt", s.login(t, attacker, "wrong-password"), 429, "RATE_LIMIT_EXCEEDED")
	requireResult(t, "blocked client, right password", s.login(t, attacker, testAdminPassword), 429, "RATE_LIMIT_EXCEEDED")

	// ...but, with real client IPs, not every other client.
	loggedIn := s.login(t, user, testAdminPassword)
	requireResult(t, "other client login", loggedIn, 200, "")
	if loggedIn.sessionCookie == "" {
		t.Fatal("login did not set the session cookie")
	}

	status := s.do(t, fiber.MethodGet, ApiPrefix+"/isconnected", "", nil, user, loggedIn.sessionCookie)
	var connected struct {
		IsConnected bool `json:"isConnected"`
	}
	if err := json.Unmarshal(status.data, &connected); err != nil || !connected.IsConnected {
		t.Fatalf("expected isConnected after login, got %s (err %v)", status.data, err)
	}

	sessions := s.do(t, fiber.MethodGet, ApiPrefix+"/sessions", "", nil, user, loggedIn.sessionCookie)
	var sessionList struct {
		Sessions []struct {
			IpAddress *string `json:"ipAddress"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal(sessions.data, &sessionList); err != nil || len(sessionList.Sessions) != 1 {
		t.Fatalf("expected one session, got %s (err %v)", sessions.data, err)
	}
	if ip := sessionList.Sessions[0].IpAddress; ip == nil || *ip != user {
		t.Errorf("session should record the real client IP %s, got %v", user, ip)
	}

	// The database saw real client IPs, never the proxy's address.
	var rateLimitIPs string
	if err := s.sqlDB.QueryRow(`SELECT coalesce(string_agg(ip_address || ':' || attempt_count, ',' ORDER BY ip_address), '') FROM rate_limits`).Scan(&rateLimitIPs); err != nil {
		t.Fatalf("failed to query rate limits: %v", err)
	}
	if rateLimitIPs != attacker+":5" {
		t.Errorf("expected only the attacker's rate-limit record (the user's is reset on success), got %q", rateLimitIPs)
	}
	var failedFromAttacker, successFromUser int
	if err := s.sqlDB.QueryRow(`SELECT
		count(*) FILTER (WHERE event_type = 'LOGIN_FAILED' AND ip_address = $1),
		count(*) FILTER (WHERE event_type = 'LOGIN_SUCCESS' AND ip_address = $2)
		FROM auth_logs`, attacker, user).Scan(&failedFromAttacker, &successFromUser); err != nil {
		t.Fatalf("failed to query auth logs: %v", err)
	}
	if failedFromAttacker != 5 || successFromUser != 1 {
		t.Errorf("expected 5 failed logins from %s and 1 success from %s, got %d and %d", attacker, user, failedFromAttacker, successFromUser)
	}

	requireResult(t, "logout", s.do(t, fiber.MethodPost, ApiPrefix+"/logout", "", nil, user, loggedIn.sessionCookie), 200, "")
	status = s.do(t, fiber.MethodGet, ApiPrefix+"/isconnected", "", nil, user, loggedIn.sessionCookie)
	if err := json.Unmarshal(status.data, &connected); err != nil || connected.IsConnected {
		t.Errorf("the session must be invalid after logout, got %s", status.data)
	}
}

func TestLoginWithUnparseableForwardedIP(t *testing.T) {
	s := newIntegrationServer(t)
	// Fiber accepts this as an IPv6 address, but it is 63 characters long — over
	// the 45-character ip_address columns. Login must still work.
	loggedIn := s.login(t, strings.Repeat("0", 60)+"::1", testAdminPassword)
	requireResult(t, "login", loggedIn, 200, "")

	var sessionIP string
	if err := s.sqlDB.QueryRow(`SELECT ip_address FROM sessions`).Scan(&sessionIP); err != nil {
		t.Fatalf("failed to query session: %v", err)
	}
	if sessionIP != "127.0.0.1" {
		t.Errorf("expected the peer address as fallback, got %q", sessionIP)
	}
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for x := 0; x < 16; x++ {
		for y := 0; y < 16; y++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 16), G: uint8(y * 16), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to encode test PNG: %v", err)
	}
	return buf.Bytes()
}

func TestImageCapOverHTTP(t *testing.T) {
	s := newIntegrationServer(t)
	const client = "198.51.100.9"
	session := s.login(t, client, testAdminPassword)
	requireResult(t, "login", session, 200, "")

	contact := s.do(t, fiber.MethodPost, ApiPrefix+"/contacts", "application/json",
		strings.NewReader(`{"name":"Agent","email":"agent@test.local","phone":"+351 900 000 000"}`), client, session.sessionCookie)
	requireResult(t, "create contact", contact, 200, "")
	var contactDTO struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(contact.data, &contactDTO); err != nil || contactDTO.ID <= 0 {
		t.Fatalf("unexpected contact response %s (err %v)", contact.data, err)
	}

	property := fmt.Sprintf(`{"title":"Casa de teste","description":"Integration test property","propertyType":"house",
		"price":250000,"status":"available","address":"Rua de Teste 1","district":"Porto","municipality":"Porto","country":"PT",
		"contacts":[{"id":%d}]}`, contactDTO.ID)
	created := s.do(t, fiber.MethodPost, ApiPrefix+"/properties", "application/json", strings.NewReader(property), client, session.sessionCookie)
	requireResult(t, "create property", created, 200, "")
	var dto struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(created.data, &dto); err != nil || dto.ID <= 0 {
		t.Fatalf("unexpected create response %s (err %v)", created.data, err)
	}

	pngData := testPNG(t)
	upload := func() apiResult {
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		part, err := form.CreateFormFile("image", "photo.png")
		if err != nil {
			t.Fatalf("failed to build form: %v", err)
		}
		_, _ = part.Write(pngData)
		_ = form.Close()
		return s.do(t, fiber.MethodPost, fmt.Sprintf("%s/properties/%d/images", ApiPrefix, dto.ID),
			form.FormDataContentType(), &body, client, session.sessionCookie)
	}

	for i := 1; i <= 20; i++ {
		requireResult(t, fmt.Sprintf("upload %d", i), upload(), 200, "")
	}
	requireResult(t, "upload 21", upload(), 400, "IMAGE_LIMIT_REACHED")

	images := s.do(t, fiber.MethodGet, fmt.Sprintf("%s/properties/%d/images", ApiPrefix, dto.ID), "", nil, client, "")
	var gallery struct {
		Images []json.RawMessage `json:"images"`
	}
	if err := json.Unmarshal(images.data, &gallery); err != nil || len(gallery.Images) != 20 {
		t.Fatalf("expected 20 images in the gallery, got %s (err %v)", images.data, err)
	}
}
