package utils

import (
	"encoding/json"
	"immo-lux/internal/models"
	"os"
)

type StandardUser struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
	Password  string `json:"password"`
}

func (u *StandardUser) ToUserDTO(isActive bool) *models.UserDTO {
	return &models.UserDTO{
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Email:     u.Email,
		IsActive:  isActive,
	}
}

func ReadStandardUsers(filePath string) []StandardUser {
	users := make([]StandardUser, 0)
	if filePath == "" {
		return users
	}
	if _, err := os.Stat(filePath); err != nil {
		return users
	}
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return users
	}
	err = json.Unmarshal(fileData, &users)
	return users
}
