package routes

import (
	"immo-lux/internal/utils"
)

type LoginPayload struct {
	Email    string `json:"email"`
	Password string `json:"password"`
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

	return nil
}
