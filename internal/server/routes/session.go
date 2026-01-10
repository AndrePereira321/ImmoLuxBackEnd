package routes

import (
	"context"
	"fmt"

	"github.com/gofiber/fiber/v3"
	"immo-lux/internal/database/ent/client"
	"immo-lux/internal/models"
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
	userContext := ctx.UserContext()
	if userContext == nil {
		return ctx.RespondError(fiber.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated")
	}

	sessionRepo := ctx.Db().NewSessionRepository()
	sessions, err := sessionRepo.GetActiveSessionsForUser(ctx.Ctx().Context(), userContext.UserID)
	if err != nil {
		ctx.Logger().Error(fmt.Sprintf("Failed to get sessions for user %d: %s", userContext.UserID, err.Error()))
		return ctx.InternalError("Failed to retrieve sessions")
	}

	currentSessionID := userContext.SessionID

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
	userContext := ctx.UserContext()
	if userContext == nil {
		return ctx.RespondError(fiber.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated")
	}

	payload := &RevokeSessionRequest{}
	if err := ctx.ReadBody(&payload); err != nil {
		return err
	}

	if payload.SessionID == 0 {
		return ctx.BadRequest("Session ID is required")
	}

	sessionRepo := ctx.Db().NewSessionRepository()
	targetSessionID := models.RecordId(payload.SessionID)

	session, err := sessionRepo.GetSessionById(ctx.Ctx().Context(), targetSessionID)
	if err != nil {
		ctx.Logger().Warn(fmt.Sprintf("Session %d not found: %s", targetSessionID, err.Error()))
		return ctx.RespondError(fiber.StatusNotFound, "SESSION_NOT_FOUND", "Session not found")
	}

	if *session.UserID != userContext.UserID {
		ctx.Logger().Warn(fmt.Sprintf("User %d attempted to revoke session %d belonging to user %d",
			userContext.UserID, targetSessionID, *session.UserID))
		return ctx.RespondError(fiber.StatusForbidden, "FORBIDDEN", "You can only revoke your own sessions")
	}

	err = ctx.Db().WithTransaction(func(txCtx context.Context, tx *client.Tx) error {
		return sessionRepo.InvalidateSession(txCtx, tx, targetSessionID, "USER_REVOKED")
	})

	if err != nil {
		ctx.Logger().Error(fmt.Sprintf("Failed to revoke session %d: %s", targetSessionID, err.Error()))
		return ctx.InternalError("Failed to revoke session")
	}

	ctx.Logger().Info(fmt.Sprintf("User %d revoked session %d", userContext.UserID, targetSessionID))

	return ctx.RespondData(fiber.Map{
		"success": true,
		"message": "Session revoked successfully",
	})
}
