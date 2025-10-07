package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"immo-lux/internal/config"
	"immo-lux/internal/logger"
	"immo-lux/internal/server_error"
	"immo-lux/internal/utils"
	"net/url"
	"os"

	"github.com/amacneil/dbmate/v2/pkg/dbmate"
	_ "modernc.org/sqlite"
)

type Database struct {
	serverConfig *config.ServerConfig
	db           *sql.DB
	logger       *logger.Logger
}

func New(serverConfig *config.ServerConfig) (*Database, error) {
	dbLogger, err := logger.New("DATABASE", serverConfig.Logging().DatabaseLogLevel(), serverConfig.Logging().LogDir())
	if err != nil {
		return nil, err
	}

	dbURL := os.Getenv(config.ImmoLuxDbUrl)
	if dbURL == "" {
		return nil, server_error.New("DB_UPGRADE", "database URL environment variable is not set")
	}

	dbLogger.Info("Initializing database.")
	err = upgradeStructure(serverConfig, dbLogger, dbURL)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbURL)
	if err != nil {
		return nil, server_error.Wrap("DB_CONNECT", "failed to open database", err)
	}

	db.SetMaxIdleConns(serverConfig.Database().MaxIdleCons())
	db.SetMaxOpenConns(serverConfig.Database().MaxOpenCons())

	err = db.Ping()
	if err != nil {
		_ = db.Close()
		return nil, server_error.Wrap("DB_CONNECT", "failed to ping database", err)
	}

	dbLogger.Info("Database successfully initialized.")

	return &Database{
		serverConfig: serverConfig,
		db:           db,
		logger:       dbLogger,
	}, nil
}

func (db *Database) Logger() *logger.Logger {
	return db.logger
}

func (db *Database) Init() error {
	err := db.initUsers()
	if err != nil {
		db.logger.Warn(fmt.Sprintf("Failed to initialize users. Error: %s", err.Error()))
		return err
	}
	return nil
}

func (db *Database) NewUserRepository() *UserRepository {
	return NewUserRepository(db)
}

func (db *Database) WithTransaction(fn func(ctx context.Context, tx *sql.Tx) error) error {
	ctx := context.Background()
	tx, err := db.db.BeginTx(ctx, nil)
	if err != nil {
		return server_error.Wrap("DB_TX", "failed to start transaction", err)
	}

	err = fn(ctx, tx)
	if err != nil {
		db.logger.Warn(fmt.Sprintf("Error during transaction. Rolling back. Error: [%s]", err.Error()))
		_ = tx.Rollback()
		return err
	}

	err = tx.Commit()
	if err != nil {
		db.logger.Warn(fmt.Sprintf("Error during transaction commit. Error: [%s]", err.Error()))
		_ = tx.Rollback()
		return server_error.Wrap("DB_TX", "error commiting transaction", err)
	}
	return nil
}

func (db *Database) Query(query string, args ...any) (*sql.Rows, error) {
	ctx := context.Background()
	rows, err := db.db.QueryContext(ctx, query, args...)
	if err != nil {
		db.logger.Warn(fmt.Sprintf("Error during query. Error: [%s]", err.Error()))
		return nil, server_error.Wrap("DB_QUERY", "error when executing query", err)
	}
	return rows, nil
}

func (db *Database) Close() error {
	err := db.db.Close()
	if err != nil {
		return server_error.Wrap("DB_CLOSE", "error when closing database", err)
	}
	return db.logger.Close()
}

//go:embed migrations/*.sql
var fs embed.FS

func upgradeStructure(serverConfig *config.ServerConfig, dbLogger *logger.Logger, dbURL string) error {
	dbLogger.Debug("Upgrading database structure")

	parsedURL := &url.URL{
		Scheme: "sqlite",
		Opaque: dbURL,
	}

	dbMateLogFilePath, err := url.JoinPath(serverConfig.Logging().LogDir(), "DB_UPGRADE.log")
	if err != nil {
		return server_error.Wrap("DB_UPGRADE", "failed to create DBMate log file path\n", err)
	}

	dbMateLogFile, err := os.OpenFile(dbMateLogFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return server_error.Wrap("DB_UPGRADE", "failed to open DBMate log file", err)
	}

	defer func(dbMateFile *os.File) {
		_ = dbMateFile.Close()
	}(dbMateLogFile)

	dbMate := dbmate.New(parsedURL)
	dbMate.FS = fs
	dbMate.Verbose = true
	dbMate.Strict = true
	dbMate.Log = dbMateLogFile
	dbMate.MigrationsDir = []string{"migrations"}

	migrations, err := dbMate.FindMigrations()
	if err != nil {
		return server_error.Wrap("DB_UPGRADE", "failed to find migrations", err)
	}

	for _, migration := range migrations {
		dbLogger.Trace(fmt.Sprintf("Found migration %s with version %s. Applied: %t", migration.FileName, migration.Version, migration.Applied))
	}

	dbLogger.Trace("Applying migrations.")
	err = dbMate.Migrate()
	if err != nil {
		return server_error.Wrap("DB_UPGRADE", "failed to apply migrations", err)
	}

	dbLogger.Debug("Database structure upgrade completed successfully")
	return nil
}

func (db *Database) Ping() bool {
	return db.db.Ping() == nil
}

func (db *Database) initUsers() error {
	standardUsersFilePath := db.serverConfig.Database().UserFilePath()
	standardUsers := utils.ReadStandardUsers(standardUsersFilePath)
	if len(standardUsers) == 0 {
		return nil
	}

	userRepository := db.NewUserRepository()
	err := db.WithTransaction(func(ctx context.Context, tx *sql.Tx) error {
		for _, standardUser := range standardUsers {
			existsResult, err := tx.Query("SELECT 1 FROM user WHERE email = ?", standardUser.Email)
			if err != nil {
				db.logger.Warn(fmt.Sprintf("Error checking if user %s already exists. Error: [%s]", standardUser.Email, err.Error()))
				return server_error.Wrap("DB_INIT_USERS", fmt.Sprintf("error checking if user [%s] already exists", standardUser.Email), err)
			}
			if existsResult.Next() {
				db.Logger().Debug(fmt.Sprintf("Skipping standard user. User %s already exists", standardUser.Email))
				continue
			}
			_ = existsResult.Close()
			userDto := standardUser.ToUserDTO(true)
			userId, err := userRepository.CreateUser(ctx, tx, userDto, standardUser.Password)
			if err != nil {
				return err
			}
			db.Logger().Debug(fmt.Sprintf("Created user %s with id %d", userDto.Email, userId))
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}
