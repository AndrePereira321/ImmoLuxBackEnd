package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"immo-lux/internal/database"
	"immo-lux/internal/models"
	"immo-lux/internal/server_error"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type UserRepository struct {
	db *database.Database
}

func NewUserRepository(db *database.Database) *UserRepository {
	return &UserRepository{db: db}
}

func (rep *UserRepository) CreateUser(user *models.UserDTO, password string) (models.RecordId, error) {
	if err := rep.validateUserInput(user, password); err != nil {
		return 0, err
	}

	hashedPassword, err := rep.hashPassword(password)
	if err != nil {
		return 0, err
	}

	now := time.Now()
	var userId models.RecordId
	err = rep.db.WithTransaction(func(ctx context.Context, tx *sql.Tx) error {

		userId, err = rep.insertUser(ctx, tx, user, now)
		if err != nil {
			rep.db.Logger().Warn(fmt.Sprintf("Error inserting user: %s", err.Error()))
			return err
		}

		if err = rep.insertUserAuth(ctx, tx, userId, hashedPassword, now); err != nil {
			rep.db.Logger().Warn(fmt.Sprintf("Error inserting user authentication: %s", err.Error()))
			return err
		}

		return nil
	})
	if err != nil {
		return 0, err
	}

	user.ID = userId
	user.CreatedAt = now
	user.UpdatedAt = now

	return userId, nil
}

func (rep *UserRepository) UserExists(email string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM user WHERE email = ?)`
	rows, err := rep.db.Query(query, email)
	if err != nil {
		return false, server_error.Wrap("USER_REPOSITORY", "failed executing user exists query", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return false, server_error.New("USER_REPOSITORY", "failed to check if user exists")
	}
	var exists bool
	err = rows.Scan(&exists)
	if err != nil {
		return false, server_error.Wrap("USER_REPOSITORY", "failed scanning db user exists", err)
	}
	return exists, nil

}

func (rep *UserRepository) insertUser(ctx context.Context, tx *sql.Tx, user *models.UserDTO, now time.Time) (models.RecordId, error) {
	query := `INSERT INTO user (first_name, last_name, email, is_active, created_at, updated_at) 
	          VALUES (?, ?, ?, ?, ?, ?)`

	result, err := tx.ExecContext(ctx, query,
		user.FirstName,
		user.LastName,
		user.Email,
		user.IsActive,
		now,
		now,
	)
	if err != nil {
		if isUniqueConstraintError(err) {
			return 0, server_error.Wrap("USER_ALREADY_EXISTS", "user with this email already exists", err)
		}
		return 0, server_error.Wrap("USER_INSERT", "failed to insert user", err)
	}

	userId, err := result.LastInsertId()
	if err != nil {
		return 0, server_error.Wrap("USER_INSERT", "failed to get user ID", err)
	}

	return models.RecordId(userId), nil
}

func (rep *UserRepository) insertUserAuth(ctx context.Context, tx *sql.Tx, userId models.RecordId, hashedPassword string, now time.Time) error {
	query := `INSERT INTO user_auth (user_id, hash, is_locked, failed_login_attempts, password_changed_at, created_at, updated_at)
	          VALUES (?, ?, 0, 0, ?, ?, ?)`

	_, err := tx.ExecContext(ctx, query,
		userId,
		hashedPassword,
		now,
		now,
		now,
	)
	if err != nil {
		return server_error.Wrap("USER_AUTH_INSERT", "failed to insert user authentication", err)
	}

	return nil
}

func (rep *UserRepository) validateUserInput(user *models.UserDTO, password string) error {
	if user.FirstName == "" {
		return server_error.New("USER_VALIDATION", "first name is required")
	}
	if user.LastName == "" {
		return server_error.New("USER_VALIDATION", "last name is required")
	}
	if user.Email == "" {
		return server_error.New("USER_VALIDATION", "email is required")
	}
	if password == "" {
		return server_error.New("USER_VALIDATION", "password is required")
	}
	return nil
}

func (rep *UserRepository) hashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		rep.db.Logger().Warn(fmt.Sprintf("Error hashing password: %s", err.Error()))
		return "", server_error.Wrap("USER_REPOSITORY", "error hashing password", err)
	}
	return string(hashedPassword), nil
}

func isUniqueConstraintError(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "UNIQUE constraint failed") ||
		strings.Contains(err.Error(), "unique constraint"))
}
