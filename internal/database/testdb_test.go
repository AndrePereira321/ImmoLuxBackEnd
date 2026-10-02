package database

import (
	"bytes"
	"context"
	"database/sql"
	"image"
	"image/color"
	"image/png"
	"immo-lux/internal/database/ent/client"
	"immo-lux/internal/database/ent/client/migrate"
	"immo-lux/internal/logger"
	"immo-lux/internal/models"
	"os"
	"regexp"
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

// testDatabaseEnv names a PostgreSQL connection URL for integration tests, e.g.
// postgres://user:pass@localhost:5432/immolux_test?sslmode=disable, or a test
// schema inside another database via ...&search_path=immolux_test. Tests that
// need it are skipped when it is unset.
const testDatabaseEnv = "IMMOLUX_TEST_DATABASE_URL"

// testNamePattern matches "test" as a separate word ("immolux_test", "test-db")
// but not inside another word ("latest", "contest").
var testNamePattern = regexp.MustCompile(`(?i)(^|[^a-z])test([^a-z]|$)`)

// testDatabaseLockKey is a PostgreSQL advisory lock that serialises tests on
// the shared test database: go test runs packages in parallel, and every test
// here and in internal/server (same key) empties all tables.
const testDatabaseLockKey = 72746855

// lockTestDatabase holds testDatabaseLockKey on its own connection until the
// test ends. Call it before registering other cleanups so it is released last.
func lockTestDatabase(t *testing.T, dsn string) {
	t.Helper()
	ctx := context.Background()
	lockDB, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	conn, err := lockDB.Conn(ctx)
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", testDatabaseLockKey); err != nil {
		t.Fatalf("failed to lock test database: %v", err)
	}
	t.Cleanup(func() {
		_, _ = conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", testDatabaseLockKey)
		_ = conn.Close()
		_ = lockDB.Close()
	})
}

// newTestDatabase connects to the test database, migrates it and empties every
// table. Because it truncates, it refuses to run unless the database name or
// the current schema contains "test".
func newTestDatabase(t *testing.T) *Database {
	t.Helper()
	dsn := os.Getenv(testDatabaseEnv)
	if dsn == "" {
		t.Skipf("%s is not set; skipping PostgreSQL integration test", testDatabaseEnv)
	}
	lockTestDatabase(t, dsn)

	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	ctx := context.Background()

	var dbName string
	var schemaName sql.NullString
	if err := sqlDB.QueryRowContext(ctx, "SELECT current_database(), current_schema()").Scan(&dbName, &schemaName); err != nil {
		t.Fatalf("failed to query test database name: %v", err)
	}
	if !schemaName.Valid {
		t.Fatalf("no schema selected in database %q; create the search_path schema first", dbName)
	}
	if !testNamePattern.MatchString(dbName) && !testNamePattern.MatchString(schemaName.String) {
		t.Fatalf("refusing to truncate %q.%q: the database or schema name must contain the word \"test\"", dbName, schemaName.String)
	}

	drv := entsql.OpenDB(dialect.Postgres, sqlDB)
	entClient := client.NewClient(client.Driver(drv))
	if err := entClient.Schema.Create(ctx); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}

	tableNames := make([]string, 0, len(migrate.Tables))
	for _, table := range migrate.Tables {
		tableNames = append(tableNames, `"`+table.Name+`"`)
	}
	if _, err := sqlDB.ExecContext(ctx, "TRUNCATE "+strings.Join(tableNames, ", ")+" RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("failed to truncate test database: %v", err)
	}

	testLogger, err := logger.New("TEST", "disabled", "", true)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	db := &Database{client: entClient, sqlDB: sqlDB, driver: drv, logger: testLogger}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func createTestUser(t *testing.T, db *Database, email string) models.RecordId {
	t.Helper()
	u, err := db.client.User.Create().
		SetFirstName("Test").
		SetLastName("User").
		SetEmail(email).
		Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	return models.RecordId(u.ID)
}

func createTestProperty(t *testing.T, db *Database, ownerId models.RecordId) models.RecordId {
	t.Helper()
	p, err := db.client.Property.Create().
		SetTitle("Test property").
		SetDescription("A property used by tests").
		SetPrice(250000).
		SetAddress("Rua de Teste 1").
		SetDistrict("Porto").
		SetMunicipality("Porto").
		SetPublisherID(int(ownerId)).
		Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create property: %v", err)
	}
	return models.RecordId(p.ID)
}

// testPNG returns a small valid PNG that ProcessImage accepts.
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

func TestTestNamePattern(t *testing.T) {
	for name, want := range map[string]bool{
		"immolux_test": true, "test": true, "test_db": true, "immolux-test-2": true, "TEST": true,
		"immolux": false, "latest": false, "contest": false, "testing": false, "": false,
	} {
		if got := testNamePattern.MatchString(name); got != want {
			t.Errorf("testNamePattern(%q) = %v, want %v", name, got, want)
		}
	}
}
