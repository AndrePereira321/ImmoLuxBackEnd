package routes

import (
	"immo-lux/internal/utils"

	"github.com/gofiber/fiber/v3"
)

type LocationsResponse struct {
	Districts      []string `json:"districts"`
	Municipalities []string `json:"municipalities"`
	Parishes       []string `json:"parishes"`
}

// GetLocations returns all Portuguese administrative divisions in a single response
func GetLocations(ctx *RouteContext) error {
	validator, err := utils.GetLocationValidator()
	if err != nil {
		return ctx.RespondError(fiber.StatusInternalServerError, "LOCATION_VALIDATOR_ERROR", "Failed to load location data")
	}

	response := LocationsResponse{
		Districts:      validator.GetDistricts(),
		Municipalities: validator.GetMunicipalities(),
		Parishes:       validator.GetParishes(),
	}

	return ctx.RespondData(response)
}
