package routes

import (
	"context"
	"fmt"
	"strconv"

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
	propertyIdStr := ctx.Ctx().Params("id")
	propertyIdInt, err := strconv.Atoi(propertyIdStr)
	if err != nil {
		return ctx.BadRequest("Invalid property ID")
	}

	propertyId := models.RecordId(propertyIdInt)

	property, err := ctx.Db().NewPropertyRepository().GetPropertyById(context.Background(), propertyId)
	if err != nil {
		if server_error.IsServerError(err, "PROPERTY_NOT_FOUND") {
			return ctx.RespondError(fiber.StatusNotFound, "PROPERTY_NOT_FOUND", "Property not found")
		}
		return err
	}

	if property.ContactID != nil {
		contact, err := ctx.Db().NewContactRepository().GetContactById(context.Background(), *property.ContactID)
		if err == nil {
			property.Contact = contact
		}
	}

	err = ctx.Db().NewPropertyRepository().IncrementViewCount(context.Background(), propertyId)
	if err != nil {
		ctx.Logger().Warn(fmt.Sprintf("Failed to increment view count for property %d: %s", propertyId, err.Error()))
	}

	return ctx.RespondData(property)
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

	filters := database.PropertyFilters{
		Limit:  limit,
		Offset: offset,
	}

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

	isPublished := true
	filters.IsPublished = &isPublished

	properties, total, err := ctx.Db().NewPropertyRepository().ListProperties(context.Background(), filters)
	if err != nil {
		ctx.Logger().Error(fmt.Sprintf("Failed to list properties: %s", err.Error()))
		return err
	}

	for i := range properties {
		if properties[i].ContactID != nil {
			contact, err := ctx.Db().NewContactRepository().GetContactById(context.Background(), *properties[i].ContactID)
			if err == nil {
				properties[i].Contact = contact
			}
		}
	}

	return ctx.RespondData(&PropertyListResponse{
		Properties: properties,
		Total:      total,
	})
}
