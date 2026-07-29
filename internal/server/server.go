package server

import (
	"context"
	"errors"
	"fmt"
	"immo-lux/internal/config"
	"immo-lux/internal/database"
	"immo-lux/internal/logger"
	"immo-lux/internal/models"
	"immo-lux/internal/server/routes"
	"immo-lux/internal/server_error"
	"strconv"

	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
)

const ApiPrefix = "/v1/api"

type Server struct {
	config *config.ServerConfig
	fiber  *fiber.App
	db     *database.Database
	logger *logger.Logger
}

func New(serverConfig *config.ServerConfig) (*Server, error) {
	serverLogger, err := getServerLogger(serverConfig)
	if err != nil {
		return nil, server_error.Wrap("SERVER_INIT", "failed creating server logger", err)
	}

	db, err := database.New(serverConfig)
	if err != nil {
		return nil, server_error.Wrap("SERVER_INIT", "failed creating database", err)
	}

	if err = db.Init(); err != nil {
		return nil, server_error.Wrap("SERVER_INIT", "failed initializing database", err)
	}

	fiberApp := getFiberApp(serverConfig)

	if len(serverConfig.HttpServer().Origins()) > 0 {
		fiberApp.Use(cors.New(cors.Config{
			AllowCredentials: true,
			AllowOrigins:     serverConfig.HttpServer().Origins(),
			AllowHeaders:     []string{"Origin", "Content-Type", "Accept"},
			AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		}))
	}

	return &Server{
		config: serverConfig,
		fiber:  fiberApp,
		db:     db,
		logger: serverLogger,
	}, nil
}

func (s *Server) Listen() error {
	addr := s.listenAddress()
	s.logger.Info("Starting listening server on " + addr)
	return s.fiber.Listen(addr, fiber.ListenConfig{
		EnablePrefork: s.config.HttpServer().EnablePreFork(),
	})
}

func (s *Server) Get(path string, handler RouteHandler) {
	s.register(s.fiber.Get, path, handler, false)
}

func (s *Server) Post(path string, handler RouteHandler) {
	s.register(s.fiber.Post, path, handler, false)
}

func (s *Server) Put(path string, handler RouteHandler) {
	s.register(s.fiber.Put, path, handler, false)
}

func (s *Server) Delete(path string, handler RouteHandler) {
	s.register(s.fiber.Delete, path, handler, false)
}

func (s *Server) SecuredGet(path string, handler RouteHandler) {
	s.register(s.fiber.Get, path, handler, true)
}

func (s *Server) SecuredPost(path string, handler RouteHandler) {
	s.register(s.fiber.Post, path, handler, true)
}

func (s *Server) SecuredPut(path string, handler RouteHandler) {
	s.register(s.fiber.Put, path, handler, true)
}

func (s *Server) SecuredDelete(path string, handler RouteHandler) {
	s.register(s.fiber.Delete, path, handler, true)
}

type fiberRegisterFunc func(path string, handler any, handlers ...any) fiber.Router

func (s *Server) register(method fiberRegisterFunc, path string, handler RouteHandler, requireAuth bool) {
	method(ApiPrefix+path, func(ctx fiber.Ctx) error {
		return s.handleRoute(ctx, handler, requireAuth)
	})
}

func (s *Server) handleRoute(ctx fiber.Ctx, handler RouteHandler, requireAuth bool) error {
	routeContext := routes.GetRouteContext(s.logger, ctx, s.db, s.config)
	if requireAuth && !routeContext.IsAuthenticated() {
		authError := routeContext.AuthError()
		if authError == nil {
			authError = server_error.Unauthorized("UNAUTHORIZED", "Authentication required")
		}
		return handleServerError(routeContext, authError)
	}

	if err := handler(routeContext); err != nil {
		return handleServerError(routeContext, err)
	}
	return nil
}

func handleServerError(ctx *routes.RouteContext, err error) error {
	var serverError *server_error.ServerError
	if !errors.As(err, &serverError) {
		serverError = server_error.Wrap("SERVER_ERROR", err.Error(), err)
	}

	status := httpStatus(serverError.Kind)
	request := ctx.Ctx().Method() + " " + ctx.Ctx().Path()
	if status >= fiber.StatusInternalServerError {
		ctx.Logger().Error(fmt.Sprintf("%s failed: %s", request, serverError.Error()))
	} else {
		ctx.Logger().Debug(fmt.Sprintf("%s rejected (%d): %s", request, status, serverError.Error()))
	}

	apiResponse := models.NewServerAPIResponse(false, nil, serverError.ToServerAPIError())
	return ctx.Respond(status, apiResponse)
}

func httpStatus(kind server_error.Kind) int {
	switch kind {
	case server_error.KindInvalid:
		return fiber.StatusBadRequest
	case server_error.KindUnauthorized:
		return fiber.StatusUnauthorized
	case server_error.KindForbidden:
		return fiber.StatusForbidden
	case server_error.KindNotFound:
		return fiber.StatusNotFound
	case server_error.KindRateLimited:
		return fiber.StatusTooManyRequests
	default:
		return fiber.StatusInternalServerError
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Debug("Shutting down server")

	if err := s.fiber.ShutdownWithContext(ctx); err != nil {
		s.logger.Error("Failed to shut down server: " + err.Error())
		return server_error.Wrap("SERVER_SHUTDOWN", "failed shutting down fiber server", err)
	}

	s.logger.Info("Server shut down successfully")
	return nil
}

func (s *Server) listenAddress() string {
	return s.config.HttpServer().Host() + ":" + strconv.FormatUint(uint64(s.config.HttpServer().Port()), 10)
}

func (s *Server) Close() []error {
	s.logger.Debug("Closing server resources")
	var errors []error
	s.logger.Trace("Closing database connection")
	if s.db != nil {
		err := s.db.Close()
		if err != nil {
			errors = append(errors, server_error.Wrap("SERVER_CLOSE", "failed closing database", err))
		}
	}
	s.logger.Trace("Closing logger files")
	if s.logger != nil {
		err := s.logger.Close()
		if err != nil {
			errors = append(errors, server_error.Wrap("SERVER_CLOSE", "failed closing logger", err))
		}
	}
	return errors
}

func getFiberApp(serverConfig *config.ServerConfig) *fiber.App {
	return fiber.New(fiber.Config{
		AppName:     serverConfig.AppConfig().Name(),
		JSONEncoder: json.Marshal,
		JSONDecoder: json.Unmarshal,
		BodyLimit:   10 * 1024 * 1024, // 10 MB
	})
}

func getServerLogger(serverConfig *config.ServerConfig) (*logger.Logger, error) {
	level := serverConfig.Logging().ServerLogLevel()
	dir := serverConfig.Logging().LogDir()
	return logger.New("SERVER", level, dir)
}
