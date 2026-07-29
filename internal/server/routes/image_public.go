package routes

import (
	"github.com/gofiber/fiber/v3"
)

func GetPropertyImages(ctx *RouteContext) error {
	propertyId, err := ctx.ParseIdParam("id")
	if err != nil {
		return err
	}

	images, err := ctx.Db().NewPropertyImageRepository().ListImageMetadata(ctx.RequestContext(), propertyId)
	if err != nil {
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

	imageData, contentType, err := ctx.Db().NewPropertyImageRepository().ImagePayload(ctx.RequestContext(), imageId)
	if err != nil {
		return err
	}

	ctx.Ctx().Set("Content-Type", contentType)
	ctx.Ctx().Set("Cache-Control", "public, max-age=31536000")
	return ctx.Ctx().Send(imageData)
}
