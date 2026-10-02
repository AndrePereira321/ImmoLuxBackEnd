package database

import (
	"context"
	"fmt"
	"immo-lux/internal/database/ent/client/authlog"
	"immo-lux/internal/models"
	"immo-lux/internal/server_error"
	"time"
)

type AuthLogRepository struct {
	db *Database
}

func NewAuthLogRepository(db *Database) *AuthLogRepository {
	return &AuthLogRepository{db: db}
}

type LogAuthEventParams struct {
	UserID        *models.RecordId
	Email         string
	EventType     string
	IpAddress     *string
	UserAgent     *string
	FailureReason *string
}

func (rep *AuthLogRepository) LogAuthEvent(ctx context.Context, params LogAuthEventParams) error {
	builder := rep.db.client.AuthLog.Create().
		SetEmail(params.Email).
		SetEventType(params.EventType)

	if params.UserID != nil && params.UserID.IsValid() {
		builder.SetUserID(int(*params.UserID))
	}

	if params.IpAddress != nil {
		builder.SetIPAddress(*params.IpAddress)
	}

	if params.UserAgent != nil {
		builder.SetUserAgent(*params.UserAgent)
	}

	if params.FailureReason != nil {
		builder.SetFailureReason(*params.FailureReason)
	}

	_, err := builder.Save(ctx)
	if err != nil {
		rep.db.Logger().Error(fmt.Sprintf("Failed to create auth log: %s", err.Error()))
		return server_error.Wrap("AUTH_LOG_CREATE", "failed to create auth log", err)
	}

	return nil
}

// DeleteCreatedBefore removes auth log entries recorded before cutoff.
func (rep *AuthLogRepository) DeleteCreatedBefore(ctx context.Context, cutoff time.Time) (int, error) {
	count, err := rep.db.client.AuthLog.Delete().
		Where(authlog.CreatedAtLT(cutoff)).
		Exec(ctx)
	if err != nil {
		return 0, server_error.Wrap("AUTH_LOG_CLEANUP", "failed to delete old auth logs", err)
	}
	return count, nil
}
