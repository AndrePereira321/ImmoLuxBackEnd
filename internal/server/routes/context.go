package routes

import (
	"context"
	"fmt"
	"net/netip"
	"strconv"

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

type RouteContext struct {
	logger       *logger.Logger
	ctx          fiber.Ctx
	userContext  *UserContext
	authError    *server_error.ServerError
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

// ClientIP is the request's client address; see ClientIP.
func (route *RouteContext) ClientIP() string {
	return ClientIP(route.ctx)
}

// ClientIP normalises the address Fiber resolved (from the TCP peer, or the
// trusted proxy header): IPv4-mapped IPv6 is unmapped and zones are dropped, so
// one client always yields the same string and it fits the 45-char ip_address
// columns. A value that does not parse as an IP falls back to the TCP peer.
func ClientIP(ctx fiber.Ctx) string {
	if addr, err := netip.ParseAddr(ctx.IP()); err == nil {
		return addr.Unmap().WithZone("").String()
	}
	if ip := ctx.RequestCtx().RemoteIP(); ip != nil {
		return ip.String()
	}
	return ""
}

// RequestContext returns the context database operations run under. Fiber v3
// hands out context.Background() unless one was set with SetContext, so it is
// not cancelled when the client disconnects.
func (route *RouteContext) RequestContext() context.Context {
	return route.ctx.Context()
}

func (route *RouteContext) IsAuthenticated() bool {
	return route.userContext != nil
}

// AuthError returns why authentication failed — always an Unauthorized-kinded
// error, nil when the request is authenticated.
func (route *RouteContext) AuthError() *server_error.ServerError {
	return route.authError
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

func (route *RouteContext) RespondData(data any) error {
	return route.Respond(fiber.StatusOK, models.NewServerAPIResponse(true, data, nil))
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
		return server_error.BadRequest("invalid request body").WithCause(err)
	}
	return nil
}

// ParseIdParam parses a route parameter as a RecordId.
func (route *RouteContext) ParseIdParam(paramName string) (models.RecordId, error) {
	idStr := route.ctx.Params(paramName)
	idInt, err := strconv.Atoi(idStr)
	if err != nil {
		return models.InvalidRecordId, server_error.BadRequest("Invalid " + paramName + " ID")
	}
	return models.RecordId(idInt), nil
}

// RequireUserId returns the authenticated user's ID or an unauthorized error.
func (route *RouteContext) RequireUserId() (models.RecordId, error) {
	userId := route.GetUserId()
	if !userId.IsValid() {
		return models.InvalidRecordId, server_error.Unauthorized("UNAUTHORIZED", "User not authenticated")
	}
	return userId, nil
}

func extractUserContext(ctx fiber.Ctx, db *database.Database, serverConfig *config.ServerConfig, logger *logger.Logger) (*UserContext, *server_error.ServerError) {
	jwtToken := ctx.Cookies(config.SessionCookieName)
	if jwtToken == "" {
		return nil, server_error.Unauthorized("COOKIE_MISSING", "Session cookie not found")
	}

	jwtSecret := serverConfig.Security().JwtSecret()
	claims, err := utils.ValidateJWT(jwtToken, jwtSecret)
	if err != nil {
		logger.Debug(fmt.Sprintf("Invalid JWT token: %s", err.Error()))
		clearSessionCookie(ctx, serverConfig)

		if server_error.IsServerError(err, "JWT_EXPIRED") {
			return nil, server_error.Unauthorized("JWT_EXPIRED", "Session token has expired")
		}
		return nil, server_error.Unauthorized("JWT_INVALID", "Session token is invalid")
	}

	sessionId := models.RecordId(claims.SessionId)
	sessionRepo := db.NewSessionRepository()

	isValid, err := sessionRepo.ValidateSession(ctx.Context(), sessionId)
	if err != nil {
		logger.Warn(fmt.Sprintf("Error validating session: %s", err.Error()))

		if server_error.IsServerError(err, "SESSION_NOT_FOUND") {
			clearSessionCookie(ctx, serverConfig)
			return nil, server_error.Unauthorized("SESSION_NOT_FOUND", "Session not found")
		}

		return nil, server_error.Unauthorized("SESSION_VALIDATION_ERROR", "Failed to validate session")
	}

	if !isValid {
		logger.Debug(fmt.Sprintf("Session %d is not valid or expired", sessionId))
		clearSessionCookie(ctx, serverConfig)

		return nil, server_error.Unauthorized("SESSION_INVALID", "Session is expired or has been invalidated")
	}

	userId := models.RecordId(claims.UserId)

	return &UserContext{
		UserID:    userId,
		SessionID: sessionId,
	}, nil
}
