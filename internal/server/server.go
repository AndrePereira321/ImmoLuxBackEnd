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
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/limiter"
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

	fiberApp := getFiberApp(serverConfig, serverLogger)

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

// getFiberApp builds the Fiber app with its global middleware; routes are added
// afterwards by RegisterRoutes.
func getFiberApp(serverConfig *config.ServerConfig, serverLogger *logger.Logger) *fiber.App {
	fiberConfig := fiber.Config{
		AppName:     serverConfig.AppConfig().Name(),
		JSONEncoder: json.Marshal,
		JSONDecoder: json.Unmarshal,
		BodyLimit:   10 * 1024 * 1024, // 10 MB
	}

	// Behind a reverse proxy every request comes from the proxy's address, so the
	// client IP (used by rate limiting, sessions and auth logs) must come from a
	// header. Only loopback peers are trusted to set it, and Fiber takes the
	// rightmost valid untrusted X-Forwarded-For entry, so entries a client
	// prepends are ignored as long as the proxy adds a plain IP (or overwrites
	// the header with $remote_addr, the recommended setup).
	if proxyHeader := serverConfig.HttpServer().ProxyHeader(); proxyHeader != "" {
		fiberConfig.ProxyHeader = proxyHeader
		fiberConfig.TrustProxy = true
		fiberConfig.TrustProxyConfig = fiber.TrustProxyConfig{Loopback: true}
		fiberConfig.EnableIPValidation = true
		serverLogger.Info(fmt.Sprintf("Client IPs are read from the %s header set by loopback proxies", proxyHeader))
	} else if isLoopbackHost(serverConfig.HttpServer().Host()) {
		serverLogger.Warn("server.proxy_header is not set: if a reverse proxy forwards to this server, every client " +
			"shares the proxy's IP and one client can exhaust the login rate limit for everyone")
	}

	fiberApp := fiber.New(fiberConfig)

	// CORS runs first so that every response, including a 429 from the login
	// limiter below, carries the headers the browser needs to read it.
	if len(serverConfig.HttpServer().Origins()) > 0 {
		fiberApp.Use(cors.New(cors.Config{
			AllowCredentials: true,
			AllowOrigins:     serverConfig.HttpServer().Origins(),
			AllowHeaders:     []string{"Origin", "Content-Type", "Accept"},
			AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		}))
	}

	fiberApp.Use(ApiPrefix+"/login", newLoginLimiter(serverLogger))

	return fiberApp
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	addr, err := netip.ParseAddr(host)
	return err == nil && addr.IsLoopback()
}

const (
	// LoginRequestLimit caps login requests per client (IP, or IPv6 /64) per
	// LoginRequestWindow. It is held in memory (per process), so a flood is
	// rejected before it reaches the database; the database's per-(email, IP)
	// limit still applies on top.
	LoginRequestLimit  = 10
	LoginRequestWindow = time.Minute
)

func newLoginLimiter(serverLogger *logger.Logger) fiber.Handler {
	return newLimiter(serverLogger, LoginRequestLimit, LoginRequestWindow)
}

func newLimiter(serverLogger *logger.Logger, max int, window time.Duration) fiber.Handler {
	rejections := &rejectionReporter{interval: window, log: serverLogger.Warn}
	return limiter.New(limiter.Config{
		Max:          max,
		Expiration:   window,
		KeyGenerator: loginLimiterKey,
		Next: func(ctx fiber.Ctx) bool {
			return ctx.Method() != fiber.MethodPost
		},
		LimitReached: func(ctx fiber.Ctx) error {
			rejections.record(time.Now(), loginLimiterKey(ctx))
			apiError := server_error.RateLimited("RATE_LIMIT_EXCEEDED", "Too many login attempts. Try again later.").ToServerAPIError()
			return ctx.Status(fiber.StatusTooManyRequests).JSON(models.NewServerAPIResponse(false, nil, apiError))
		},
	})
}

// loginLimiterKey buckets IPv6 clients by /64, the block a single subscriber
// usually gets, so rotating addresses within it does not reset the limit.
func loginLimiterKey(ctx fiber.Ctx) string {
	ip := routes.ClientIP(ctx)
	if addr, err := netip.ParseAddr(ip); err == nil && addr.Is6() {
		return netip.PrefixFrom(addr, 64).Masked().String()
	}
	return ip
}

// rejectionReporter logs rate-limit rejections at most once per interval, so a
// flood shows up in the logs without rotating their history away.
type rejectionReporter struct {
	mu        sync.Mutex
	interval  time.Duration
	log       func(string)
	lastLog   time.Time
	unlogged  int
	firstSeen time.Time
}

func (r *rejectionReporter) record(now time.Time, key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.unlogged == 0 {
		r.firstSeen = now
	}
	r.unlogged++
	if !r.lastLog.IsZero() && now.Sub(r.lastLog) < r.interval {
		return
	}
	r.log(fmt.Sprintf("Login rate limit: rejected %d request(s) since %s, latest from %s",
		r.unlogged, r.firstSeen.Format(time.RFC3339), key))
	r.lastLog = now
	r.unlogged = 0
}

func getServerLogger(serverConfig *config.ServerConfig) (*logger.Logger, error) {
	loggingConfig := serverConfig.Logging()
	return logger.New("SERVER", loggingConfig.ServerLogLevel(), loggingConfig.LogDir(), loggingConfig.LogToConsole())
}
