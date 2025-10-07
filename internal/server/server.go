package server

import (
	"context"
	"immo-lux/internal/config"
	"immo-lux/internal/database"
	"immo-lux/internal/logger"
	"immo-lux/internal/server/routes"
	"immo-lux/internal/server_error"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
)

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

	if serverConfig.HttpServer().Origin() != "" {
		fiberApp.Use(cors.New(cors.Config{
			AllowCredentials: true,
			AllowOrigins:     []string{serverConfig.HttpServer().Origin()},
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
	listenAddress := s.listenAddress()

	s.logger.Info("Starting listening server on " + listenAddress)
	return s.fiber.Listen(s.listenAddress(), fiber.ListenConfig{
		EnablePrefork: s.config.HttpServer().EnablePreFork(),
	})
}

func (s *Server) Get(path string, handler RouteHandler) {
	s.fiber.Get(path, func(ctx fiber.Ctx) error {
		return s.handleRoute(ctx, handler)
	})
}

func (s *Server) Post(path string, handler RouteHandler) {
	s.fiber.Post(path, func(ctx fiber.Ctx) error {
		return s.handleRoute(ctx, handler)
	})
}

func (s *Server) handleRoute(ctx fiber.Ctx, handler RouteHandler) error {
	routeContext, err := routes.GetRouteContext(ctx, s.db)
	if err != nil {
		return err
	}
	return handler(routeContext)
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
			errors = append(errors, server_error.Wrap("SERVER_CLOSE", "failed closing database", err))
		}
	}
	return errors
}

func getFiberApp(serverConfig *config.ServerConfig) *fiber.App {
	return fiber.New(fiber.Config{
		AppName: serverConfig.AppConfig().Name(),
	})
}

func getServerLogger(serverConfig *config.ServerConfig) (*logger.Logger, error) {
	level := serverConfig.Logging().ServerLogLevel()
	dir := serverConfig.Logging().LogDir()
	return logger.New("SERVER", level, dir)
}
