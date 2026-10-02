package server

import (
	"fmt"
	"immo-lux/internal/config"
	"immo-lux/internal/logger"
	"immo-lux/internal/models"
	"immo-lux/internal/server/routes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v3"
)

const testOrigin = "http://localhost:8080"

func newTestServerConfig(t *testing.T, proxyHeader string) *config.ServerConfig {
	t.Helper()
	toml := fmt.Sprintf(`
[app]
name="ImmoLux"
version="1.0.0"

[server]
host="127.0.0.1"
origin=%q
proxy_header=%q

[security]
jwt_secret="test-secret-that-is-at-least-32-characters"

[database]
host="localhost"
username="test"
password="test"
db_name="test"
`, testOrigin, proxyHeader)
	serverConfig, err := config.GetServerConfig([]byte(toml))
	if err != nil {
		t.Fatalf("invalid test config: %v", err)
	}
	return serverConfig
}

// newTestApp builds the production Fiber app (middleware included) and adds
// handlers that echo the resolved client IP in place of the real routes.
func newTestApp(t *testing.T, proxyHeader string) *fiber.App {
	t.Helper()
	testLogger, err := logger.New("TEST", "disabled", "", true)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	t.Cleanup(func() { _ = testLogger.Close() })

	app := getFiberApp(newTestServerConfig(t, proxyHeader), testLogger)
	echoIP := func(ctx fiber.Ctx) error { return ctx.SendString(routes.ClientIP(ctx)) }
	app.Post(ApiPrefix+"/login", echoIP)
	app.Get(ApiPrefix+"/login", echoIP)
	app.Post(ApiPrefix+"/logout", echoIP)
	app.Get(ApiPrefix+"/ip", echoIP)
	return app
}

type testResponse struct {
	status int
	header http.Header
	body   string
}

// doTest sends a request through app.Test, whose fake connection comes from
// 0.0.0.0 — an address that is never a trusted proxy.
func doTest(t *testing.T, app *fiber.App, method, path string, headers map[string]string) testResponse {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("%s %s failed: %v", method, path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return testResponse{status: resp.StatusCode, header: resp.Header, body: string(body)}
}

// startLoopbackServer serves app on 127.0.0.1, so requests come from a loopback
// peer exactly as they would from a reverse proxy on the same host.
func startLoopbackServer(t *testing.T, app *fiber.App) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	go func() {
		_ = app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true})
	}()
	t.Cleanup(func() { _ = app.ShutdownWithTimeout(5 * time.Second) })
	return "http://" + ln.Addr().String()
}

func doLoopback(t *testing.T, baseURL, method, path string, headers map[string]string) testResponse {
	t.Helper()
	req, err := http.NewRequest(method, baseURL+path, nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	// The listener is bound before Serve starts, so early connections just wait
	// in the backlog.
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("%s %s failed: %v", method, path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return testResponse{status: resp.StatusCode, header: resp.Header, body: string(body)}
}

func exhaustLoginLimit(t *testing.T, send func() testResponse) {
	t.Helper()
	for i := 1; i <= LoginRequestLimit; i++ {
		if resp := send(); resp.status != fiber.StatusOK {
			t.Fatalf("login request %d of %d: expected 200, got %d", i, LoginRequestLimit, resp.status)
		}
	}
}

func TestLoginLimiter(t *testing.T) {
	t.Run("rejects requests over the limit with the API envelope", func(t *testing.T) {
		app := newTestApp(t, "")
		send := func() testResponse {
			return doTest(t, app, fiber.MethodPost, ApiPrefix+"/login", map[string]string{"Origin": testOrigin})
		}
		exhaustLoginLimit(t, send)

		resp := send()
		if resp.status != fiber.StatusTooManyRequests {
			t.Fatalf("expected 429, got %d", resp.status)
		}
		var envelope models.ServerAPIResponse
		if err := json.Unmarshal([]byte(resp.body), &envelope); err != nil {
			t.Fatalf("429 body is not the API envelope: %q", resp.body)
		}
		if envelope.Success || envelope.Error == nil || envelope.Error.Code != "RATE_LIMIT_EXCEEDED" {
			t.Errorf("unexpected envelope: %s", resp.body)
		}
		if resp.header.Get("Retry-After") == "" {
			t.Error("429 should carry Retry-After")
		}
		// Without CORS headers the browser could not read the 429 at all.
		if got := resp.header.Get("Access-Control-Allow-Origin"); got != testOrigin {
			t.Errorf("429 should carry CORS headers, got Access-Control-Allow-Origin=%q", got)
		}
	})

	t.Run("only limits POST /login", func(t *testing.T) {
		app := newTestApp(t, "")
		exhaustLoginLimit(t, func() testResponse {
			return doTest(t, app, fiber.MethodPost, ApiPrefix+"/login", nil)
		})

		for i := 0; i < LoginRequestLimit+5; i++ {
			if resp := doTest(t, app, fiber.MethodPost, ApiPrefix+"/logout", nil); resp.status != fiber.StatusOK {
				t.Fatalf("POST /logout should not be limited, got %d", resp.status)
			}
			if resp := doTest(t, app, fiber.MethodGet, ApiPrefix+"/login", nil); resp.status != fiber.StatusOK {
				t.Fatalf("GET /login should not be limited, got %d", resp.status)
			}
		}
	})

	t.Run("CORS preflight is not limited", func(t *testing.T) {
		app := newTestApp(t, "")
		exhaustLoginLimit(t, func() testResponse {
			return doTest(t, app, fiber.MethodPost, ApiPrefix+"/login", nil)
		})

		resp := doTest(t, app, fiber.MethodOptions, ApiPrefix+"/login", map[string]string{
			"Origin":                        testOrigin,
			"Access-Control-Request-Method": fiber.MethodPost,
		})
		if resp.status == fiber.StatusTooManyRequests || resp.header.Get("Access-Control-Allow-Origin") != testOrigin {
			t.Errorf("preflight should be answered by CORS, got %d", resp.status)
		}
	})

	t.Run("forwarded headers cannot bypass the limit without a proxy configured", func(t *testing.T) {
		app := newTestApp(t, "")
		for i := 0; i < LoginRequestLimit; i++ {
			doTest(t, app, fiber.MethodPost, ApiPrefix+"/login", map[string]string{
				"X-Forwarded-For": fmt.Sprintf("203.0.113.%d", i+1),
			})
		}
		resp := doTest(t, app, fiber.MethodPost, ApiPrefix+"/login", map[string]string{"X-Forwarded-For": "203.0.113.99"})
		if resp.status != fiber.StatusTooManyRequests {
			t.Errorf("expected 429 regardless of X-Forwarded-For, got %d", resp.status)
		}
	})

	t.Run("forwarded headers from an untrusted peer are ignored", func(t *testing.T) {
		app := newTestApp(t, fiber.HeaderXForwardedFor)
		for i := 0; i < LoginRequestLimit; i++ {
			doTest(t, app, fiber.MethodPost, ApiPrefix+"/login", map[string]string{
				"X-Forwarded-For": fmt.Sprintf("203.0.113.%d", i+1),
			})
		}
		resp := doTest(t, app, fiber.MethodPost, ApiPrefix+"/login", map[string]string{"X-Forwarded-For": "203.0.113.99"})
		if resp.status != fiber.StatusTooManyRequests {
			t.Errorf("app.Test peer 0.0.0.0 is not loopback; expected 429, got %d", resp.status)
		}
	})

	t.Run("limits each client separately behind a trusted proxy", func(t *testing.T) {
		baseURL := startLoopbackServer(t, newTestApp(t, fiber.HeaderXForwardedFor))
		clientA := map[string]string{"X-Forwarded-For": "203.0.113.10"}
		clientB := map[string]string{"X-Forwarded-For": "203.0.113.20"}

		exhaustLoginLimit(t, func() testResponse {
			return doLoopback(t, baseURL, fiber.MethodPost, ApiPrefix+"/login", clientA)
		})
		if resp := doLoopback(t, baseURL, fiber.MethodPost, ApiPrefix+"/login", clientA); resp.status != fiber.StatusTooManyRequests {
			t.Errorf("client A should be limited, got %d", resp.status)
		}
		if resp := doLoopback(t, baseURL, fiber.MethodPost, ApiPrefix+"/login", clientB); resp.status != fiber.StatusOK {
			t.Errorf("client B should not be limited by client A, got %d", resp.status)
		}
	})

	t.Run("a client cannot dodge the limit by spoofing X-Forwarded-For", func(t *testing.T) {
		baseURL := startLoopbackServer(t, newTestApp(t, fiber.HeaderXForwardedFor))
		send := func(i int) testResponse {
			// A proxy appending to X-Forwarded-For keeps whatever the client sent
			// on the left; the real client address is the rightmost entry.
			return doLoopback(t, baseURL, fiber.MethodPost, ApiPrefix+"/login", map[string]string{
				"X-Forwarded-For": fmt.Sprintf("198.51.100.%d, 203.0.113.30", i),
			})
		}
		for i := 1; i <= LoginRequestLimit; i++ {
			send(i)
		}
		if resp := send(99); resp.status != fiber.StatusTooManyRequests {
			t.Errorf("expected 429 despite varying spoofed entries, got %d", resp.status)
		}
	})
}

func TestClientIPResolution(t *testing.T) {
	cases := []struct {
		name        string
		proxyHeader string
		headers     map[string]string
		want        string
	}{
		{"no proxy configured ignores X-Forwarded-For", "", map[string]string{"X-Forwarded-For": "203.0.113.7"}, "127.0.0.1"},
		{"X-Forwarded-For single client", fiber.HeaderXForwardedFor, map[string]string{"X-Forwarded-For": "203.0.113.7"}, "203.0.113.7"},
		{"X-Forwarded-For takes the rightmost untrusted entry", fiber.HeaderXForwardedFor, map[string]string{"X-Forwarded-For": "198.51.100.1, 203.0.113.7"}, "203.0.113.7"},
		{"X-Forwarded-For skips trusted loopback hops", fiber.HeaderXForwardedFor, map[string]string{"X-Forwarded-For": "203.0.113.7, 127.0.0.1"}, "203.0.113.7"},
		{"X-Forwarded-For IPv6 client", fiber.HeaderXForwardedFor, map[string]string{"X-Forwarded-For": "2001:db8::1"}, "2001:db8::1"},
		{"missing header falls back to the peer", fiber.HeaderXForwardedFor, nil, "127.0.0.1"},
		{"garbage header falls back to the peer", fiber.HeaderXForwardedFor, map[string]string{"X-Forwarded-For": "not-an-ip"}, "127.0.0.1"},
		{"X-Real-IP", "X-Real-IP", map[string]string{"X-Real-IP": "203.0.113.8"}, "203.0.113.8"},
		// Fiber's validator accepts IPv6 groups longer than 4 digits; ClientIP must
		// not pass such a value (63 chars, over the 45-char ip_address columns) on.
		{"oversized IPv6 falls back to the peer", "X-Real-IP", map[string]string{"X-Real-IP": strings.Repeat("0", 60) + "::1"}, "127.0.0.1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baseURL := startLoopbackServer(t, newTestApp(t, tc.proxyHeader))
			resp := doLoopback(t, baseURL, fiber.MethodGet, ApiPrefix+"/ip", tc.headers)
			if got := strings.TrimSpace(resp.body); got != tc.want {
				t.Errorf("expected client IP %q, got %q", tc.want, got)
			}
		})
	}
}

func newTestServer(t *testing.T, proxyHeader string) *Server {
	t.Helper()
	testLogger, err := logger.New("TEST", "disabled", "", true)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	t.Cleanup(func() { _ = testLogger.Close() })
	serverConfig := newTestServerConfig(t, proxyHeader)
	s := &Server{config: serverConfig, fiber: getFiberApp(serverConfig, testLogger), logger: testLogger}
	s.RegisterRoutes()
	return s
}

// Exercises the real Login route (malformed JSON is rejected with 400 before
// any database access), so moving the route away from the limiter's path
// fails here.
func TestLoginLimiterGuardsRealLoginRoute(t *testing.T) {
	s := newTestServer(t, "")
	paths := []string{ApiPrefix + "/login", ApiPrefix + "/login/", "/V1/API/LOGIN"}
	send := func(path string) int {
		req := httptest.NewRequest(fiber.MethodPost, path, strings.NewReader("{"))
		req.Header.Set("Content-Type", "application/json")
		resp, err := s.fiber.Test(req)
		if err != nil {
			t.Fatalf("POST %s failed: %v", path, err)
		}
		_ = resp.Body.Close()
		return resp.StatusCode
	}

	for i := 0; i < LoginRequestLimit; i++ {
		path := paths[i%len(paths)]
		if status := send(path); status != fiber.StatusBadRequest {
			t.Fatalf("POST %s: expected the Login handler's 400, got %d", path, status)
		}
	}
	for _, path := range paths {
		if status := send(path); status != fiber.StatusTooManyRequests {
			t.Errorf("POST %s: path variants must share the limit, got %d", path, status)
		}
	}
}

func TestLoginLimiterBucketsIPv6By64(t *testing.T) {
	baseURL := startLoopbackServer(t, newTestApp(t, fiber.HeaderXForwardedFor))
	send := func(ip string) int {
		return doLoopback(t, baseURL, fiber.MethodPost, ApiPrefix+"/login", map[string]string{"X-Forwarded-For": ip}).status
	}

	for i := 1; i <= LoginRequestLimit; i++ {
		send(fmt.Sprintf("2001:db8::%x", i)) // rotating addresses inside one /64
	}
	if status := send("2001:db8::ffff"); status != fiber.StatusTooManyRequests {
		t.Errorf("rotating within a /64 should not reset the limit, got %d", status)
	}
	if status := send("2001:db8:0:1::1"); status != fiber.StatusOK {
		t.Errorf("another /64 should have its own limit, got %d", status)
	}
}

func TestRejectionReporterThrottlesLogs(t *testing.T) {
	var logged []string
	r := &rejectionReporter{interval: time.Minute, log: func(msg string) { logged = append(logged, msg) }}
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	r.record(start, "203.0.113.1")
	r.record(start.Add(10*time.Second), "203.0.113.2")
	r.record(start.Add(50*time.Second), "203.0.113.3")
	if len(logged) != 1 || !strings.Contains(logged[0], "rejected 1 request(s)") {
		t.Fatalf("expected one log for the first rejection, got %q", logged)
	}

	r.record(start.Add(61*time.Second), "203.0.113.4")
	if len(logged) != 2 || !strings.Contains(logged[1], "rejected 3 request(s)") || !strings.Contains(logged[1], "203.0.113.4") {
		t.Fatalf("expected a summary of the 3 rejections since the last log, got %q", logged)
	}
}

func TestLoginLimiterIsExactUnderConcurrency(t *testing.T) {
	app := newTestApp(t, "")
	const requests = 60
	statuses := make(chan int, requests)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			req := httptest.NewRequest(fiber.MethodPost, ApiPrefix+"/login", nil)
			resp, err := app.Test(req)
			if err != nil {
				statuses <- -1
				return
			}
			_ = resp.Body.Close()
			statuses <- resp.StatusCode
		}()
	}
	close(start)
	wg.Wait()
	close(statuses)

	counts := map[int]int{}
	for status := range statuses {
		counts[status]++
	}
	if counts[fiber.StatusOK] != LoginRequestLimit || counts[fiber.StatusTooManyRequests] != requests-LoginRequestLimit {
		t.Errorf("expected exactly %d allowed and %d rejected, got %v", LoginRequestLimit, requests-LoginRequestLimit, counts)
	}
}

func TestLimiterWindowResets(t *testing.T) {
	testLogger, err := logger.New("TEST", "disabled", "", true)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	t.Cleanup(func() { _ = testLogger.Close() })
	app := fiber.New()
	app.Use(ApiPrefix+"/login", newLimiter(testLogger, 2, time.Second))
	app.Post(ApiPrefix+"/login", func(ctx fiber.Ctx) error { return ctx.SendStatus(fiber.StatusOK) })
	send := func() int {
		resp, err := app.Test(httptest.NewRequest(fiber.MethodPost, ApiPrefix+"/login", nil))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		_ = resp.Body.Close()
		return resp.StatusCode
	}

	if send() != fiber.StatusOK || send() != fiber.StatusOK || send() != fiber.StatusTooManyRequests {
		t.Fatal("expected 2 allowed requests, then 429")
	}
	// The limiter's clock has one-second resolution, so wait past two windows.
	time.Sleep(2500 * time.Millisecond)
	if status := send(); status != fiber.StatusOK {
		t.Errorf("a new window should allow requests again, got %d", status)
	}
}
