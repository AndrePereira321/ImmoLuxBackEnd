package routes

import (
	"context"

	"immo-lux/internal/utils"

	"github.com/gofiber/fiber/v3"
)

type LocationsResponse struct {
	Districts      []string `json:"districts"`
	Municipalities []string `json:"municipalities"`
	Parishes       []string `json:"parishes"`
}

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

func GetLocationStats(ctx *RouteContext) error {
	districtSlug := ctx.Ctx().Query("district")
	if districtSlug == "" {
		return ctx.RespondError(fiber.StatusBadRequest, "MISSING_DISTRICT", "district parameter is required")
	}

	validator, err := utils.GetLocationValidator()
	if err != nil {
		return ctx.RespondError(fiber.StatusInternalServerError, "LOCATION_VALIDATOR_ERROR", "Failed to load location data")
	}

	district, ok := validator.SlugToDistrict(districtSlug)
	if !ok {
		return ctx.RespondError(fiber.StatusNotFound, "DISTRICT_NOT_FOUND", "district not found")
	}

	var municipality *string
	if municipalitySlug := ctx.Ctx().Query("municipality"); municipalitySlug != "" {
		name, ok := validator.SlugToMunicipality(municipalitySlug)
		if !ok {
			return ctx.RespondError(fiber.StatusNotFound, "MUNICIPALITY_NOT_FOUND", "municipality not found")
		}
		municipality = &name
	}

	stats, err := ctx.Db().NewPropertyRepository().GetLocationStats(context.Background(), district, municipality)
	if err != nil {
		return err
	}

	return ctx.RespondData(stats)
}

func GetPublishedLocations(ctx *RouteContext) error {
	locations, err := ctx.Db().NewPropertyRepository().GetPublishedLocations(context.Background())
	if err != nil {
		return err
	}
	return ctx.RespondData(locations)
}
