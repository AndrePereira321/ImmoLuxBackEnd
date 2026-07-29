package routes

import (
	"immo-lux/internal/server_error"
	"immo-lux/internal/utils"
)

type LocationsResponse struct {
	Districts      []string `json:"districts"`
	Municipalities []string `json:"municipalities"`
	Parishes       []string `json:"parishes"`
}

func GetLocations(ctx *RouteContext) error {
	validator, err := utils.GetLocationValidator()
	if err != nil {
		return server_error.Wrap("LOCATION_VALIDATOR_ERROR", "Failed to load location data", err)
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
		return server_error.Invalid("MISSING_DISTRICT", "district parameter is required")
	}

	validator, err := utils.GetLocationValidator()
	if err != nil {
		return server_error.Wrap("LOCATION_VALIDATOR_ERROR", "Failed to load location data", err)
	}

	district, ok := validator.SlugToDistrict(districtSlug)
	if !ok {
		return server_error.NotFound("DISTRICT_NOT_FOUND", "district not found")
	}

	var municipality *string
	if municipalitySlug := ctx.Ctx().Query("municipality"); municipalitySlug != "" {
		name, ok := validator.SlugToMunicipality(municipalitySlug)
		if !ok {
			return server_error.NotFound("MUNICIPALITY_NOT_FOUND", "municipality not found")
		}
		municipality = &name
	}

	stats, err := ctx.Db().NewPropertyRepository().GetLocationStats(ctx.RequestContext(), district, municipality)
	if err != nil {
		return err
	}

	return ctx.RespondData(stats)
}

func GetPublishedLocations(ctx *RouteContext) error {
	locations, err := ctx.Db().NewPropertyRepository().GetPublishedLocations(ctx.RequestContext())
	if err != nil {
		return err
	}
	return ctx.RespondData(locations)
}
