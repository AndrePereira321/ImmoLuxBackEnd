package database

import (
	"context"
	"fmt"

	"immo-lux/internal/database/ent/client"
	"immo-lux/internal/database/ent/client/propertyimage"
	"immo-lux/internal/models"
	"immo-lux/internal/server_error"
)

type PropertyImageRepository struct {
	db *Database
}

func NewPropertyImageRepository(db *Database) *PropertyImageRepository {
	return &PropertyImageRepository{db: db}
}

func (rep *PropertyImageRepository) CreateImage(ctx context.Context, tx *client.Tx, propertyId models.RecordId, imageData []byte, contentType string, width, height, fileSize, displayOrder int) (models.RecordId, error) {
	if !propertyId.IsValid() {
		return models.InvalidRecordId, server_error.New("INVALID_PROPERTY_ID", "property ID is invalid")
	}

	if len(imageData) == 0 {
		return models.InvalidRecordId, server_error.New("IMAGE_VALIDATION", "image data is empty")
	}

	rep.db.Logger().Debug(fmt.Sprintf("Creating image for property: %d", propertyId))

	createdImage, err := tx.PropertyImage.Create().
		SetPropertyID(int(propertyId)).
		SetImageData(imageData).
		SetContentType(contentType).
		SetFileSize(fileSize).
		SetWidth(width).
		SetHeight(height).
		SetDisplayOrder(displayOrder).
		Save(ctx)

	if err != nil {
		rep.db.Logger().Error(fmt.Sprintf("Failed to create image for property [%d]: %s", propertyId, err.Error()))
		return models.InvalidRecordId, server_error.Wrap("IMAGE_INSERT", "failed to insert property image", err)
	}

	rep.db.Logger().Info(fmt.Sprintf("Image created successfully: %d", createdImage.ID))
	return models.RecordId(createdImage.ID), nil
}

func (rep *PropertyImageRepository) GetImageById(ctx context.Context, imageId models.RecordId) ([]byte, string, error) {
	if !imageId.IsValid() {
		return nil, "", server_error.New("INVALID_IMAGE_ID", "image ID is invalid")
	}

	img, err := rep.db.client.PropertyImage.Query().
		Where(propertyimage.IDEQ(int(imageId))).
		Only(ctx)

	if err != nil {
		if client.IsNotFound(err) {
			return nil, "", server_error.New("IMAGE_NOT_FOUND", "image not found")
		}
		return nil, "", server_error.Wrap("IMAGE_QUERY", "failed to query image", err)
	}

	return img.ImageData, img.ContentType, nil
}

func (rep *PropertyImageRepository) GetImagesByPropertyId(ctx context.Context, propertyId models.RecordId) ([]models.PropertyImageDTO, error) {
	if !propertyId.IsValid() {
		return nil, server_error.New("INVALID_PROPERTY_ID", "property ID is invalid")
	}

	images, err := rep.db.client.PropertyImage.Query().
		Where(propertyimage.PropertyIDEQ(int(propertyId))).
		Order(client.Asc(propertyimage.FieldDisplayOrder)).
		All(ctx)

	if err != nil {
		return nil, server_error.Wrap("IMAGE_LIST", "failed to list property images", err)
	}

	dtos := make([]models.PropertyImageDTO, len(images))
	for i, img := range images {
		dtos[i] = *rep.entToDTO(img)
	}

	return dtos, nil
}

func (rep *PropertyImageRepository) UpdateImageOrder(ctx context.Context, tx *client.Tx, imageId models.RecordId, newOrder int) error {
	if !imageId.IsValid() {
		return server_error.New("INVALID_IMAGE_ID", "image ID is invalid")
	}

	if newOrder < 0 {
		return server_error.New("IMAGE_VALIDATION", "display order must be non-negative")
	}

	err := tx.PropertyImage.UpdateOneID(int(imageId)).
		SetDisplayOrder(newOrder).
		Exec(ctx)

	if err != nil {
		if client.IsNotFound(err) {
			return server_error.New("IMAGE_NOT_FOUND", "image not found")
		}
		return server_error.Wrap("IMAGE_UPDATE_ORDER", "failed to update image order", err)
	}

	rep.db.Logger().Info(fmt.Sprintf("Image order updated: %d", imageId))
	return nil
}

func (rep *PropertyImageRepository) DeleteImage(ctx context.Context, tx *client.Tx, imageId models.RecordId) error {
	if !imageId.IsValid() {
		return server_error.New("INVALID_IMAGE_ID", "image ID is invalid")
	}

	rep.db.Logger().Debug(fmt.Sprintf("Deleting image: %d", imageId))

	err := tx.PropertyImage.DeleteOneID(int(imageId)).Exec(ctx)
	if err != nil {
		if client.IsNotFound(err) {
			return server_error.New("IMAGE_NOT_FOUND", "image not found")
		}
		rep.db.Logger().Error(fmt.Sprintf("Failed to delete image [%d]: %s", imageId, err.Error()))
		return server_error.Wrap("IMAGE_DELETE", "failed to delete image", err)
	}

	rep.db.Logger().Info(fmt.Sprintf("Image deleted successfully: %d", imageId))
	return nil
}

func (rep *PropertyImageRepository) DeleteImagesByPropertyId(ctx context.Context, tx *client.Tx, propertyId models.RecordId) error {
	if !propertyId.IsValid() {
		return server_error.New("INVALID_PROPERTY_ID", "property ID is invalid")
	}

	rep.db.Logger().Debug(fmt.Sprintf("Deleting all images for property: %d", propertyId))

	deleted, err := tx.PropertyImage.Delete().
		Where(propertyimage.PropertyIDEQ(int(propertyId))).
		Exec(ctx)

	if err != nil {
		rep.db.Logger().Error(fmt.Sprintf("Failed to delete images for property [%d]: %s", propertyId, err.Error()))
		return server_error.Wrap("IMAGE_DELETE_BATCH", "failed to delete property images", err)
	}

	rep.db.Logger().Info(fmt.Sprintf("Deleted %d images for property: %d", deleted, propertyId))
	return nil
}

func (rep *PropertyImageRepository) GetNextDisplayOrder(ctx context.Context, propertyId models.RecordId) (int, error) {
	if !propertyId.IsValid() {
		return 0, server_error.New("INVALID_PROPERTY_ID", "property ID is invalid")
	}

	images, err := rep.db.client.PropertyImage.Query().
		Where(propertyimage.PropertyIDEQ(int(propertyId))).
		Order(client.Desc(propertyimage.FieldDisplayOrder)).
		Limit(1).
		All(ctx)

	if err != nil {
		return 0, server_error.Wrap("IMAGE_QUERY", "failed to query max display order", err)
	}

	if len(images) == 0 {
		return 0, nil
	}

	return images[0].DisplayOrder + 1, nil
}

func (rep *PropertyImageRepository) entToDTO(img *client.PropertyImage) *models.PropertyImageDTO {
	imageId := models.RecordId(img.ID)
	propertyId := models.RecordId(img.PropertyID)

	return &models.PropertyImageDTO{
		ID:           &imageId,
		PropertyID:   &propertyId,
		ContentType:  &img.ContentType,
		FileSize:     &img.FileSize,
		Width:        &img.Width,
		Height:       &img.Height,
		DisplayOrder: &img.DisplayOrder,
		CreatedAt:    &img.CreatedAt,
	}
}
