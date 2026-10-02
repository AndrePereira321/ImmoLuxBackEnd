package database

import (
	"context"
	"errors"
	"fmt"
	"immo-lux/internal/config"
	"immo-lux/internal/database/ent/client/authlog"
	"immo-lux/internal/database/ent/client/ratelimit"
	"immo-lux/internal/database/ent/client/session"
	"immo-lux/internal/logger"
	"immo-lux/internal/models"
	"immo-lux/internal/server_error"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func requireServerError(t *testing.T, err error, kind server_error.Kind, code string) {
	t.Helper()
	var serverError *server_error.ServerError
	if !errors.As(err, &serverError) {
		t.Fatalf("expected ServerError %s, got %v", code, err)
	}
	if serverError.Kind != kind || serverError.Code != code {
		t.Fatalf("expected %s (kind %d), got %s (kind %d): %s", code, kind, serverError.Code, serverError.Kind, serverError.Message)
	}
}

func createTestSession(t *testing.T, db *Database, userId models.RecordId, token string, expiresAt time.Time, invalidatedAt *time.Time) {
	t.Helper()
	_, err := db.client.Session.Create().
		SetUserID(int(userId)).
		SetSessionToken(token).
		SetExpiresAt(expiresAt).
		SetIsActive(invalidatedAt == nil).
		SetNillableInvalidatedAt(invalidatedAt).
		Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create session %s: %v", token, err)
	}
}

func createTestAuthLog(t *testing.T, db *Database, email string, createdAt time.Time) {
	t.Helper()
	_, err := db.client.AuthLog.Create().
		SetEmail(email).
		SetEventType("LOGIN_FAILED").
		SetCreatedAt(createdAt).
		Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create auth log %s: %v", email, err)
	}
}

func createTestRateLimit(t *testing.T, db *Database, email string, windowStart time.Time, blockedUntil *time.Time) {
	t.Helper()
	_, err := db.client.RateLimit.Create().
		SetEmail(email).
		SetIPAddress("203.0.113.1").
		SetAction(loginAction).
		SetAttemptCount(LoginMaxAttempts).
		SetWindowStart(windowStart).
		SetNillableBlockedUntil(blockedUntil).
		Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create rate limit %s: %v", email, err)
	}
}

func remainingSessionTokens(t *testing.T, db *Database) []string {
	t.Helper()
	tokens, err := db.client.Session.Query().Select(session.FieldSessionToken).Strings(context.Background())
	if err != nil {
		t.Fatalf("failed to list sessions: %v", err)
	}
	sort.Strings(tokens)
	return tokens
}

func remainingAuthLogEmails(t *testing.T, db *Database) []string {
	t.Helper()
	emails, err := db.client.AuthLog.Query().Select(authlog.FieldEmail).Strings(context.Background())
	if err != nil {
		t.Fatalf("failed to list auth logs: %v", err)
	}
	sort.Strings(emails)
	return emails
}

func remainingRateLimitEmails(t *testing.T, db *Database) []string {
	t.Helper()
	emails, err := db.client.RateLimit.Query().Select(ratelimit.FieldEmail).Strings(context.Background())
	if err != nil {
		t.Fatalf("failed to list rate limits: %v", err)
	}
	sort.Strings(emails)
	return emails
}

func requireStrings(t *testing.T, what string, got []string, want ...string) {
	t.Helper()
	sort.Strings(want)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("%s: expected %v, got %v", what, want, got)
	}
}

func ptr[T any](v T) *T { return &v }

// seedRetentionFixtures creates one record on each side of the retention
// cutoff in every table the maintenance job cleans, plus records that must
// survive regardless of age rules.
func seedRetentionFixtures(t *testing.T, db *Database, now time.Time) {
	t.Helper()
	cutoff := now.Add(-DataRetention)
	userId := createTestUser(t, db, "owner@test.local")

	createTestSession(t, db, userId, "expired-before-cutoff", cutoff.Add(-time.Hour), nil)
	createTestSession(t, db, userId, "expired-after-cutoff", cutoff.Add(time.Hour), nil)
	createTestSession(t, db, userId, "active", now.Add(24*time.Hour), nil)
	createTestSession(t, db, userId, "revoked-before-cutoff", cutoff.Add(10*24*time.Hour), ptr(cutoff.Add(-time.Hour)))
	createTestSession(t, db, userId, "revoked-recently", now.Add(20*24*time.Hour), ptr(now.Add(-24*time.Hour)))

	createTestAuthLog(t, db, "before-cutoff@test.local", cutoff.Add(-time.Hour))
	createTestAuthLog(t, db, "after-cutoff@test.local", cutoff.Add(time.Hour))
	createTestAuthLog(t, db, "now@test.local", now)

	createTestRateLimit(t, db, "before-cutoff@test.local", cutoff.Add(-time.Hour), nil)
	createTestRateLimit(t, db, "blocked-before-cutoff@test.local", cutoff.Add(-time.Hour), ptr(cutoff.Add(-time.Hour+LoginBlockDuration)))
	createTestRateLimit(t, db, "after-cutoff@test.local", cutoff.Add(time.Hour), nil)
	createTestRateLimit(t, db, "blocked-now@test.local", now.Add(-10*time.Minute), ptr(now.Add(20*time.Minute)))
}

func requireRetentionApplied(t *testing.T, db *Database) {
	t.Helper()
	requireStrings(t, "sessions", remainingSessionTokens(t, db),
		"expired-after-cutoff", "active", "revoked-recently")
	requireStrings(t, "auth logs", remainingAuthLogEmails(t, db),
		"after-cutoff@test.local", "now@test.local")
	requireStrings(t, "rate limits", remainingRateLimitEmails(t, db),
		"after-cutoff@test.local", "blocked-now@test.local")
}

func TestRetentionDeletes(t *testing.T) {
	db := newTestDatabase(t)
	ctx := context.Background()
	now := time.Now()
	cutoff := now.Add(-DataRetention)
	seedRetentionFixtures(t, db, now)

	sessions, err := db.NewSessionRepository().DeleteEndedBefore(ctx, cutoff)
	if err != nil || sessions != 2 {
		t.Fatalf("sessions: expected 2 deleted, got %d (err %v)", sessions, err)
	}
	authLogs, err := db.NewAuthLogRepository().DeleteCreatedBefore(ctx, cutoff)
	if err != nil || authLogs != 1 {
		t.Fatalf("auth logs: expected 1 deleted, got %d (err %v)", authLogs, err)
	}
	rateLimits, err := db.NewRateLimitRepository().DeleteInactiveBefore(ctx, cutoff)
	if err != nil || rateLimits != 2 {
		t.Fatalf("rate limits: expected 2 deleted, got %d (err %v)", rateLimits, err)
	}
	requireRetentionApplied(t, db)

	// A second pass finds nothing left to delete.
	if n, _ := db.NewSessionRepository().DeleteEndedBefore(ctx, cutoff); n != 0 {
		t.Errorf("second session pass deleted %d", n)
	}
}

func TestRetentionKeepsActiveRateLimitBlocks(t *testing.T) {
	db := newTestDatabase(t)
	ctx := context.Background()
	now := time.Now()
	createTestRateLimit(t, db, "blocked@test.local", now.Add(-10*time.Minute), ptr(now.Add(20*time.Minute)))

	db.runMaintenance(ctx)

	allowed, blockedUntil, err := db.NewRateLimitRepository().CheckRateLimit(ctx, "blocked@test.local", "203.0.113.1", loginAction)
	if err != nil {
		t.Fatalf("CheckRateLimit failed: %v", err)
	}
	if allowed || blockedUntil == nil {
		t.Fatal("an active block must survive maintenance")
	}
}

func TestRunMaintenance(t *testing.T) {
	db := newTestDatabase(t)
	seedRetentionFixtures(t, db, time.Now())

	db.runMaintenance(context.Background())

	requireRetentionApplied(t, db)
}

func TestRunMaintenanceWithCancelledContext(t *testing.T) {
	db := newTestDatabase(t)
	seedRetentionFixtures(t, db, time.Now())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	db.runMaintenance(ctx)

	if got := len(remainingSessionTokens(t, db)); got != 5 {
		t.Errorf("a cancelled run must not delete anything, %d of 5 sessions left", got)
	}
}

func TestMaintenanceJobLifecycle(t *testing.T) {
	db := newTestDatabase(t)
	createTestAuthLog(t, db, "old@test.local", time.Now().Add(-DataRetention-time.Hour))

	// Init starts the job (with no seed users file, initUsers is a no-op).
	serverConfig, err := config.GetServerConfig([]byte(fmt.Sprintf(`
[app]
name="ImmoLux"
version="1.0.0"
[server]
host="localhost"
[security]
jwt_secret="test-secret-that-is-at-least-32-characters"
[database]
host="localhost"
username="test"
password="test"
db_name="test"
user_file_path=%q
`, filepath.ToSlash(filepath.Join(t.TempDir(), "missing.json")))))
	if err != nil {
		t.Fatalf("invalid test config: %v", err)
	}
	db.serverConfig = serverConfig
	if err := db.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	job := db.maintenance
	if job == nil {
		t.Fatal("Init should start the maintenance job")
	}
	db.startMaintenance()
	if db.maintenance != job {
		t.Fatal("starting maintenance twice must not replace (and leak) the running job")
	}

	// The first run happens immediately, not after maintenanceInterval.
	deadline := time.Now().Add(10 * time.Second)
	for len(remainingAuthLogEmails(t, db)) > 0 {
		if time.Now().After(deadline) {
			t.Fatal("maintenance did not run at startup")
		}
		time.Sleep(50 * time.Millisecond)
	}

	stopped := make(chan struct{})
	go func() {
		db.stopMaintenance()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("stopMaintenance did not return")
	}
	if db.maintenance != nil {
		t.Error("stopMaintenance should clear the job")
	}

	db.stopMaintenance() // stopping twice is a no-op
}

func TestRunMaintenanceLogsStepTimeouts(t *testing.T) {
	db := newTestDatabase(t)
	seedRetentionFixtures(t, db, time.Now())

	logDir := t.TempDir()
	fileLogger, err := logger.New("MAINTENANCE", "debug", logDir, false)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	quietLogger := db.logger
	db.logger = fileLogger
	t.Cleanup(func() {
		_ = fileLogger.Close()
		db.logger = quietLogger
	})

	previousTimeout := maintenanceStepTimeout
	// A non-positive timeout cancels synchronously; 1ns would race the first query.
	maintenanceStepTimeout = 0
	t.Cleanup(func() { maintenanceStepTimeout = previousTimeout })

	db.runMaintenance(context.Background())

	logged, err := os.ReadFile(filepath.Join(logDir, "MAINTENANCE.log"))
	if err != nil {
		t.Fatalf("failed to read log: %v", err)
	}
	// Every step is attempted and its timeout reported, not silently dropped.
	for _, step := range []string{"sessions", "auth logs", "rate limit records"} {
		if !strings.Contains(string(logged), "Maintenance: failed to delete old "+step) {
			t.Errorf("timeout of %q step was not logged; log:\n%s", step, logged)
		}
	}
	if got := len(remainingSessionTokens(t, db)); got != 5 {
		t.Errorf("timed-out deletes must not remove rows, %d of 5 sessions left", got)
	}
}

func TestMaintenanceJobRepeatsOnInterval(t *testing.T) {
	db := newTestDatabase(t)
	previousInterval := maintenanceInterval
	maintenanceInterval = 100 * time.Millisecond
	t.Cleanup(func() { maintenanceInterval = previousInterval })

	db.startMaintenance()
	t.Cleanup(db.stopMaintenance)

	// Rows that become stale after the startup run are removed by later ticks.
	for round := 0; round < 3; round++ {
		email := fmt.Sprintf("old-%d@test.local", round)
		createTestAuthLog(t, db, email, time.Now().Add(-DataRetention-time.Hour))
		deadline := time.Now().Add(5 * time.Second)
		for len(remainingAuthLogEmails(t, db)) > 0 {
			if time.Now().After(deadline) {
				t.Fatalf("round %d: %s was not deleted by a scheduled run", round, email)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
}
