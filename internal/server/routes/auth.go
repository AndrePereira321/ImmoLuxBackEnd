package routes

import (
	"fmt"
	"immo-lux/internal/models"
	"immo-lux/internal/server_error"
	"immo-lux/internal/utils"
)

type LoginPayload struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	IsConnected  bool            `json:"isConnected"`
	SessionToken string          `json:"sessionToken"`
	UserData     *models.UserDTO `json:"userData"`
}

func Login(ctx *RouteContext) error {
	payload := &LoginPayload{}
	err := ctx.ReadBody(&payload)
	if err != nil {
		return err
	}

	if !utils.IsValidEmail(payload.Email) {
		return ctx.BadRequest("Invalid email")
	}

	if len(payload.Password) < models.PasswordMinLength {
		return ctx.BadRequest(fmt.Sprintf("Password must be at least %d characters long", models.PasswordMinLength))
	}

	userDto, userAuthDto, err := ctx.Db().NewUserRepository().GetUserAuth(payload.Email)
	if err != nil {
		if server_error.IsServerError(err, "USER_NOT_FOUND") {
			return ctx.BadRequest("User not found")
		}
		ctx.Logger().Warn(fmt.Sprintf("Error getting user data: %s", err.Error()))
		return err
	}

	//TODO Validate user
	print(userAuthDto)

	return ctx.RespondData(&LoginResponse{
		IsConnected:  true,
		SessionToken: "",
		UserData:     userDto,
	})
}
