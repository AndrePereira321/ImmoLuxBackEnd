package database

import (
	"context"
	"fmt"
	"immo-lux/internal/database/ent/client"
	"immo-lux/internal/database/ent/client/session"
	"immo-lux/internal/models"
	"immo-lux/internal/server_error"
	"time"
)

type SessionRepository struct {
	db *Database
}

func NewSessionRepository(db *Database) *SessionRepository {
	return &SessionRepository{db: db}
}

type CreateSessionParams struct {
	UserID     models.RecordId
	TokenHash  string
	RememberMe bool
	ExpiresAt  time.Time
	IpAddress  *string
	UserAgent  *string
}

// createSession inserts a session inside the caller's transaction; only the
// login operation opens sessions.
func (rep *SessionRepository) createSession(ctx context.Context, tx *client.Tx, params CreateSessionParams) (models.RecordId, error) {
	sessionBuilder := tx.Session.Create().
		SetUserID(int(params.UserID)).
		SetSessionToken(params.TokenHash).
		SetIsActive(true).
		SetRememberMe(params.RememberMe).
		SetExpiresAt(params.ExpiresAt)

	if params.IpAddress != nil {
		sessionBuilder.SetIPAddress(*params.IpAddress)
	}

	if params.UserAgent != nil {
		sessionBuilder.SetUserAgent(*params.UserAgent)
	}

	createdSession, err := sessionBuilder.Save(ctx)
	if err != nil {
		rep.db.Logger().Error(fmt.Sprintf("Failed to create session for user %d: %s", params.UserID, err.Error()))
		return models.InvalidRecordId, server_error.Wrap("SESSION_INSERT", "failed to create session", err)
	}

	sessionId := models.RecordId(createdSession.ID)
	return sessionId, nil
}

func (rep *SessionRepository) GetSessionById(ctx context.Context, sessionId models.RecordId) (*models.SessionDTO, error) {
	sess, err := rep.db.client.Session.Query().
		Where(session.IDEQ(int(sessionId))).
		Only(ctx)
	if err != nil {
		if client.IsNotFound(err) {
			return nil, server_error.NotFound("SESSION_NOT_FOUND", fmt.Sprintf("session not found: %d", sessionId))
		}
		return nil, server_error.Wrap("SESSION_REPOSITORY", "failed querying session", err)
	}

	return rep.sessionToDTO(sess), nil
}

func (rep *SessionRepository) GetSessionByToken(ctx context.Context, tokenHash string) (*models.SessionDTO, error) {
	sess, err := rep.db.client.Session.Query().
		Where(session.SessionTokenEQ(tokenHash)).
		Only(ctx)
	if err != nil {
		if client.IsNotFound(err) {
			return nil, server_error.NotFound("SESSION_NOT_FOUND", "session not found")
		}
		return nil, server_error.Wrap("SESSION_REPOSITORY", "failed querying session by token", err)
	}

	return rep.sessionToDTO(sess), nil
}

func (rep *SessionRepository) ValidateSession(ctx context.Context, sessionId models.RecordId) (bool, error) {
	sess, err := rep.GetSessionById(ctx, sessionId)
	if err != nil {
		if server_error.IsServerError(err, "SESSION_NOT_FOUND") {
			return false, nil
		}
		return false, err
	}

	if !*sess.IsActive {
		return false, nil
	}

	if sess.ExpiresAt.Before(time.Now()) {
		return false, nil
	}

	return true, nil
}

// RevokeOwnedSession invalidates a session ownerId owns with one
// owner-predicated statement, probing only when nothing matched.
func (rep *SessionRepository) RevokeOwnedSession(ctx context.Context, ownerId, sessionId models.RecordId, reason string) error {
	updated, err := rep.db.client.Session.Update().
		Where(session.IDEQ(int(sessionId)), session.UserIDEQ(int(ownerId))).
		SetIsActive(false).
		SetInvalidatedAt(time.Now()).
		SetInvalidatedReason(reason).
		Save(ctx)
	if err != nil {
		rep.db.Logger().Error(fmt.Sprintf("Failed to invalidate session %d: %s", sessionId, err.Error()))
		return server_error.Wrap("SESSION_INVALIDATE", "failed to invalidate session", err)
	}
	if updated == 0 {
		return ownershipVerdict(ctx,
			func(c context.Context) (bool, error) {
				return rep.db.client.Session.Query().Where(session.IDEQ(int(sessionId))).Exist(c)
			},
			"SESSION_REPOSITORY",
			server_error.NotFound("SESSION_NOT_FOUND", "Session not found"),
			server_error.Forbidden("FORBIDDEN", "You can only revoke your own sessions"),
		)
	}

	rep.db.Logger().Info(fmt.Sprintf("User %d revoked session %d", ownerId, sessionId))
	return nil
}

// DeleteEndedBefore removes sessions that expired, or were logged out or
// revoked, before cutoff.
func (rep *SessionRepository) DeleteEndedBefore(ctx context.Context, cutoff time.Time) (int, error) {
	count, err := rep.db.client.Session.Delete().
		Where(session.Or(
			session.ExpiresAtLT(cutoff),
			session.InvalidatedAtLT(cutoff),
		)).
		Exec(ctx)
	if err != nil {
		return 0, server_error.Wrap("SESSION_CLEANUP", "failed to delete ended sessions", err)
	}
	return count, nil
}

func (rep *SessionRepository) GetActiveSessionsForUser(ctx context.Context, userId models.RecordId) ([]*models.SessionDTO, error) {
	sessions, err := rep.db.client.Session.Query().
		Where(
			session.UserIDEQ(int(userId)),
			session.IsActiveEQ(true),
			session.ExpiresAtGT(time.Now()),
		).
		Order(client.Desc(session.FieldCreatedAt)).
		All(ctx)

	if err != nil {
		return nil, server_error.Wrap("SESSION_REPOSITORY", "failed querying active sessions", err)
	}

	sessionDTOs := make([]*models.SessionDTO, len(sessions))
	for i, sess := range sessions {
		sessionDTOs[i] = rep.sessionToDTO(sess)
	}

	return sessionDTOs, nil
}

func (rep *SessionRepository) sessionToDTO(sess *client.Session) *models.SessionDTO {
	sessionId := models.RecordId(sess.ID)
	userId := models.RecordId(sess.UserID)

	return &models.SessionDTO{
		ID:                &sessionId,
		UserID:            &userId,
		SessionToken:      &sess.SessionToken,
		IsActive:          &sess.IsActive,
		RememberMe:        &sess.RememberMe,
		ExpiresAt:         &sess.ExpiresAt,
		InvalidatedAt:     sess.InvalidatedAt,
		InvalidatedReason: sess.InvalidatedReason,
		IpAddress:         sess.IPAddress,
		UserAgent:         sess.UserAgent,
		CreatedAt:         &sess.CreatedAt,
		UpdatedAt:         &sess.UpdatedAt,
	}
}
