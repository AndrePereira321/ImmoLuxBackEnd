package routes

import (
	"immo-lux/internal/models"
	"immo-lux/internal/server_error"

	"github.com/gofiber/fiber/v3"
)

type SessionInfoResponse struct {
	ID        int64   `json:"id"`
	IpAddress *string `json:"ipAddress"`
	UserAgent *string `json:"userAgent"`
	CreatedAt string  `json:"createdAt"`
	ExpiresAt string  `json:"expiresAt"`
	IsCurrent bool    `json:"isCurrent"`
}

type ListSessionsResponse struct {
	Sessions []SessionInfoResponse `json:"sessions"`
	Total    int                   `json:"total"`
}

func GetSessions(ctx *RouteContext) error {
	userId, err := ctx.RequireUserId()
	if err != nil {
		return err
	}

	sessions, err := ctx.Db().NewSessionRepository().GetActiveSessionsForUser(ctx.RequestContext(), userId)
	if err != nil {
		return err
	}

	currentSessionID := ctx.GetSessionId()

	response := ListSessionsResponse{
		Sessions: make([]SessionInfoResponse, len(sessions)),
		Total:    len(sessions),
	}

	for i, sess := range sessions {
		response.Sessions[i] = SessionInfoResponse{
			ID:        int64(*sess.ID),
			IpAddress: sess.IpAddress,
			UserAgent: sess.UserAgent,
			CreatedAt: sess.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			ExpiresAt: sess.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
			IsCurrent: *sess.ID == currentSessionID,
		}
	}

	return ctx.RespondData(response)
}

type RevokeSessionRequest struct {
	SessionID int64 `json:"sessionId"`
}

func RevokeSession(ctx *RouteContext) error {
	userId, err := ctx.RequireUserId()
	if err != nil {
		return err
	}

	payload := &RevokeSessionRequest{}
	if err := ctx.ReadBody(&payload); err != nil {
		return err
	}

	if payload.SessionID == 0 {
		return server_error.Invalid("BAD_REQUEST", "Session ID is required")
	}

	if err := ctx.Db().NewSessionRepository().RevokeOwnedSession(
		ctx.RequestContext(), userId, models.RecordId(payload.SessionID), "USER_REVOKED"); err != nil {
		return err
	}

	return ctx.RespondData(fiber.Map{
		"success": true,
		"message": "Session revoked successfully",
	})
}
