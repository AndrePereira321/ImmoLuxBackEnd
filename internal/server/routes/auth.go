package routes

import (
	"fmt"
	"time"

	"immo-lux/internal/config"
	"immo-lux/internal/database"
	"immo-lux/internal/models"
	"immo-lux/internal/utils"

	"github.com/gofiber/fiber/v3"
)

type LoginPayload struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	RememberMe bool   `json:"rememberMe"`
}

type AuthStatusResponse struct {
	IsConnected bool            `json:"isConnected"`
	UserData    *models.UserDTO `json:"userData"`
}

type LogoutResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func Login(ctx *RouteContext) error {
	payload := &LoginPayload{}
	if err := ctx.ReadBody(&payload); err != nil {
		return err
	}

	result, err := ctx.Db().NewAuthOperations().Login(ctx.RequestContext(), database.LoginInput{
		Email:      payload.Email,
		Password:   payload.Password,
		RememberMe: payload.RememberMe,
		IpAddress:  ctx.ClientIP(),
		UserAgent:  ctx.Ctx().Get("User-Agent"),
	})
	if err != nil {
		return err
	}

	ctx.Ctx().Cookie(createSessionCookie(result.Token, payload.RememberMe, ctx.ServerConfig()))

	return ctx.RespondData(&AuthStatusResponse{
		IsConnected: true,
		UserData:    result.User,
	})
}

func IsConnected(ctx *RouteContext) error {
	if !ctx.IsAuthenticated() {
		return ctx.RespondData(&AuthStatusResponse{
			IsConnected: false,
			UserData:    nil,
		})
	}

	userId := ctx.GetUserId()
	userDto, err := ctx.Db().NewUserRepository().GetUserById(ctx.RequestContext(), userId)
	if err != nil {
		ctx.Logger().Warn(fmt.Sprintf("Failed to get user %d: %s", userId, err.Error()))
		return ctx.RespondData(&AuthStatusResponse{
			IsConnected: false,
			UserData:    nil,
		})
	}

	return ctx.RespondData(&AuthStatusResponse{
		IsConnected: true,
		UserData:    userDto,
	})
}

func Logout(ctx *RouteContext) error {
	// Logout is best-effort: the cookie is cleared and success reported even if
	// the session row could not be touched.
	_ = ctx.Db().NewAuthOperations().Logout(
		ctx.RequestContext(), ctx.GetUserId(), ctx.GetSessionId(), ctx.ClientIP(), ctx.Ctx().Get("User-Agent"))

	clearSessionCookie(ctx.Ctx(), ctx.ServerConfig())

	return ctx.RespondData(&LogoutResponse{
		Success: true,
		Message: "Logged out successfully",
	})
}

func createSessionCookie(token string, rememberMe bool, cfg *config.ServerConfig) *fiber.Cookie {
	return &fiber.Cookie{
		Name:     config.SessionCookieName,
		Value:    token,
		Expires:  utils.CalculateExpiration(rememberMe),
		HTTPOnly: true,
		Secure:   cfg.Security().CookieSecure(),
		SameSite: fiber.CookieSameSiteStrictMode,
		Path:     "/",
	}
}

func clearSessionCookie(ctx fiber.Ctx, cfg *config.ServerConfig) {
	ctx.Cookie(&fiber.Cookie{
		Name:     config.SessionCookieName,
		Value:    "",
		Expires:  time.Unix(0, 0),
		HTTPOnly: true,
		Secure:   cfg.Security().CookieSecure(),
		SameSite: fiber.CookieSameSiteStrictMode,
		Path:     "/",
	})
}
