package routes

import (
	"immo-lux/internal/database"

	"github.com/gofiber/fiber/v3"
)

type UserContext struct {
	userId    uint64
	sessionId uint64
}
type RouteContext struct {
	ctx         *fiber.Ctx
	userContext *UserContext
	db          *database.Database
}

func GetRouteContext(ctx *fiber.Ctx, db *database.Database) (*RouteContext, error) {
	//TODO Retrieve user context from DB
	return &RouteContext{
		ctx:         ctx,
		userContext: nil,
		db:          db,
	}, nil
}

// TODO Temporary
func (route *RouteContext) Respond(obj any) error {
	c := *route.ctx
	err := c.JSON(obj)
	if err != nil {
		return err
	}

	return c.SendStatus(200)
}

func (route *RouteContext) DbPing() bool {
	return route.db.Ping()
}
