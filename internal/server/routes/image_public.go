package routes

import (
	"context"
	"fmt"

	"immo-lux/internal/server_error"

	"github.com/gofiber/fiber/v3"
)

func GetPropertyImages(ctx *RouteContext) error {
	propertyId, err := ctx.ParseIdParam("id")
	if err != nil {
		return err
	}

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
	imageId, err := ctx.ParseIdParam("id")
	if err != nil {
		return err
	}

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
