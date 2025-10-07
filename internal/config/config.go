package config

import (
	"bytes"
	"immo-lux/internal/server_error"

	"github.com/spf13/viper"
)

type ServerConfig struct {
	appConfig  *AppConfig
	httpServer *HttpConfig
	database   *DatabaseConfig
	logging    *LoggingConfig
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
	origin        string
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

func (s *HttpConfig) Origin() string {
	return s.origin
}

type DatabaseConfig struct {
	maxOpenCons  int
	maxIdleCons  int
	userFilePath string
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

type LoggingConfig struct {
	logDir           string
	serverLogLevel   string
	databaseLogLevel string
}

func (l *LoggingConfig) LogDir() string {
	return l.logDir
}

func (l *LoggingConfig) ServerLogLevel() string {
	return l.serverLogLevel
}

func (l *LoggingConfig) DatabaseLogLevel() string {
	return l.databaseLogLevel
}

func GetServerConfig(data []byte) (*ServerConfig, error) {
	serverConfig := &ServerConfig{}
	v := getViper()

	if err := v.ReadConfig(bytes.NewReader(data)); err != nil {
		return nil, server_error.Wrap("CONFIG_PARSER", "error when reading config data", err)
	}

	if appConfig, err := getAppConfig(v); err != nil {
		return nil, err
	} else {
		serverConfig.appConfig = appConfig
	}

	if httpConfig, err := getHttpConfig(v); err != nil {
		return nil, err
	} else {
		serverConfig.httpServer = httpConfig
	}

	if databaseConfig, err := getDatabaseConfig(v); err != nil {
		return nil, err
	} else {
		serverConfig.database = databaseConfig
	}

	if loggingConfig, err := getLoggingConfig(v); err != nil {
		return nil, err
	} else {
		serverConfig.logging = loggingConfig
	}

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
	httpConfig.origin = v.GetString("server.origin")

	return &httpConfig, nil
}

func getLoggingConfig(v *viper.Viper) (*LoggingConfig, error) {
	loggingConfig := LoggingConfig{}

	loggingConfig.logDir = v.GetString("logging.log_dir")

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

func getViper() *viper.Viper {
	v := viper.New()
	v.SetConfigType("toml")
	return v
}
