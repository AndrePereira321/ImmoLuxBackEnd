package routes

import (
	"context"
	"fmt"
	"immo-lux/internal/config"
	"immo-lux/internal/database"
	"immo-lux/internal/database/ent/client"
	"immo-lux/internal/models"
	"immo-lux/internal/server_error"
	"immo-lux/internal/utils"
	"time"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/crypto/bcrypt"
)

type LoginPayload struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	RememberMe bool   `json:"rememberMe"`
}

type LoginResponse struct {
	IsConnected bool            `json:"isConnected"`
	UserData    *models.UserDTO `json:"userData"`
}

type IsConnectedResponse struct {
	IsConnected bool            `json:"isConnected"`
	UserData    *models.UserDTO `json:"userData"`
}

type LogoutResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func Login(ctx *RouteContext) error {
	payload := &LoginPayload{}
	if err := ctx.ReadBody(&payload); err != nil {
		return err
	}

	if !utils.IsValidEmail(payload.Email) {
		return ctx.BadRequest("Invalid email")
	}

	if len(payload.Password) < models.PasswordMinLength {
		return ctx.BadRequest(fmt.Sprintf("Password must be at least %d characters long", models.PasswordMinLength))
	}

	userDto, userAuthDto, err := ctx.Db().NewUserRepository().GetUserAuth(payload.Email)
	if err != nil {
		if server_error.IsServerError(err, "USER_NOT_FOUND") {
			return ctx.RespondError(fiber.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password")
		}
		ctx.Logger().Warn(fmt.Sprintf("Error getting user data: %s", err.Error()))
		return err
	}

	if *userAuthDto.IsLocked {
		reason := "Account is locked"
		if userAuthDto.LockedReason != nil {
			reason = *userAuthDto.LockedReason
		}
		return ctx.RespondError(fiber.StatusForbidden, "USER_LOCKED", reason)
	}

	err = bcrypt.CompareHashAndPassword([]byte(*userAuthDto.Hash), []byte(payload.Password))
	if err != nil {
		ctx.Logger().Trace(fmt.Sprintf("Invalid password attempt for user: %s", payload.Email))

		txErr := ctx.Db().WithTransaction(func(txCtx context.Context, tx *client.Tx) error {
			return ctx.Db().NewUserRepository().IncrementFailedAttempts(txCtx, tx, *userDto.ID)
		})

		if txErr != nil {
			ctx.Logger().Warn(fmt.Sprintf("Failed to increment failed attempts: %s", txErr.Error()))
		}

		return ctx.RespondError(fiber.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password")
	}

	var sessionId models.RecordId
	var jwtToken string

	err = ctx.Db().WithTransaction(func(txCtx context.Context, tx *client.Tx) error {
		if err := ctx.Db().NewUserRepository().ResetFailedAttempts(txCtx, tx, *userDto.ID); err != nil {
			return err
		}

		expiresAt := utils.CalculateExpiration(payload.RememberMe)
		ipAddress := ctx.Ctx().IP()
		userAgent := ctx.Ctx().Get("User-Agent")

		tokenHash := utils.HashToken(fmt.Sprintf("%d-%d-%s", *userDto.ID, time.Now().UnixNano(), ipAddress))

		createdSessionId, err := ctx.Db().NewSessionRepository().CreateSession(txCtx, tx, database.CreateSessionParams{
			UserID:     *userDto.ID,
			TokenHash:  tokenHash,
			RememberMe: payload.RememberMe,
			ExpiresAt:  expiresAt,
			IpAddress:  &ipAddress,
			UserAgent:  &userAgent,
		})
		if err != nil {
			return err
		}

		sessionId = createdSessionId

		token, err := utils.GenerateJWT(*userDto.ID, sessionId, payload.RememberMe, ctx.ServerConfig().Security().JwtSecret(), ctx.ServerConfig().Security().JwtIssuer())
		if err != nil {
			return err
		}

		jwtToken = token
		return nil
	})

	if err != nil {
		ctx.Logger().Error(fmt.Sprintf("Failed to create session for user %s: %s", payload.Email, err.Error()))
		return ctx.RespondError(fiber.StatusInternalServerError, "SESSION_CREATE_ERROR", "Failed to create session")
	}

	cookie := createSessionCookie(jwtToken, payload.RememberMe, ctx.ServerConfig())
	ctx.Ctx().Cookie(cookie)

	ctx.Logger().InfoEvent().
		Str("email", payload.Email).
		Int64("userId", int64(*userDto.ID)).
		Int64("sessionId", int64(sessionId)).
		Bool("rememberMe", payload.RememberMe).
		Msg("User logged in successfully")

	return ctx.RespondData(&LoginResponse{
		IsConnected: true,
		UserData:    userDto,
	})
}

func IsConnected(ctx *RouteContext) error {
	if !ctx.IsAuthenticated() {
		return ctx.RespondData(&IsConnectedResponse{
			IsConnected: false,
			UserData:    nil,
		})
	}

	userId := ctx.GetUserId()
	userDto, err := ctx.Db().NewUserRepository().GetUserById(context.Background(), userId)
	if err != nil {
		ctx.Logger().Warn(fmt.Sprintf("Failed to get user %d: %s", userId, err.Error()))
		return ctx.RespondData(&IsConnectedResponse{
			IsConnected: false,
			UserData:    nil,
		})
	}

	return ctx.RespondData(&IsConnectedResponse{
		IsConnected: true,
		UserData:    userDto,
	})
}

func Logout(ctx *RouteContext) error {
	sessionId := ctx.GetSessionId()
	userId := ctx.GetUserId()

	userDto, err := ctx.Db().NewUserRepository().GetUserById(context.Background(), userId)
	if err != nil {
		ctx.Logger().Warn(fmt.Sprintf("Failed to get user %d for logout: %s", userId, err.Error()))
	}

	err = ctx.Db().WithTransaction(func(txCtx context.Context, tx *client.Tx) error {
		return ctx.Db().NewSessionRepository().InvalidateSession(txCtx, tx, sessionId, "User logged out")
	})

	if err != nil {
		ctx.Logger().Warn(fmt.Sprintf("Failed to invalidate session %d: %s", sessionId, err.Error()))
	}

	clearSessionCookie(ctx.Ctx(), ctx.ServerConfig())

	logEvent := ctx.Logger().InfoEvent().Int64("sessionId", int64(sessionId))
	if userDto != nil && userDto.Email != nil {
		logEvent.Str("email", *userDto.Email)
	}
	if userDto != nil && userDto.ID != nil {
		logEvent.Int64("userId", int64(*userDto.ID))
	}
	logEvent.Msg("User logged out successfully")

	return ctx.RespondData(&LogoutResponse{
		Success: true,
		Message: "Logged out successfully",
	})
}

func createSessionCookie(token string, rememberMe bool, cfg *config.ServerConfig) *fiber.Cookie {
	return &fiber.Cookie{
		Name:     config.SessionCookieName,
		Value:    token,
		Expires:  utils.CalculateExpiration(rememberMe),
		HTTPOnly: true,
		Secure:   cfg.Security().CookieSecure(),
		SameSite: fiber.CookieSameSiteStrictMode,
		Path:     "/",
	}
}

func clearSessionCookie(ctx fiber.Ctx, cfg *config.ServerConfig) {
	ctx.ClearCookie(config.SessionCookieName)
}
