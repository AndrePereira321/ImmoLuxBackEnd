package database

import (
	"context"
	"fmt"
	"immo-lux/internal/database/ent/client"
	"immo-lux/internal/database/ent/client/ratelimit"
	"immo-lux/internal/server_error"
	"time"
)

type RateLimitRepository struct {
	db *Database
}

func NewRateLimitRepository(db *Database) *RateLimitRepository {
	return &RateLimitRepository{db: db}
}

const (
	LoginRateLimitWindow = 15 * time.Minute
	LoginMaxAttempts     = 5
	LoginBlockDuration   = 30 * time.Minute
)

func (rep *RateLimitRepository) CheckRateLimit(ctx context.Context, email string, ipAddress string, action string) (bool, *time.Time, error) {
	now := time.Now()
	windowStart := now.Add(-LoginRateLimitWindow)

	record, err := rep.db.client.RateLimit.Query().
		Where(
			ratelimit.EmailEQ(email),
			ratelimit.IPAddressEQ(ipAddress),
			ratelimit.ActionEQ(action),
		).
		Only(ctx)

	if err != nil {
		if client.IsNotFound(err) {
			_, createErr := rep.db.client.RateLimit.Create().
				SetEmail(email).
				SetIPAddress(ipAddress).
				SetAction(action).
				SetAttemptCount(1).
				SetWindowStart(now).
				Save(ctx)

			if createErr != nil {
				return false, nil, server_error.Wrap("RATE_LIMIT_CREATE", "failed to create rate limit record", createErr)
			}

			return true, nil, nil
		}
		return false, nil, server_error.Wrap("RATE_LIMIT_QUERY", "failed to query rate limit", err)
	}

	if record.BlockedUntil != nil && record.BlockedUntil.After(now) {
		return false, record.BlockedUntil, nil
	}

	if record.BlockedUntil != nil && record.BlockedUntil.Before(now) {
		_, err = rep.db.client.RateLimit.UpdateOne(record).
			SetAttemptCount(1).
			SetWindowStart(now).
			ClearBlockedUntil().
			Save(ctx)

		if err != nil {
			return false, nil, server_error.Wrap("RATE_LIMIT_UPDATE", "failed to reset expired block", err)
		}

		rep.db.Logger().Info(fmt.Sprintf("Rate limit block expired and reset for %s from IP %s for action %s", email, ipAddress, action))
		return true, nil, nil
	}

	if record.WindowStart.Before(windowStart) {
		_, err = rep.db.client.RateLimit.UpdateOne(record).
			SetAttemptCount(1).
			SetWindowStart(now).
			ClearBlockedUntil().
			Save(ctx)

		if err != nil {
			return false, nil, server_error.Wrap("RATE_LIMIT_UPDATE", "failed to reset rate limit window", err)
		}

		return true, nil, nil
	}

	if record.AttemptCount >= LoginMaxAttempts {
		blockedUntil := now.Add(LoginBlockDuration)
		_, err = rep.db.client.RateLimit.UpdateOne(record).
			SetBlockedUntil(blockedUntil).
			Save(ctx)

		if err != nil {
			return false, nil, server_error.Wrap("RATE_LIMIT_UPDATE", "failed to block IP", err)
		}

		rep.db.Logger().Warn(fmt.Sprintf("Rate limit: %s from IP %s blocked for action %s until %s", email, ipAddress, action, blockedUntil.Format(time.RFC3339)))

		return false, &blockedUntil, nil
	}

	_, err = rep.db.client.RateLimit.UpdateOne(record).
		SetAttemptCount(record.AttemptCount + 1).
		Save(ctx)

	if err != nil {
		return false, nil, server_error.Wrap("RATE_LIMIT_UPDATE", "failed to increment rate limit counter", err)
	}

	return true, nil, nil
}

func (rep *RateLimitRepository) ResetRateLimit(ctx context.Context, email string, ipAddress string, action string) error {
	_, err := rep.db.client.RateLimit.Delete().
		Where(
			ratelimit.EmailEQ(email),
			ratelimit.IPAddressEQ(ipAddress),
			ratelimit.ActionEQ(action),
		).
		Exec(ctx)

	if err != nil {
		return server_error.Wrap("RATE_LIMIT_DELETE", "failed to reset rate limit", err)
	}

	return nil
}

// DeleteInactiveBefore removes rate-limit records whose current window started
// before cutoff. A window is only ever extended by a block (at most
// LoginRateLimitWindow + LoginBlockDuration after it starts) and attempts after
// it expire reset window_start, so such a record no longer limits anything.
func (rep *RateLimitRepository) DeleteInactiveBefore(ctx context.Context, cutoff time.Time) (int, error) {
	count, err := rep.db.client.RateLimit.Delete().
		Where(ratelimit.WindowStartLT(cutoff)).
		Exec(ctx)
	if err != nil {
		return 0, server_error.Wrap("RATE_LIMIT_CLEANUP", "failed to delete old rate limit records", err)
	}
	return count, nil
}
