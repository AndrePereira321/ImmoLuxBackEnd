package config

import (
	"bytes"
	"fmt"
	"immo-lux/internal/server_error"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

type ServerConfig struct {
	appConfig  *AppConfig
	httpServer *HttpConfig
	database   *DatabaseConfig
	logging    *LoggingConfig
	security   *SecurityConfig
}

func (c *ServerConfig) AppConfig() *AppConfig {
	return c.appConfig
}

func (c *ServerConfig) HttpServer() *HttpConfig {
	return c.httpServer
}

func (c *ServerConfig) Logging() *LoggingConfig {
	return c.logging
}

func (c *ServerConfig) Database() *DatabaseConfig {
	return c.database
}

func (c *ServerConfig) Security() *SecurityConfig {
	return c.security
}

type AppConfig struct {
	name    string
	version *AppVersion
}

func (a *AppConfig) Name() string {
	return a.name
}

func (a *AppConfig) Version() *AppVersion {
	return a.version
}

type HttpConfig struct {
	host          string
	port          uint
	enablePreFork bool
	origins       []string
	proxyHeader   string
	spaFolder     string
}

func (s *HttpConfig) Host() string {
	return s.host
}

func (s *HttpConfig) Port() uint {
	return s.port
}

func (s *HttpConfig) EnablePreFork() bool {
	return s.enablePreFork
}

func (s *HttpConfig) Origins() []string {
	return s.origins
}

// ProxyHeader is the header carrying the client IP when requests arrive through
// a reverse proxy on loopback (e.g. "X-Forwarded-For"); empty means no proxy.
func (s *HttpConfig) ProxyHeader() string {
	return s.proxyHeader
}

func (s *HttpConfig) SpaFolder() string {
	return s.spaFolder
}

type DatabaseConfig struct {
	maxOpenCons  int
	maxIdleCons  int
	userFilePath string
	host         string
	port         int
	username     string
	password     string
	dbName       string
	sslMode      string
}

func (d *DatabaseConfig) MaxOpenCons() int {
	return d.maxOpenCons
}

func (d *DatabaseConfig) MaxIdleCons() int {
	return d.maxIdleCons
}

func (d *DatabaseConfig) UserFilePath() string {
	return d.userFilePath
}

func (d *DatabaseConfig) Host() string {
	return d.host
}

func (d *DatabaseConfig) Port() int {
	return d.port
}

func (d *DatabaseConfig) Username() string {
	return d.username
}

func (d *DatabaseConfig) Password() string {
	return d.password
}

func (d *DatabaseConfig) DbName() string {
	return d.dbName
}

func (d *DatabaseConfig) SslMode() string {
	return d.sslMode
}

func (d *DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		d.username, d.password, d.host, d.port, d.dbName, d.sslMode)
}

type LoggingConfig struct {
	logDir           string
	logToConsole     bool
	serverLogLevel   string
	databaseLogLevel string
}

func (l *LoggingConfig) LogDir() string {
	return l.logDir
}

func (l *LoggingConfig) LogToConsole() bool {
	return l.logToConsole
}

func (l *LoggingConfig) ServerLogLevel() string {
	return l.serverLogLevel
}

func (l *LoggingConfig) DatabaseLogLevel() string {
	return l.databaseLogLevel
}

type SecurityConfig struct {
	jwtSecret    string
	jwtIssuer    string
	cookieSecure bool
}

func (s *SecurityConfig) JwtSecret() string {
	return s.jwtSecret
}

func (s *SecurityConfig) JwtIssuer() string {
	return s.jwtIssuer
}

func (s *SecurityConfig) CookieSecure() bool {
	return s.cookieSecure
}

func GetServerConfig(data []byte) (*ServerConfig, error) {
	serverConfig := &ServerConfig{}
	v := getViper()

	if err := v.ReadConfig(bytes.NewReader(data)); err != nil {
		return nil, server_error.Wrap("CONFIG_PARSER", "error when reading config data", err)
	}

	appConfig, err := getAppConfig(v)
	if err != nil {
		return nil, err
	}
	serverConfig.appConfig = appConfig

	httpConfig, err := getHttpConfig(v)
	if err != nil {
		return nil, err
	}
	serverConfig.httpServer = httpConfig

	databaseConfig, err := getDatabaseConfig(v)
	if err != nil {
		return nil, err
	}
	serverConfig.database = databaseConfig

	loggingConfig, err := getLoggingConfig(v)
	if err != nil {
		return nil, err
	}
	serverConfig.logging = loggingConfig

	securityConfig, err := getSecurityConfig(v)
	if err != nil {
		return nil, err
	}
	serverConfig.security = securityConfig

	return serverConfig, nil
}

func getHttpConfig(v *viper.Viper) (*HttpConfig, error) {
	httpConfig := HttpConfig{}

	httpConfig.host = v.GetString("server.host")
	if len(httpConfig.host) == 0 {
		return nil, server_error.New("CONFIG_PARSER", "http server host is empty")
	}

	httpConfig.port = v.GetUint("server.port")
	if httpConfig.port == 0 {
		httpConfig.port = 80
	}

	httpConfig.enablePreFork = v.GetBool("server.enable_pre_fork")

	originStr := v.GetString("server.origin")
	if len(originStr) > 0 {
		parts := strings.Split(originStr, ",")
		origins := make([]string, 0, len(parts))
		for _, part := range parts {
			origins = append(origins, strings.TrimSpace(part))
		}
		httpConfig.origins = origins
	}

	httpConfig.proxyHeader = strings.TrimSpace(v.GetString("server.proxy_header"))
	if strings.EqualFold(httpConfig.proxyHeader, "Forwarded") {
		// RFC 7239 "for=..." syntax is not parsed; every client would silently
		// fall back to the proxy's address.
		return nil, server_error.New("CONFIG_PARSER", "server.proxy_header \"Forwarded\" is not supported; use X-Forwarded-For or X-Real-IP")
	}

	httpConfig.spaFolder = v.GetString("server.spaFolder")

	return &httpConfig, nil
}

func getLoggingConfig(v *viper.Viper) (*LoggingConfig, error) {
	loggingConfig := LoggingConfig{}

	loggingConfig.logDir = strings.TrimSpace(v.GetString("logging.log_dir"))

	// Parsed strictly: viper's GetBool turns any unparseable value ("yes", "on")
	// into false, which would silently switch console logging off.
	loggingConfig.logToConsole = true
	if v.IsSet("logging.log_to_console") {
		switch value := v.Get("logging.log_to_console").(type) {
		case bool:
			loggingConfig.logToConsole = value
		case string:
			parsed, err := strconv.ParseBool(strings.TrimSpace(value))
			if err != nil {
				return nil, server_error.New("CONFIG_PARSER", fmt.Sprintf("logging.log_to_console must be true or false, got %q", value))
			}
			loggingConfig.logToConsole = parsed
		default:
			return nil, server_error.New("CONFIG_PARSER", fmt.Sprintf("logging.log_to_console must be true or false, got %v", value))
		}
	}
	if !loggingConfig.logToConsole && len(loggingConfig.logDir) == 0 {
		return nil, server_error.New("CONFIG_PARSER", "logging.log_to_console is disabled but logging.log_dir is empty")
	}

	loggingConfig.serverLogLevel = v.GetString("logging.server_log_level")
	if len(loggingConfig.serverLogLevel) == 0 {
		loggingConfig.serverLogLevel = "INFO"
	}

	loggingConfig.databaseLogLevel = v.GetString("logging.database_log_level")
	if len(loggingConfig.databaseLogLevel) == 0 {
		loggingConfig.databaseLogLevel = "INFO"
	}

	return &loggingConfig, nil
}

func getDatabaseConfig(v *viper.Viper) (*DatabaseConfig, error) {
	databaseConfig := DatabaseConfig{}

	databaseConfig.maxOpenCons = v.GetInt("database.max_open_connections")
	if databaseConfig.maxOpenCons <= 0 {
		databaseConfig.maxOpenCons = 25
	}

	databaseConfig.maxIdleCons = v.GetInt("database.max_idle_connections")
	if databaseConfig.maxIdleCons <= 0 {
		databaseConfig.maxIdleCons = 25
	}

	databaseConfig.userFilePath = v.GetString("database.user_file_path")

	databaseConfig.host = v.GetString("database.host")
	if len(databaseConfig.host) == 0 {
		return nil, server_error.New("CONFIG_PARSER", "database host is empty")
	}

	databaseConfig.port = v.GetInt("database.port")
	if databaseConfig.port == 0 {
		databaseConfig.port = 5432
	}

	databaseConfig.username = v.GetString("database.username")
	if len(databaseConfig.username) == 0 {
		return nil, server_error.New("CONFIG_PARSER", "database username is empty")
	}

	databaseConfig.password = v.GetString("database.password")
	if len(databaseConfig.password) == 0 {
		return nil, server_error.New("CONFIG_PARSER", "database password is empty")
	}

	databaseConfig.dbName = v.GetString("database.db_name")
	if len(databaseConfig.dbName) == 0 {
		return nil, server_error.New("CONFIG_PARSER", "database name is empty")
	}

	databaseConfig.sslMode = v.GetString("database.ssl_mode")
	if len(databaseConfig.sslMode) == 0 {
		databaseConfig.sslMode = "disable"
	}

	return &databaseConfig, nil
}

func getAppConfig(v *viper.Viper) (*AppConfig, error) {
	appConfig := AppConfig{}

	appConfig.name = v.GetString("app.name")
	if len(appConfig.name) == 0 {
		return nil, server_error.New("CONFIG_PARSER", "app name is empty")
	}

	version, err := NewAppVersion(v.GetString("app.version"))
	if err != nil {
		return nil, err
	}
	appConfig.version = version

	return &appConfig, nil
}

func getSecurityConfig(v *viper.Viper) (*SecurityConfig, error) {
	securityConfig := SecurityConfig{}

	securityConfig.jwtSecret = v.GetString("security.jwt_secret")
	if len(securityConfig.jwtSecret) == 0 {
		return nil, server_error.New("CONFIG_PARSER", "JWT secret is empty")
	}
	if len(securityConfig.jwtSecret) < 32 {
		return nil, server_error.New("CONFIG_PARSER", "JWT secret must be at least 32 characters")
	}

	securityConfig.jwtIssuer = v.GetString("security.jwt_issuer")
	if len(securityConfig.jwtIssuer) == 0 {
		securityConfig.jwtIssuer = "immolux"
	}

	securityConfig.cookieSecure = v.GetBool("security.cookie_secure")

	return &securityConfig, nil
}

func getViper() *viper.Viper {
	v := viper.New()
	v.SetConfigType("toml")
	return v
}
