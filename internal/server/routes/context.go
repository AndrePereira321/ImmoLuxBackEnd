package routes

import (
	"context"
	"fmt"
	"immo-lux/internal/config"
	"immo-lux/internal/database"
	"immo-lux/internal/logger"
	"immo-lux/internal/models"
	"immo-lux/internal/server_error"
	"immo-lux/internal/utils"

	"github.com/gofiber/fiber/v3"
)

type UserContext struct {
	UserID    models.RecordId
	SessionID models.RecordId
}

type AuthError struct {
	Code    string
	Message string
}

type RouteContext struct {
	logger       *logger.Logger
	ctx          fiber.Ctx
	userContext  *UserContext
	authError    *AuthError
	db           *database.Database
	serverConfig *config.ServerConfig
}

func GetRouteContext(logger *logger.Logger, ctx fiber.Ctx, db *database.Database, serverConfig *config.ServerConfig) *RouteContext {
	userContext, authError := extractUserContext(ctx, db, serverConfig, logger)

	return &RouteContext{
		logger:       logger,
		ctx:          ctx,
		userContext:  userContext,
		authError:    authError,
		db:           db,
		serverConfig: serverConfig,
	}
}

func (route *RouteContext) Logger() *logger.Logger {
	return route.logger
}

func (route *RouteContext) Db() *database.Database {
	return route.db
}

func (route *RouteContext) ServerConfig() *config.ServerConfig {
	return route.serverConfig
}

func (route *RouteContext) Ctx() fiber.Ctx {
	return route.ctx
}

func (route *RouteContext) IsAuthenticated() bool {
	return route.userContext != nil
}

func (route *RouteContext) GetAuthError() *AuthError {
	return route.authError
}

func (route *RouteContext) UserContext() *UserContext {
	return route.userContext
}

func (route *RouteContext) GetUserId() models.RecordId {
	if route.userContext == nil {
		return models.InvalidRecordId
	}
	return route.userContext.UserID
}

func (route *RouteContext) GetSessionId() models.RecordId {
	if route.userContext == nil {
		return models.InvalidRecordId
	}
	return route.userContext.SessionID
}

func (route *RouteContext) InternalError(msg string) error {
	return route.RespondError(fiber.StatusInternalServerError, "INTERNAL_ERROR", msg)
}

func (route *RouteContext) BadRequest(msg string) error {
	return route.RespondError(fiber.StatusBadRequest, "BAD_REQUEST", msg)
}

func (route *RouteContext) RespondData(data any) error {
	return route.Respond(fiber.StatusOK, models.NewServerAPIResponse(true, data, nil))
}

func (route *RouteContext) RespondError(status int, code string, message string) error {
	return route.Respond(status, models.NewServerAPIResponse(false, nil, models.NewServerAPIError(code, message)))
}

func (route *RouteContext) Respond(status int, response *models.ServerAPIResponse) error {
	err := route.ctx.Status(status).JSON(response)
	if err != nil {
		route.logger.Warn(fmt.Sprintf("Failed to respond: %s", err.Error()))
		return server_error.Wrap("ROUTE_CONTEXT", "failed to respond", err)
	}
	return nil
}

func (route *RouteContext) ReadBody(body any) error {
	err := route.ctx.Bind().JSON(body)
	if err != nil {
		return server_error.Wrap("ROUTE_HANDLER", "failed to read body", err)
	}
	return nil
}

func extractUserContext(ctx fiber.Ctx, db *database.Database, serverConfig *config.ServerConfig, logger *logger.Logger) (*UserContext, *AuthError) {
	jwtToken := ctx.Cookies(config.SessionCookieName)
	if jwtToken == "" {
		return nil, &AuthError{
			Code:    "COOKIE_MISSING",
			Message: "Session cookie not found",
		}
	}

	jwtSecret := serverConfig.Security().JwtSecret()
	claims, err := utils.ValidateJWT(jwtToken, jwtSecret)
	if err != nil {
		logger.Debug(fmt.Sprintf("Invalid JWT token: %s", err.Error()))

		// Clear invalid cookie
		clearSessionCookie(ctx, serverConfig)

		if server_error.IsServerError(err, "JWT_VALIDATION") {
			serverErr := err.(*server_error.ServerError)
			if serverErr.Contains("expired") {
				return nil, &AuthError{
					Code:    "JWT_EXPIRED",
					Message: "Session token has expired",
				}
			}
			return nil, &AuthError{
				Code:    "JWT_INVALID",
				Message: "Session token is invalid",
			}
		}
		return nil, &AuthError{
			Code:    "JWT_INVALID",
			Message: "Session token is invalid",
		}
	}

	sessionId := models.RecordId(claims.SessionId)
	sessionRepo := db.NewSessionRepository()

	isValid, err := sessionRepo.ValidateSession(context.Background(), sessionId)
	if err != nil {
		logger.Warn(fmt.Sprintf("Error validating session: %s", err.Error()))

		// Clear cookie if session not found in database
		if server_error.IsServerError(err, "SESSION_NOT_FOUND") {
			clearSessionCookie(ctx, serverConfig)
			return nil, &AuthError{
				Code:    "SESSION_NOT_FOUND",
				Message: "Session not found",
			}
		}

		return nil, &AuthError{
			Code:    "SESSION_VALIDATION_ERROR",
			Message: "Failed to validate session",
		}
	}

	if !isValid {
		logger.Debug(fmt.Sprintf("Session %d is not valid or expired", sessionId))

		// Clear cookie if session is invalid/expired
		clearSessionCookie(ctx, serverConfig)

		return nil, &AuthError{
			Code:    "SESSION_INVALID",
			Message: "Session is expired or has been invalidated",
		}
	}

	userId := models.RecordId(claims.UserId)

	return &UserContext{
		UserID:    userId,
		SessionID: sessionId,
	}, nil
}
