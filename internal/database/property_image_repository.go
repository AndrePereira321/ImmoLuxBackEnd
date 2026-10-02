package database

import (
	"context"
	"fmt"

	"immo-lux/internal/database/ent/client"
	"immo-lux/internal/database/ent/client/property"
	"immo-lux/internal/database/ent/client/propertyimage"
	"immo-lux/internal/models"
	"immo-lux/internal/server_error"
	"immo-lux/internal/utils"
)

// MaxImagesPerProperty caps a property's gallery. The panel allows 10 images
// and deletes removed ones before uploading new ones, so this leaves headroom
// while bounding how much image data a single property can hold.
const MaxImagesPerProperty = 20

type PropertyImageRepository struct {
	db *Database
}

func NewPropertyImageRepository(db *Database) *PropertyImageRepository {
	return &PropertyImageRepository{db: db}
}

// imageMetadataColumns is every generated column except image_data, derived so
// a new schema field joins the projection automatically. No metadata query may
// read the blob; only ImagePayload does.
var imageMetadataColumns = func() []string {
	columns := make([]string, 0, len(propertyimage.Columns)-1)
	for _, column := range propertyimage.Columns {
		if column != propertyimage.FieldImageData {
			columns = append(columns, column)
		}
	}
	return columns
}()

// UploadImage processes and stores a new image on a property ownerId owns. The
// payload is normalised (JPEG, bounded dimensions) before storage; a nil
// requestedOrder appends the image after the current gallery. A property holds
// at most MaxImagesPerProperty images.
func (rep *PropertyImageRepository) UploadImage(ctx context.Context, ownerId, propertyId models.RecordId, data []byte, contentType string, requestedOrder *int) (models.RecordId, error) {
	if !propertyId.IsValid() {
		return models.InvalidRecordId, server_error.Invalid("INVALID_PROPERTY_ID", "property ID is invalid")
	}
	if requestedOrder != nil && *requestedOrder < 0 {
		return models.InvalidRecordId, server_error.Invalid("IMAGE_VALIDATION", "display order must be non-negative")
	}

	// Certify ownership and the gallery limit before paying for the decode; the
	// in-transaction checks below are the ones that hold at write time.
	owned, err := rep.db.client.Property.Query().
		Where(property.IDEQ(int(propertyId)), property.PublisherIDEQ(int(ownerId))).
		Exist(ctx)
	if err != nil {
		return models.InvalidRecordId, server_error.Wrap("PROPERTY_QUERY", "failed to check property ownership", err)
	}
	if !owned {
		propertyRepo := NewPropertyRepository(rep.db)
		return models.InvalidRecordId, propertyRepo.ownershipVerdict(ctx, func(c context.Context) (bool, error) {
			return rep.db.client.Property.Query().Where(property.IDEQ(int(propertyId))).Exist(c)
		})
	}
	if err := ensureImageCapacity(ctx, rep.db.client.PropertyImage, propertyId); err != nil {
		return models.InvalidRecordId, err
	}

	processed, err := utils.ProcessImage(data, contentType)
	if err != nil {
		return models.InvalidRecordId, err
	}

	var imageId models.RecordId
	err = rep.db.WithTransaction(ctx, func(txCtx context.Context, tx *client.Tx) error {
		// The property row lock serialises concurrent uploads, so the count
		// below cannot be raced past the limit.
		if err := NewPropertyRepository(rep.db).lockOwned(txCtx, tx, ownerId, propertyId); err != nil {
			return err
		}
		if err := ensureImageCapacity(txCtx, tx.PropertyImage, propertyId); err != nil {
			return err
		}

		var displayOrder int
		if requestedOrder != nil {
			displayOrder = *requestedOrder
		} else {
			nextOrder, err := nextDisplayOrder(txCtx, tx, propertyId)
			if err != nil {
				return err
			}
			displayOrder = nextOrder
		}

		createdImage, err := tx.PropertyImage.Create().
			SetPropertyID(int(propertyId)).
			SetImageData(processed.Data).
			SetContentType(processed.ContentType).
			SetFileSize(processed.FileSize).
			SetWidth(processed.Width).
			SetHeight(processed.Height).
			SetDisplayOrder(displayOrder).
			Save(txCtx)
		if err != nil {
			rep.db.Logger().Error(fmt.Sprintf("Failed to create image for property [%d]: %s", propertyId, err.Error()))
			return server_error.Wrap("IMAGE_INSERT", "failed to insert property image", err)
		}

		imageId = models.RecordId(createdImage.ID)
		return nil
	})
	if err != nil {
		return models.InvalidRecordId, err
	}

	rep.db.Logger().Info(fmt.Sprintf("Image uploaded for property %d: %d", propertyId, imageId))
	return imageId, nil
}

// UpdateImageOrder moves an image within a gallery ownerId owns, with one
// owner-predicated statement; the probes run only when nothing matched.
func (rep *PropertyImageRepository) UpdateImageOrder(ctx context.Context, ownerId, imageId models.RecordId, newOrder int) error {
	if !imageId.IsValid() {
		return server_error.Invalid("INVALID_IMAGE_ID", "image ID is invalid")
	}
	if newOrder < 0 {
		return server_error.Invalid("IMAGE_VALIDATION", "display order must be non-negative")
	}

	updated, err := rep.db.client.PropertyImage.Update().
		Where(
			propertyimage.IDEQ(int(imageId)),
			propertyimage.HasPropertyWith(property.PublisherIDEQ(int(ownerId))),
		).
		SetDisplayOrder(newOrder).
		Save(ctx)
	if err != nil {
		return server_error.Wrap("IMAGE_UPDATE_ORDER", "failed to update image order", err)
	}
	if updated == 0 {
		return rep.imageOwnershipVerdict(ctx, imageId)
	}

	rep.db.Logger().Info(fmt.Sprintf("Image order updated: %d", imageId))
	return nil
}

// DeleteImage removes an image from a gallery ownerId owns, with one
// owner-predicated statement; the probes run only when nothing matched.
func (rep *PropertyImageRepository) DeleteImage(ctx context.Context, ownerId, imageId models.RecordId) error {
	if !imageId.IsValid() {
		return server_error.Invalid("INVALID_IMAGE_ID", "image ID is invalid")
	}

	deleted, err := rep.db.client.PropertyImage.Delete().
		Where(
			propertyimage.IDEQ(int(imageId)),
			propertyimage.HasPropertyWith(property.PublisherIDEQ(int(ownerId))),
		).
		Exec(ctx)
	if err != nil {
		rep.db.Logger().Error(fmt.Sprintf("Failed to delete image [%d]: %s", imageId, err.Error()))
		return server_error.Wrap("IMAGE_DELETE", "failed to delete image", err)
	}
	if deleted == 0 {
		return rep.imageOwnershipVerdict(ctx, imageId)
	}

	rep.db.Logger().Info(fmt.Sprintf("Image deleted successfully: %d", imageId))
	return nil
}

// deleteImagesByPropertyId clears a property's gallery inside the caller's
// transaction; the property delete operation runs it before removing the row.
func (rep *PropertyImageRepository) deleteImagesByPropertyId(ctx context.Context, tx *client.Tx, propertyId models.RecordId) error {
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

// ListImageMetadata returns a property's gallery metadata in display order.
func (rep *PropertyImageRepository) ListImageMetadata(ctx context.Context, propertyId models.RecordId) ([]models.PropertyImageDTO, error) {
	if !propertyId.IsValid() {
		return nil, server_error.Invalid("INVALID_PROPERTY_ID", "property ID is invalid")
	}

	images, err := rep.db.client.PropertyImage.Query().
		Where(propertyimage.PropertyIDEQ(int(propertyId))).
		Order(client.Asc(propertyimage.FieldDisplayOrder)).
		Select(imageMetadataColumns...).
		All(ctx)
	if err != nil {
		return nil, server_error.Wrap("IMAGE_LIST", "failed to list property images", err)
	}

	dtos := make([]models.PropertyImageDTO, len(images))
	for i, img := range images {
		dtos[i] = *imageMetadataToDTO(img)
	}
	return dtos, nil
}

// ImagePayload returns the stored bytes and content type of one image — the
// only query that reads image_data.
func (rep *PropertyImageRepository) ImagePayload(ctx context.Context, imageId models.RecordId) ([]byte, string, error) {
	if !imageId.IsValid() {
		return nil, "", server_error.Invalid("INVALID_IMAGE_ID", "image ID is invalid")
	}

	img, err := rep.db.client.PropertyImage.Query().
		Where(propertyimage.IDEQ(int(imageId))).
		Select(propertyimage.FieldImageData, propertyimage.FieldContentType).
		Only(ctx)
	if err != nil {
		if client.IsNotFound(err) {
			return nil, "", server_error.NotFound("IMAGE_NOT_FOUND", "image not found")
		}
		return nil, "", server_error.Wrap("IMAGE_QUERY", "failed to query image", err)
	}

	return img.ImageData, img.ContentType, nil
}

// imageOwnershipVerdict explains an owner-predicated image statement that
// matched nothing.
func (rep *PropertyImageRepository) imageOwnershipVerdict(ctx context.Context, imageId models.RecordId) error {
	return ownershipVerdict(ctx,
		func(c context.Context) (bool, error) {
			return rep.db.client.PropertyImage.Query().Where(propertyimage.IDEQ(int(imageId))).Exist(c)
		},
		"IMAGE_QUERY",
		server_error.NotFound("IMAGE_NOT_FOUND", "image not found"),
		server_error.Forbidden("ACCESS_DENIED", "you can only modify images of your own properties"),
	)
}

// ensureImageCapacity fails with IMAGE_LIMIT_REACHED when the property already
// holds MaxImagesPerProperty images.
func ensureImageCapacity(ctx context.Context, images *client.PropertyImageClient, propertyId models.RecordId) error {
	count, err := images.Query().
		Where(propertyimage.PropertyIDEQ(int(propertyId))).
		Count(ctx)
	if err != nil {
		return server_error.Wrap("IMAGE_QUERY", "failed to count property images", err)
	}
	if count >= MaxImagesPerProperty {
		return server_error.Invalid("IMAGE_LIMIT_REACHED",
			fmt.Sprintf("a property can have at most %d images", MaxImagesPerProperty))
	}
	return nil
}

func nextDisplayOrder(ctx context.Context, tx *client.Tx, propertyId models.RecordId) (int, error) {
	orders, err := tx.PropertyImage.Query().
		Where(propertyimage.PropertyIDEQ(int(propertyId))).
		Order(client.Desc(propertyimage.FieldDisplayOrder)).
		Limit(1).
		Select(propertyimage.FieldDisplayOrder).
		Ints(ctx)
	if err != nil {
		return 0, server_error.Wrap("IMAGE_QUERY", "failed to query max display order", err)
	}
	if len(orders) == 0 {
		return 0, nil
	}
	return orders[0] + 1, nil
}

func imageMetadataToDTO(img *client.PropertyImage) *models.PropertyImageDTO {
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
