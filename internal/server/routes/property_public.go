package routes

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"immo-lux/internal/database"
	"immo-lux/internal/models"
	"immo-lux/internal/server_error"

	"github.com/gofiber/fiber/v3"
)

type PropertyListResponse struct {
	Properties []models.PropertyDTO `json:"properties"`
	Total      int                  `json:"total"`
}

func GetProperty(ctx *RouteContext) error {
	propertyId, err := ctx.ParseIdParam("id")
	if err != nil {
		return err
	}

	property, err := ctx.Db().NewPropertyRepository().GetPropertyById(context.Background(), propertyId)
	if err != nil {
		if server_error.IsServerError(err, "PROPERTY_NOT_FOUND") {
			return ctx.RespondError(fiber.StatusNotFound, "PROPERTY_NOT_FOUND", "Property not found")
		}
		return err
	}

	err = ctx.Db().NewPropertyRepository().IncrementViewCount(context.Background(), propertyId)
	if err != nil {
		ctx.Logger().Warn(fmt.Sprintf("Failed to increment view count for property %d: %s", propertyId, err.Error()))
	}

	return ctx.RespondData(property)
}

// publicPropertyFilters reads the search query string. The listing and the facet
// counts must agree on what is being asked, so both read it through here.
// IsPublished is pinned on: an unpublished property is not public inventory,
// whatever the caller passes.
func publicPropertyFilters(ctx *RouteContext) database.PropertyFilters {
	filters := database.PropertyFilters{}

	if district := ctx.Ctx().Query("district"); district != "" {
		filters.District = &district
	}
	if municipality := ctx.Ctx().Query("municipality"); municipality != "" {
		filters.Municipality = &municipality
	}
	if parish := ctx.Ctx().Query("parish"); parish != "" {
		filters.Parish = &parish
	}
	if propertyType := ctx.Ctx().Query("propertyType"); propertyType != "" {
		filters.PropertyType = &propertyType
	}
	if status := ctx.Ctx().Query("status"); status != "" {
		filters.Status = &status
	}
	if minPriceStr := ctx.Ctx().Query("minPrice"); minPriceStr != "" {
		if minPrice, err := strconv.ParseFloat(minPriceStr, 64); err == nil {
			filters.MinPrice = &minPrice
		}
	}
	if maxPriceStr := ctx.Ctx().Query("maxPrice"); maxPriceStr != "" {
		if maxPrice, err := strconv.ParseFloat(maxPriceStr, 64); err == nil {
			filters.MaxPrice = &maxPrice
		}
	}
	if search := strings.TrimSpace(ctx.Ctx().Query("q")); search != "" {
		filters.Query = &search
	}

	isPublished := true
	filters.IsPublished = &isPublished

	return filters
}

func ListProperties(ctx *RouteContext) error {
	limit := 20
	if limitStr := ctx.Ctx().Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}
	offset := 0
	if offsetStr := ctx.Ctx().Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil {
			offset = o
		}
	}

	filters := publicPropertyFilters(ctx)
	filters.Limit = limit
	filters.Offset = offset

	if orderBy := ctx.Ctx().Query("orderBy"); orderBy != "" {
		filters.OrderBy = &orderBy
	}

	properties, total, err := ctx.Db().NewPropertyRepository().ListProperties(context.Background(), filters)
	if err != nil {
		ctx.Logger().Error(fmt.Sprintf("Failed to list properties: %s", err.Error()))
		return err
	}

	return ctx.RespondData(&PropertyListResponse{
		Properties: properties,
		Total:      total,
	})
}

// ListPropertyFacets reports how many properties sit behind each remaining choice,
// so the search can show its options with counts and withhold the ones that lead
// nowhere.
func ListPropertyFacets(ctx *RouteContext) error {
	facets, err := ctx.Db().NewPropertyRepository().GetPropertyFacets(context.Background(), publicPropertyFilters(ctx))
	if err != nil {
		ctx.Logger().Error(fmt.Sprintf("Failed to compute property facets: %s", err.Error()))
		return err
	}

	return ctx.RespondData(facets)
}
