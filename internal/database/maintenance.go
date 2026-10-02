package database

import (
	"context"
	"fmt"
	"time"
)

const (
	// DataRetention is how long ended sessions, auth logs and inactive
	// rate-limit records are kept before the maintenance job deletes them.
	DataRetention = 90 * 24 * time.Hour
)

// Vars so tests can shorten the interval and force a timeout.
var (
	maintenanceInterval    = 24 * time.Hour
	maintenanceStepTimeout = 5 * time.Minute // bounds each table's delete
)

type maintenanceJob struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// startMaintenance runs runMaintenance immediately and then every
// maintenanceInterval until stopMaintenance is called. It is a no-op while a
// job is already running.
func (db *Database) startMaintenance() {
	if db.maintenance != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	job := &maintenanceJob{cancel: cancel, done: make(chan struct{})}
	db.maintenance = job

	go func() {
		defer close(job.done)
		ticker := time.NewTicker(maintenanceInterval)
		defer ticker.Stop()
		for {
			db.runMaintenance(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

// stopMaintenance cancels the job, which rolls back an in-flight delete, and
// waits for its goroutine to exit so the database can be closed safely.
func (db *Database) stopMaintenance() {
	if db.maintenance == nil {
		return
	}
	db.maintenance.cancel()
	<-db.maintenance.done
	db.maintenance = nil
}

// runMaintenance deletes records older than DataRetention. Each table gets its
// own timeout and is cleaned independently, so a slow or failing delete does
// not skip the others; only cancelling parent (shutdown) ends a run early.
func (db *Database) runMaintenance(parent context.Context) {
	cutoff := time.Now().Add(-DataRetention)
	steps := []struct {
		name string
		run  func(context.Context, time.Time) (int, error)
	}{
		{"sessions", db.NewSessionRepository().DeleteEndedBefore},
		{"auth logs", db.NewAuthLogRepository().DeleteCreatedBefore},
		{"rate limit records", db.NewRateLimitRepository().DeleteInactiveBefore},
	}

	deleted := 0
	for _, step := range steps {
		ctx, cancel := context.WithTimeout(parent, maintenanceStepTimeout)
		count, err := step.run(ctx, cutoff)
		cancel()
		if err != nil {
			if parent.Err() != nil {
				return
			}
			db.logger.Error(fmt.Sprintf("Maintenance: failed to delete old %s: %s", step.name, err.Error()))
			continue
		}
		deleted += count
		if count > 0 {
			db.logger.Info(fmt.Sprintf("Maintenance: deleted %d %s older than %s", count, step.name, cutoff.Format(time.DateOnly)))
		}
	}
	db.logger.Debug(fmt.Sprintf("Maintenance: run finished, %d records older than %s deleted", deleted, cutoff.Format(time.DateOnly)))
}
