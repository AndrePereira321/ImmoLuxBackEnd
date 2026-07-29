package routes

import (
	"fmt"
	"strconv"

	"immo-lux/internal/models"
	"immo-lux/internal/server_error"
	"immo-lux/internal/utils"

	"github.com/gofiber/fiber/v3"
)

const MaxImageSize = 10 * 1024 * 1024

type UploadImageResponse struct {
	ImageID models.RecordId `json:"imageId"`
	Message string          `json:"message"`
}

type UpdateImageOrderPayload struct {
	DisplayOrder int `json:"displayOrder"`
}

func UploadPropertyImage(ctx *RouteContext) error {
	propertyId, err := ctx.ParseIdParam("id")
	if err != nil {
		return err
	}

	userId, err := ctx.RequireUserId()
	if err != nil {
		return err
	}

	fileHeader, err := ctx.Ctx().FormFile("image")
	if err != nil {
		return server_error.BadRequest("No image file provided")
	}
	if fileHeader.Size > MaxImageSize {
		return server_error.BadRequest(fmt.Sprintf("Image size must be less than %d MB", MaxImageSize/(1024*1024)))
	}

	file, err := fileHeader.Open()
	if err != nil {
		return server_error.Wrap("IMAGE_READ", "failed to open uploaded file", err)
	}
	defer file.Close()

	imageData, contentType, err := utils.ReadImageFromReader(file, MaxImageSize)
	if err != nil {
		return err
	}

	var displayOrder *int
	if s := ctx.Ctx().FormValue("displayOrder"); s != "" {
		if n, parseErr := strconv.Atoi(s); parseErr == nil {
			displayOrder = &n
		}
	}

	imageId, err := ctx.Db().NewPropertyImageRepository().UploadImage(
		ctx.RequestContext(), userId, propertyId, imageData, contentType, displayOrder)
	if err != nil {
		return err
	}

	return ctx.RespondData(&UploadImageResponse{
		ImageID: imageId,
		Message: "Image uploaded successfully",
	})
}

func DeletePropertyImage(ctx *RouteContext) error {
	imageId, err := ctx.ParseIdParam("id")
	if err != nil {
		return err
	}

	userId, err := ctx.RequireUserId()
	if err != nil {
		return err
	}

	if err := ctx.Db().NewPropertyImageRepository().DeleteImage(ctx.RequestContext(), userId, imageId); err != nil {
		return err
	}

	return ctx.RespondData(&fiber.Map{
		"success": true,
		"message": "Image deleted successfully",
	})
}

func UpdatePropertyImageOrder(ctx *RouteContext) error {
	imageId, err := ctx.ParseIdParam("id")
	if err != nil {
		return err
	}

	userId, err := ctx.RequireUserId()
	if err != nil {
		return err
	}

	payload := &UpdateImageOrderPayload{}
	if err := ctx.ReadBody(&payload); err != nil {
		return err
	}

	if err := ctx.Db().NewPropertyImageRepository().UpdateImageOrder(
		ctx.RequestContext(), userId, imageId, payload.DisplayOrder); err != nil {
		return err
	}

	return ctx.RespondData(&fiber.Map{
		"success": true,
		"message": "Image order updated successfully",
	})
}
