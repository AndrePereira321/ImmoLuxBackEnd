package models

import "time"

type UserDTO struct {
	ID          *RecordId  `json:"id"`
	FirstName   *string    `json:"firstName"`
	LastName    *string    `json:"lastName"`
	Email       *string    `json:"email"`
	IsActive    *bool      `json:"isActive"`
	IsSuperUser *bool      `json:"isSuperUser"`
	CreatedAt   *time.Time `json:"createdAt"`
	UpdatedAt   *time.Time `json:"updatedAt"`
}

type UserAuthDTO struct {
	ID                *RecordId  `json:"id"`
	UserID            *RecordId  `json:"userId"`
	Hash              *string    `json:"-"`
	PasswordChangedAt *time.Time `json:"passwordChangedAt"`
	CreatedAt         *time.Time `json:"createdAt"`
	UpdatedAt         *time.Time `json:"updatedAt"`
}

type SessionDTO struct {
	ID                *RecordId  `json:"id"`
	UserID            *RecordId  `json:"userId"`
	SessionToken      *string    `json:"-"` // Sensitive, don't send to client
	IsActive          *bool      `json:"isActive"`
	RememberMe        *bool      `json:"rememberMe"`
	ExpiresAt         *time.Time `json:"expiresAt"`
	InvalidatedAt     *time.Time `json:"invalidatedAt"`
	InvalidatedReason *string    `json:"invalidatedReason"`
	IpAddress         *string    `json:"ipAddress"`
	UserAgent         *string    `json:"userAgent"`
	CreatedAt         *time.Time `json:"createdAt"`
	UpdatedAt         *time.Time `json:"updatedAt"`
}
