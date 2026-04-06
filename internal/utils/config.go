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
		FirstName: &u.FirstName,
		LastName:  &u.LastName,
		Email:     &u.Email,
		IsActive:  &isActive,
	}
}

func ReadStandardUsers(filePath string) []StandardUser {
	if filePath == "" {
		return nil
	}
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}
	var users []StandardUser
	_ = json.Unmarshal(fileData, &users)
	return users
}
