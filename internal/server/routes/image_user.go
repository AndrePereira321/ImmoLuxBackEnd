package routes

import (
	"context"
	"fmt"
	"strconv"

	"immo-lux/internal/database/ent/client"
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
	propertyIdStr := ctx.Ctx().Params("id")
	propertyIdInt, err := strconv.Atoi(propertyIdStr)
	if err != nil {
		return ctx.BadRequest("Invalid property ID")
	}

	propertyId := models.RecordId(propertyIdInt)
	userId := ctx.GetUserId()
	if !userId.IsValid() {
		return ctx.RespondError(fiber.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated")
	}

	existingProperty, err := ctx.Db().NewPropertyRepository().GetPropertyById(context.Background(), propertyId)
	if err != nil {
		if server_error.IsServerError(err, "PROPERTY_NOT_FOUND") {
			return ctx.RespondError(fiber.StatusNotFound, "PROPERTY_NOT_FOUND", "Property not found")
		}
		return err
	}

	if *existingProperty.PublisherID != userId {
		return ctx.RespondError(fiber.StatusForbidden, "ACCESS_DENIED", "You can only upload images to your own properties")
	}

	fileHeader, err := ctx.Ctx().FormFile("image")
	if err != nil {
		return ctx.BadRequest("No image file provided")
	}

	if fileHeader.Size > MaxImageSize {
		return ctx.BadRequest(fmt.Sprintf("Image size must be less than %d MB", MaxImageSize/(1024*1024)))
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

	processedImage, err := utils.ProcessImage(imageData, contentType)
	if err != nil {
		return err
	}

	var imageId models.RecordId
	err = ctx.Db().WithTransaction(func(txCtx context.Context, tx *client.Tx) error {
		nextOrder, err := ctx.Db().NewPropertyImageRepository().GetNextDisplayOrder(txCtx, propertyId)
		if err != nil {
			return err
		}

		id, err := ctx.Db().NewPropertyImageRepository().CreateImage(
			txCtx,
			tx,
			propertyId,
			processedImage.Data,
			processedImage.ContentType,
			processedImage.Width,
			processedImage.Height,
			processedImage.FileSize,
			nextOrder,
		)
		if err != nil {
			return err
		}

		imageId = id
		return nil
	})

	if err != nil {
		ctx.Logger().Error(fmt.Sprintf("Failed to upload image for property %d: %s", propertyId, err.Error()))
		return err
	}

	ctx.Logger().Info(fmt.Sprintf("Image uploaded for property %d by user %d: image %d", propertyId, userId, imageId))

	return ctx.RespondData(&UploadImageResponse{
		ImageID: imageId,
		Message: "Image uploaded successfully",
	})
}

func DeletePropertyImage(ctx *RouteContext) error {
	imageIdStr := ctx.Ctx().Params("id")
	imageIdInt, err := strconv.Atoi(imageIdStr)
	if err != nil {
		return ctx.BadRequest("Invalid image ID")
	}

	imageId := models.RecordId(imageIdInt)
	userId := ctx.GetUserId()
	if !userId.IsValid() {
		return ctx.RespondError(fiber.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated")
	}

	images, err := ctx.Db().NewPropertyImageRepository().GetImagesByPropertyId(context.Background(), models.RecordId(imageIdInt))
	if err != nil {
		return err
	}

	if len(images) == 0 {
		return ctx.RespondError(fiber.StatusNotFound, "IMAGE_NOT_FOUND", "Image not found")
	}

	propertyId := *images[0].PropertyID
	existingProperty, err := ctx.Db().NewPropertyRepository().GetPropertyById(context.Background(), propertyId)
	if err != nil {
		return err
	}

	if *existingProperty.PublisherID != userId {
		return ctx.RespondError(fiber.StatusForbidden, "ACCESS_DENIED", "You can only delete images from your own properties")
	}

	err = ctx.Db().WithTransaction(func(txCtx context.Context, tx *client.Tx) error {
		return ctx.Db().NewPropertyImageRepository().DeleteImage(txCtx, tx, imageId)
	})

	if err != nil {
		ctx.Logger().Error(fmt.Sprintf("Failed to delete image %d: %s", imageId, err.Error()))
		return err
	}

	ctx.Logger().Info(fmt.Sprintf("Image deleted by user %d: %d", userId, imageId))

	return ctx.RespondData(&fiber.Map{
		"success": true,
		"message": "Image deleted successfully",
	})
}

func UpdatePropertyImageOrder(ctx *RouteContext) error {
	imageIdStr := ctx.Ctx().Params("id")
	imageIdInt, err := strconv.Atoi(imageIdStr)
	if err != nil {
		return ctx.BadRequest("Invalid image ID")
	}

	imageId := models.RecordId(imageIdInt)
	userId := ctx.GetUserId()
	if !userId.IsValid() {
		return ctx.RespondError(fiber.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated")
	}

	payload := &UpdateImageOrderPayload{}
	if err := ctx.ReadBody(&payload); err != nil {
		return err
	}

	images, err := ctx.Db().NewPropertyImageRepository().GetImagesByPropertyId(context.Background(), models.RecordId(imageIdInt))
	if err != nil {
		return err
	}

	if len(images) == 0 {
		return ctx.RespondError(fiber.StatusNotFound, "IMAGE_NOT_FOUND", "Image not found")
	}

	propertyId := *images[0].PropertyID
	existingProperty, err := ctx.Db().NewPropertyRepository().GetPropertyById(context.Background(), propertyId)
	if err != nil {
		return err
	}

	if *existingProperty.PublisherID != userId {
		return ctx.RespondError(fiber.StatusForbidden, "ACCESS_DENIED", "You can only modify images from your own properties")
	}

	err = ctx.Db().WithTransaction(func(txCtx context.Context, tx *client.Tx) error {
		return ctx.Db().NewPropertyImageRepository().UpdateImageOrder(txCtx, tx, imageId, payload.DisplayOrder)
	})

	if err != nil {
		ctx.Logger().Error(fmt.Sprintf("Failed to update image order %d: %s", imageId, err.Error()))
		return err
	}

	ctx.Logger().Info(fmt.Sprintf("Image order updated by user %d: %d", userId, imageId))

	return ctx.RespondData(&fiber.Map{
		"success": true,
		"message": "Image order updated successfully",
	})
}
