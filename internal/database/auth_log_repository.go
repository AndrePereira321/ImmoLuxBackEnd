package database

import (
	"context"
	"fmt"
	"immo-lux/internal/models"
	"immo-lux/internal/server_error"
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
