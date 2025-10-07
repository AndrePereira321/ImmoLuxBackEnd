package routes

import (
	"fmt"
	"immo-lux/internal/database"
	"immo-lux/internal/logger"
	"immo-lux/internal/models"
	"immo-lux/internal/server_error"

	"github.com/gofiber/fiber/v3"
)

type UserContext struct {
	userId    uint64
	sessionId uint64
}
type RouteContext struct {
	logger      *logger.Logger
	ctx         fiber.Ctx
	userContext *UserContext
	db          *database.Database
}

func GetRouteContext(logger *logger.Logger, ctx fiber.Ctx, db *database.Database) (*RouteContext, error) {
	//TODO Retrieve user context from DB
	return &RouteContext{
		logger:      logger,
		ctx:         ctx,
		userContext: nil,
		db:          db,
	}, nil
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

func (route *RouteContext) DbPing() bool {
	return route.db.Ping()
}
