package routes

import (
	"context"
	"fmt"
	"strconv"

	"immo-lux/internal/models"
	"immo-lux/internal/server_error"

	"github.com/gofiber/fiber/v3"
)

func GetPropertyImages(ctx *RouteContext) error {
	propertyIdStr := ctx.Ctx().Params("id")
	propertyIdInt, err := strconv.Atoi(propertyIdStr)
	if err != nil {
		return ctx.BadRequest("Invalid property ID")
	}

	propertyId := models.RecordId(propertyIdInt)

	images, err := ctx.Db().NewPropertyImageRepository().GetImagesByPropertyId(context.Background(), propertyId)
	if err != nil {
		ctx.Logger().Error(fmt.Sprintf("Failed to get images for property %d: %s", propertyId, err.Error()))
		return err
	}

	return ctx.RespondData(&fiber.Map{
		"images": images,
	})
}

func GetImage(ctx *RouteContext) error {
	imageIdStr := ctx.Ctx().Params("id")
	imageIdInt, err := strconv.Atoi(imageIdStr)
	if err != nil {
		return ctx.BadRequest("Invalid image ID")
	}

	imageId := models.RecordId(imageIdInt)

	imageData, contentType, err := ctx.Db().NewPropertyImageRepository().GetImageById(context.Background(), imageId)
	if err != nil {
		if server_error.IsServerError(err, "IMAGE_NOT_FOUND") {
			return ctx.RespondError(fiber.StatusNotFound, "IMAGE_NOT_FOUND", "Image not found")
		}
		return err
	}

	ctx.Ctx().Set("Content-Type", contentType)
	ctx.Ctx().Set("Cache-Control", "public, max-age=31536000")
	return ctx.Ctx().Send(imageData)
}
