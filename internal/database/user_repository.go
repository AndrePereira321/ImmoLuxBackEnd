package database

import (
	"context"
	"fmt"
	"immo-lux/internal/database/ent/client"
	"immo-lux/internal/database/ent/client/user"
	"immo-lux/internal/models"
	"immo-lux/internal/server_error"
	"immo-lux/internal/utils"

	"golang.org/x/crypto/bcrypt"
)

type UserRepository struct {
	db *Database
}

func NewUserRepository(db *Database) *UserRepository {
	return &UserRepository{db: db}
}

func (rep *UserRepository) CreateUser(ctx context.Context, tx *client.Tx, user *models.UserDTO, password string) (models.RecordId, error) {
	if err := rep.validateUserInput(user, password); err != nil {
		return models.InvalidRecordId, err
	}

	hashedPassword, err := rep.hashPassword(password)
	if err != nil {
		return models.InvalidRecordId, err
	}

	rep.db.Logger().Debug(fmt.Sprintf("Creating user: %s", *user.Email))

	createdUser, err := tx.User.Create().
		SetFirstName(*user.FirstName).
		SetLastName(*user.LastName).
		SetEmail(*user.Email).
		SetIsActive(*user.IsActive).
		Save(ctx)
	if err != nil {
		rep.db.Logger().Error(fmt.Sprintf("Failed to create user [%s]: %s", *user.Email, err.Error()))
		if client.IsConstraintError(err) {
			return models.InvalidRecordId, server_error.Wrap("USER_ALREADY_EXISTS",
				fmt.Sprintf("user with email [%s] already exists", *user.Email), err)
		}
		return models.InvalidRecordId, server_error.Wrap("USER_INSERT",
			fmt.Sprintf("failed to insert user [%s]", *user.Email), err)
	}

	userId := models.RecordId(createdUser.ID)
	rep.db.Logger().Debug(fmt.Sprintf("User created successfully: %s (ID: %d)", *user.Email, userId))

	_, err = tx.UserAuth.Create().
		SetUserID(createdUser.ID).
		SetHash(hashedPassword).
		SetIsLocked(false).
		SetFailedLoginAttempts(0).
		Save(ctx)
	if err != nil {
		rep.db.Logger().Error(fmt.Sprintf("Failed to create auth for user [%s]: %s", *user.Email, err.Error()))
		return models.InvalidRecordId, server_error.Wrap("USER_AUTH_INSERT",
			fmt.Sprintf("failed to insert authentication for user [%s]", *user.Email), err)
	}

	rep.db.Logger().Debug(fmt.Sprintf("User authentication created successfully for: %s", *user.Email))

	user.ID = &userId
	now := createdUser.CreatedAt
	user.CreatedAt = &now
	user.UpdatedAt = &createdUser.UpdatedAt

	return userId, nil
}

func (rep *UserRepository) GetUserID(email string) (models.RecordId, error) {
	ctx := context.Background()
	u, err := rep.db.client.User.Query().
		Where(user.EmailEQ(email)).
		Only(ctx)
	if err != nil {
		if client.IsNotFound(err) {
			return models.InvalidRecordId, server_error.New("USER_NOT_FOUND", fmt.Sprintf("user not found: %s", email))
		}
		return models.InvalidRecordId, server_error.Wrap("USER_REPOSITORY", "failed querying user", err)
	}
	return models.RecordId(u.ID), nil
}

func (rep *UserRepository) UserExists(email string) (bool, error) {
	ctx := context.Background()
	exists, err := rep.db.client.User.Query().
		Where(user.EmailEQ(email)).
		Exist(ctx)
	if err != nil {
		return false, server_error.Wrap("USER_REPOSITORY", "failed checking if user exists", err)
	}
	return exists, nil
}

func (rep *UserRepository) GetUserAuth(email string) (*models.UserDTO, *models.UserAuthDTO, error) {
	ctx := context.Background()

	u, err := rep.db.client.User.Query().
		Where(user.EmailEQ(email)).
		WithAuth().
		Only(ctx)
	if err != nil {
		if client.IsNotFound(err) {
			return nil, nil, server_error.New("USER_NOT_FOUND", fmt.Sprintf("user not found: %s", email))
		}
		return nil, nil, server_error.Wrap("USER_REPOSITORY", "failed querying user with auth", err)
	}

	auth, err := u.QueryAuth().Only(ctx)
	if err != nil {
		if client.IsNotFound(err) {
			return nil, nil, server_error.New("USER_AUTH_NOT_FOUND", fmt.Sprintf("user auth not found for user: %s", email))
		}
		return nil, nil, server_error.Wrap("USER_REPOSITORY", "failed querying user auth", err)
	}

	userId := models.RecordId(u.ID)
	userAuthId := models.RecordId(auth.ID)
	userAuthUserId := models.RecordId(auth.UserID)

	userDTO := &models.UserDTO{
		ID:        &userId,
		FirstName: &u.FirstName,
		LastName:  &u.LastName,
		Email:     &u.Email,
		IsActive:  &u.IsActive,
		CreatedAt: &u.CreatedAt,
		UpdatedAt: &u.UpdatedAt,
	}

	userAuthDTO := &models.UserAuthDTO{
		ID:                  &userAuthId,
		UserID:              &userAuthUserId,
		Hash:                &auth.Hash,
		IsLocked:            &auth.IsLocked,
		LockedReason:        auth.LockedReason,
		FailedLoginAttempts: &auth.FailedLoginAttempts,
		LastFailedLogin:     auth.LastFailedAttempt,
		PasswordChangedAt:   auth.PasswordChangedAt,
		CreatedAt:           &auth.CreatedAt,
		UpdatedAt:           &auth.UpdatedAt,
	}

	return userDTO, userAuthDTO, nil
}

func (rep *UserRepository) validateUserInput(user *models.UserDTO, password string) error {
	if *user.FirstName == "" {
		return server_error.New("USER_VALIDATION", "first name is required")
	}
	if *user.LastName == "" {
		return server_error.New("USER_VALIDATION", "last name is required")
	}
	if *user.Email == "" {
		return server_error.New("USER_VALIDATION", "email is required")
	}
	if !utils.IsValidEmail(*user.Email) {
		return server_error.New("USER_VALIDATION", "invalid email")
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
