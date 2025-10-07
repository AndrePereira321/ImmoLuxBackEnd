package models

import "time"

type UserDTO struct {
	ID        *RecordId  `json:"id"`
	FirstName *string    `json:"firstName"`
	LastName  *string    `json:"lastName"`
	Email     *string    `json:"email"`
	IsActive  *bool      `json:"isActive"`
	CreatedAt *time.Time `json:"createdAt"`
	UpdatedAt *time.Time `json:"updatedAt"`
}

type UserAuthDTO struct {
	ID                  *RecordId  `json:"id"`
	UserID              *RecordId  `json:"userId"`
	Hash                *string    `json:"-"`
	IsLocked            *bool      `json:"isLocked"`
	LockedReason        *string    `json:"lockedReason"`
	FailedLoginAttempts *int       `json:"failedLoginAttempts"`
	LastFailedLogin     *time.Time `json:"lastFailedLogin"`
	PasswordChangedAt   *time.Time `json:"passwordChangedAt"`
	CreatedAt           *time.Time `json:"createdAt"`
	UpdatedAt           *time.Time `json:"updatedAt"`
}
