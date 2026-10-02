package database

import (
	"context"
	"database/sql"
	"fmt"
	"immo-lux/internal/config"
	"immo-lux/internal/database/ent/client"
	"immo-lux/internal/database/ent/client/user"
	"immo-lux/internal/logger"
	"immo-lux/internal/server_error"
	"immo-lux/internal/utils"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/lib/pq"
)

type Database struct {
	serverConfig *config.ServerConfig
	client       *client.Client
	sqlDB        *sql.DB
	driver       *entsql.Driver
	logger       *logger.Logger
	maintenance  *maintenanceJob
}

func New(serverConfig *config.ServerConfig) (*Database, error) {
	loggingConfig := serverConfig.Logging()
	dbLogger, err := logger.New("DATABASE", loggingConfig.DatabaseLogLevel(), loggingConfig.LogDir(), loggingConfig.LogToConsole())
	if err != nil {
		return nil, err
	}

	dbURL := serverConfig.Database().ConnectionString()
	dbLogger.Info("Initializing database.")

	sqlDB, err := sql.Open("postgres", dbURL)
	if err != nil {
		_ = dbLogger.Close()
		return nil, server_error.Wrap("DB_CONNECT", "failed to open database", err)
	}

	sqlDB.SetMaxIdleConns(serverConfig.Database().MaxIdleCons())
	sqlDB.SetMaxOpenConns(serverConfig.Database().MaxOpenCons())

	dbLogger.Debug(fmt.Sprintf("Connection pool configured: MaxIdle=%d, MaxOpen=%d",
		serverConfig.Database().MaxIdleCons(),
		serverConfig.Database().MaxOpenCons()))

	drv := entsql.OpenDB(dialect.Postgres, sqlDB)
	entClient := client.NewClient(client.Driver(drv))

	ctx := context.Background()
	if err := entClient.Schema.Create(ctx); err != nil {
		_ = entClient.Close()
		_ = drv.Close()
		_ = sqlDB.Close()
		_ = dbLogger.Close()
		return nil, server_error.Wrap("DB_MIGRATE", "failed to create schema", err)
	}

	dbLogger.Info("Database successfully initialized and migrated.")

	return &Database{
		serverConfig: serverConfig,
		client:       entClient,
		sqlDB:        sqlDB,
		driver:       drv,
		logger:       dbLogger,
	}, nil
}

func (db *Database) Logger() *logger.Logger {
	return db.logger
}

func (db *Database) Init() error {
	if err := db.initUsers(); err != nil {
		db.logger.Warn(fmt.Sprintf("Failed to initialize users. Error: %s", err.Error()))
		return err
	}
	db.startMaintenance()
	return nil
}

func (db *Database) Client() *client.Client {
	return db.client
}

func (db *Database) NewUserRepository() *UserRepository {
	return NewUserRepository(db)
}

func (db *Database) NewSessionRepository() *SessionRepository {
	return NewSessionRepository(db)
}

func (db *Database) NewPropertyRepository() *PropertyRepository {
	return NewPropertyRepository(db)
}

func (db *Database) NewPropertyImageRepository() *PropertyImageRepository {
	return NewPropertyImageRepository(db)
}

func (db *Database) NewContactRepository() *ContactRepository {
	return NewContactRepository(db)
}

func (db *Database) NewAuthLogRepository() *AuthLogRepository {
	return NewAuthLogRepository(db)
}

func (db *Database) NewRateLimitRepository() *RateLimitRepository {
	return NewRateLimitRepository(db)
}

func (db *Database) WithTransaction(ctx context.Context, fn func(ctx context.Context, tx *client.Tx) error) error {
	tx, err := db.client.Tx(ctx)
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
		return server_error.Wrap("DB_TX", "error commiting transaction", err)
	}
	return nil
}

func (db *Database) Close() error {
	db.stopMaintenance()

	if err := db.client.Close(); err != nil {
		db.logger.Warn(fmt.Sprintf("Error closing Ent client: %s", err.Error()))
	}

	if err := db.driver.Close(); err != nil {
		db.logger.Warn(fmt.Sprintf("Error closing driver: %s", err.Error()))
	}

	if err := db.sqlDB.Close(); err != nil {
		db.logger.Warn(fmt.Sprintf("Error closing SQL database: %s", err.Error()))
		_ = db.logger.Close()
		return server_error.Wrap("DB_CLOSE", "error when closing database", err)
	}

	return db.logger.Close()
}

func (db *Database) Ping() bool {
	ctx := context.Background()
	_, err := db.client.User.Query().Limit(1).Count(ctx)
	return err == nil
}

func (db *Database) initUsers() error {
	standardUsersFilePath := db.serverConfig.Database().UserFilePath()
	standardUsers := utils.ReadStandardUsers(standardUsersFilePath)
	if len(standardUsers) == 0 {
		return nil
	}

	userRepository := db.NewUserRepository()
	return db.WithTransaction(context.Background(), func(ctx context.Context, tx *client.Tx) error {
		for _, standardUser := range standardUsers {
			exists, err := tx.User.Query().
				Where(user.EmailEQ(standardUser.Email)).
				Exist(ctx)
			if err != nil {
				db.logger.Warn(fmt.Sprintf("Error checking if user %s already exists. Error: [%s]", standardUser.Email, err.Error()))
				return server_error.Wrap("DB_INIT_USERS", fmt.Sprintf("error checking if user [%s] already exists", standardUser.Email), err)
			}
			if exists {
				db.logger.Debug(fmt.Sprintf("Skipping standard user. User %s already exists", standardUser.Email))
				continue
			}

			userDto := standardUser.ToUserDTO(true)
			userId, err := userRepository.createUser(ctx, tx, userDto, standardUser.Password)
			if err != nil {
				return err
			}
			db.logger.Debug(fmt.Sprintf("Created user %s with id %d", *userDto.Email, userId))
		}
		return nil
	})
}
