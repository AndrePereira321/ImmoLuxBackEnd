package database

import (
	"context"
	"fmt"
	"time"

	"immo-lux/internal/database/ent/client"
	"immo-lux/internal/models"
	"immo-lux/internal/server_error"
	"immo-lux/internal/utils"

	"golang.org/x/crypto/bcrypt"
)

// AuthOperations owns the authentication flows — the one place where
// credentials, rate limiting, sessions, tokens and the audit log meet.
type AuthOperations struct {
	db *Database
}

func (db *Database) NewAuthOperations() *AuthOperations {
	return &AuthOperations{db: db}
}

const loginAction = "LOGIN"

type LoginInput struct {
	Email      string
	Password   string
	RememberMe bool
	IpAddress  string
	UserAgent  string
}

type LoginResult struct {
	User  *models.UserDTO
	Token string
}

func (input LoginInput) validate() error {
	if !utils.IsValidEmail(input.Email) {
		return server_error.BadRequest("Invalid email")
	}
	if len(input.Password) < models.PasswordMinLength {
		return server_error.BadRequest(fmt.Sprintf("Password must be at least %d characters long", models.PasswordMinLength))
	}
	return nil
}

// Login authenticates the user and opens a session, enforcing the login rate
// limit and recording the attempt in the audit log. The session row and the
// signed JWT commit or roll back together.
func (ops *AuthOperations) Login(ctx context.Context, input LoginInput) (*LoginResult, error) {
	if err := input.validate(); err != nil {
		return nil, err
	}

	rateLimitRepo := ops.db.NewRateLimitRepository()
	allowed, blockedUntil, err := rateLimitRepo.CheckRateLimit(ctx, input.Email, input.IpAddress, loginAction)
	if err != nil {
		ops.db.Logger().Warn(fmt.Sprintf("Rate limit check error: %s", err.Error()))
	}
	if !allowed && blockedUntil != nil {
		ops.db.Logger().Warn(fmt.Sprintf("Login blocked for %s from IP %s until %s",
			input.Email, input.IpAddress, blockedUntil.Format(time.RFC3339)))
		return nil, server_error.RateLimited("RATE_LIMIT_EXCEEDED",
			fmt.Sprintf("Too many login attempts. Try again after %s", blockedUntil.Format(time.RFC3339)))
	}

	userDto, userAuthDto, err := ops.db.NewUserRepository().GetUserAuth(input.Email)
	if err != nil {
		if server_error.IsServerError(err, "USER_NOT_FOUND") {
			return nil, server_error.Unauthorized("INVALID_CREDENTIALS", "Invalid email or password")
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*userAuthDto.Hash), []byte(input.Password)); err != nil {
		ops.db.Logger().Trace(fmt.Sprintf("Invalid password attempt for user: %s", input.Email))

		failureReason := "Invalid password"
		_ = ops.db.NewAuthLogRepository().LogAuthEvent(ctx, LogAuthEventParams{
			UserID:        userDto.ID,
			Email:         input.Email,
			EventType:     "LOGIN_FAILED",
			IpAddress:     &input.IpAddress,
			UserAgent:     &input.UserAgent,
			FailureReason: &failureReason,
		})

		return nil, server_error.Unauthorized("INVALID_CREDENTIALS", "Invalid email or password")
	}

	var sessionId models.RecordId
	var jwtToken string

	err = ops.db.WithTransaction(ctx, func(txCtx context.Context, tx *client.Tx) error {
		expiresAt := utils.CalculateExpiration(input.RememberMe)
		tokenHash := utils.HashToken(fmt.Sprintf("%d-%d-%s", *userDto.ID, time.Now().UnixNano(), input.IpAddress))

		createdSessionId, err := ops.db.NewSessionRepository().createSession(txCtx, tx, CreateSessionParams{
			UserID:     *userDto.ID,
			TokenHash:  tokenHash,
			RememberMe: input.RememberMe,
			ExpiresAt:  expiresAt,
			IpAddress:  &input.IpAddress,
			UserAgent:  &input.UserAgent,
		})
		if err != nil {
			return err
		}
		sessionId = createdSessionId

		security := ops.db.serverConfig.Security()
		token, err := utils.GenerateJWT(*userDto.ID, sessionId, input.RememberMe, security.JwtSecret(), security.JwtIssuer())
		if err != nil {
			return err
		}
		jwtToken = token
		return nil
	})
	if err != nil {
		ops.db.Logger().Error(fmt.Sprintf("Failed to create session for user %s: %s", input.Email, err.Error()))
		return nil, server_error.Wrap("SESSION_CREATE_ERROR", "Failed to create session", err)
	}

	_ = rateLimitRepo.ResetRateLimit(ctx, input.Email, input.IpAddress, loginAction)

	_ = ops.db.NewAuthLogRepository().LogAuthEvent(ctx, LogAuthEventParams{
		UserID:    userDto.ID,
		Email:     input.Email,
		EventType: "LOGIN_SUCCESS",
		IpAddress: &input.IpAddress,
		UserAgent: &input.UserAgent,
	})

	ops.db.Logger().InfoEvent().
		Str("email", input.Email).
		Int64("userId", int64(*userDto.ID)).
		Int64("sessionId", int64(sessionId)).
		Bool("rememberMe", input.RememberMe).
		Msg("User logged in successfully")

	return &LoginResult{
		User:  userDto,
		Token: jwtToken,
	}, nil
}

// Logout revokes the session and records the event. Audit logging is
// best-effort; the revocation error is returned for the caller to judge.
func (ops *AuthOperations) Logout(ctx context.Context, userId, sessionId models.RecordId, ipAddress, userAgent string) error {
	email := ""
	if userDto, err := ops.db.NewUserRepository().GetUserById(ctx, userId); err != nil {
		ops.db.Logger().Warn(fmt.Sprintf("Failed to get user %d for logout: %s", userId, err.Error()))
	} else if userDto.Email != nil {
		email = *userDto.Email
	}

	revokeErr := ops.db.NewSessionRepository().RevokeOwnedSession(ctx, userId, sessionId, "User logged out")

	if email != "" {
		_ = ops.db.NewAuthLogRepository().LogAuthEvent(ctx, LogAuthEventParams{
			UserID:    &userId,
			Email:     email,
			EventType: "LOGOUT",
			IpAddress: &ipAddress,
			UserAgent: &userAgent,
		})
	}

	ops.db.Logger().InfoEvent().
		Int64("sessionId", int64(sessionId)).
		Int64("userId", int64(userId)).
		Str("email", email).
		Msg("User logged out successfully")

	return revokeErr
}
