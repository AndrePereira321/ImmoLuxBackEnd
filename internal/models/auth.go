package models

import "time"

type UserDTO struct {
	ID        RecordId
	FirstName string
	LastName  string
	Email     string
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}
