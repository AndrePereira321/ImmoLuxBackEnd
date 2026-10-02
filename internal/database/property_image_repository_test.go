package database

import (
	"context"
	"immo-lux/internal/database/ent/client"
	"immo-lux/internal/database/ent/client/propertyimage"
	"immo-lux/internal/models"
	"immo-lux/internal/server_error"
	"immo-lux/internal/utils"
	"sync"
	"testing"
	"time"
)

func countImages(t *testing.T, db *Database, propertyId models.RecordId) int {
	t.Helper()
	count, err := db.client.PropertyImage.Query().
		Where(propertyimage.PropertyIDEQ(int(propertyId))).
		Count(context.Background())
	if err != nil {
		t.Fatalf("failed to count images: %v", err)
	}
	return count
}

// insertImages stores n images directly, bypassing UploadImage, to set up a
// gallery quickly.
func insertImages(t *testing.T, db *Database, propertyId models.RecordId, n int) {
	t.Helper()
	processed, err := utils.ProcessImage(testPNG(t), "image/png")
	if err != nil {
		t.Fatalf("failed to process test image: %v", err)
	}
	builders := make([]*client.PropertyImageCreate, n)
	for i := range builders {
		builders[i] = db.client.PropertyImage.Create().
			SetPropertyID(int(propertyId)).
			SetImageData(processed.Data).
			SetContentType(processed.ContentType).
			SetFileSize(processed.FileSize).
			SetWidth(processed.Width).
			SetHeight(processed.Height).
			SetDisplayOrder(i)
	}
	if _, err := db.client.PropertyImage.CreateBulk(builders...).Save(context.Background()); err != nil {
		t.Fatalf("failed to insert images: %v", err)
	}
}

func TestUploadImageLimit(t *testing.T) {
	db := newTestDatabase(t)
	ctx := context.Background()
	repo := db.NewPropertyImageRepository()
	owner := createTestUser(t, db, "owner@test.local")
	propertyId := createTestProperty(t, db, owner)
	data := testPNG(t)

	var lastImageId models.RecordId
	for i := 0; i < MaxImagesPerProperty; i++ {
		imageId, err := repo.UploadImage(ctx, owner, propertyId, data, "image/png", nil)
		if err != nil {
			t.Fatalf("upload %d of %d failed: %v", i+1, MaxImagesPerProperty, err)
		}
		lastImageId = imageId
	}

	_, err := repo.UploadImage(ctx, owner, propertyId, data, "image/png", nil)
	requireServerError(t, err, server_error.KindInvalid, "IMAGE_LIMIT_REACHED")
	if got := countImages(t, db, propertyId); got != MaxImagesPerProperty {
		t.Fatalf("expected %d images, got %d", MaxImagesPerProperty, got)
	}

	// Deleting one frees a slot again.
	if err := repo.DeleteImage(ctx, owner, lastImageId); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if _, err := repo.UploadImage(ctx, owner, propertyId, data, "image/png", nil); err != nil {
		t.Fatalf("upload after delete failed: %v", err)
	}
	if got := countImages(t, db, propertyId); got != MaxImagesPerProperty {
		t.Fatalf("expected %d images, got %d", MaxImagesPerProperty, got)
	}
}

func TestUploadImageLimitIsPerProperty(t *testing.T) {
	db := newTestDatabase(t)
	owner := createTestUser(t, db, "owner@test.local")
	full := createTestProperty(t, db, owner)
	other := createTestProperty(t, db, owner)
	insertImages(t, db, full, MaxImagesPerProperty)

	if _, err := db.NewPropertyImageRepository().UploadImage(context.Background(), owner, other, testPNG(t), "image/png", nil); err != nil {
		t.Fatalf("a full gallery must not block other properties: %v", err)
	}
}

func TestUploadImageLimitUnderConcurrency(t *testing.T) {
	db := newTestDatabase(t)
	repo := db.NewPropertyImageRepository()
	owner := createTestUser(t, db, "owner@test.local")
	data := testPNG(t)
	const free, uploaders = 2, 12

	// Several rounds, since a race only shows up some of the time.
	for round := 0; round < 5; round++ {
		propertyId := createTestProperty(t, db, owner)
		insertImages(t, db, propertyId, MaxImagesPerProperty-free)

		var wg sync.WaitGroup
		start := make(chan struct{})
		errs := make([]error, uploaders)
		for i := 0; i < uploaders; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				_, errs[i] = repo.UploadImage(context.Background(), owner, propertyId, data, "image/png", nil)
			}(i)
		}
		close(start)
		wg.Wait()

		succeeded := 0
		for _, err := range errs {
			if err == nil {
				succeeded++
				continue
			}
			requireServerError(t, err, server_error.KindInvalid, "IMAGE_LIMIT_REACHED")
		}
		if succeeded != free {
			t.Fatalf("round %d: expected exactly %d uploads to succeed, got %d", round, free, succeeded)
		}
		if got := countImages(t, db, propertyId); got != MaxImagesPerProperty {
			t.Fatalf("round %d: expected %d images, got %d", round, MaxImagesPerProperty, got)
		}
	}
}

func TestUploadImageOwnership(t *testing.T) {
	db := newTestDatabase(t)
	ctx := context.Background()
	repo := db.NewPropertyImageRepository()
	owner := createTestUser(t, db, "owner@test.local")
	stranger := createTestUser(t, db, "stranger@test.local")
	propertyId := createTestProperty(t, db, owner)
	data := testPNG(t)

	_, err := repo.UploadImage(ctx, stranger, propertyId, data, "image/png", nil)
	requireServerError(t, err, server_error.KindForbidden, "ACCESS_DENIED")

	_, err = repo.UploadImage(ctx, owner, propertyId+1000, data, "image/png", nil)
	requireServerError(t, err, server_error.KindNotFound, "PROPERTY_NOT_FOUND")

	_, err = repo.UploadImage(ctx, owner, models.InvalidRecordId, data, "image/png", nil)
	requireServerError(t, err, server_error.KindInvalid, "INVALID_PROPERTY_ID")

	if got := countImages(t, db, propertyId); got != 0 {
		t.Fatalf("rejected uploads must not store images, found %d", got)
	}
}

func TestLockOwned(t *testing.T) {
	db := newTestDatabase(t)
	ctx := context.Background()
	repo := db.NewPropertyRepository()
	owner := createTestUser(t, db, "owner@test.local")
	stranger := createTestUser(t, db, "stranger@test.local")
	propertyId := createTestProperty(t, db, owner)

	before, err := db.client.Property.Get(ctx, int(propertyId))
	if err != nil {
		t.Fatalf("failed to load property: %v", err)
	}
	time.Sleep(10 * time.Millisecond)

	lockAs := func(userId, id models.RecordId) error {
		return db.WithTransaction(ctx, func(txCtx context.Context, tx *client.Tx) error {
			return repo.lockOwned(txCtx, tx, userId, id)
		})
	}

	if err := lockAs(owner, propertyId); err != nil {
		t.Fatalf("owner lock failed: %v", err)
	}
	after, err := db.client.Property.Get(ctx, int(propertyId))
	if err != nil {
		t.Fatalf("failed to reload property: %v", err)
	}
	if !after.UpdatedAt.After(before.UpdatedAt) {
		t.Error("lockOwned should touch updated_at")
	}

	requireServerError(t, lockAs(stranger, propertyId), server_error.KindForbidden, "ACCESS_DENIED")
	requireServerError(t, lockAs(owner, propertyId+1000), server_error.KindNotFound, "PROPERTY_NOT_FOUND")
	requireServerError(t, lockAs(owner, models.InvalidRecordId), server_error.KindInvalid, "INVALID_PROPERTY_ID")
}

// An upload in progress holds the property lock while it inserts an image.
// DeleteProperty must wait for it and then delete that image too; without the
// lock it deleted the gallery first and then failed on the image foreign key.
func TestDeletePropertyWaitsForUploadInProgress(t *testing.T) {
	db := newTestDatabase(t)
	ctx := context.Background()
	owner := createTestUser(t, db, "owner@test.local")
	propertyId := createTestProperty(t, db, owner)
	processed, err := utils.ProcessImage(testPNG(t), "image/png")
	if err != nil {
		t.Fatalf("failed to process test image: %v", err)
	}

	uploadTx, err := db.client.Tx(ctx)
	if err != nil {
		t.Fatalf("failed to start upload transaction: %v", err)
	}
	if err := db.NewPropertyRepository().lockOwned(ctx, uploadTx, owner, propertyId); err != nil {
		_ = uploadTx.Rollback()
		t.Fatalf("lockOwned failed: %v", err)
	}

	deleteErr := make(chan error, 1)
	go func() {
		deleteErr <- db.NewPropertyRepository().DeleteProperty(ctx, owner, propertyId)
	}()

	// Give DeleteProperty time to reach the lock and block behind the upload.
	select {
	case err := <-deleteErr:
		_ = uploadTx.Rollback()
		t.Fatalf("DeleteProperty should wait for the in-progress upload, returned early: %v", err)
	case <-time.After(500 * time.Millisecond):
	}

	_, err = uploadTx.PropertyImage.Create().
		SetPropertyID(int(propertyId)).
		SetImageData(processed.Data).
		SetContentType(processed.ContentType).
		SetFileSize(processed.FileSize).
		SetWidth(processed.Width).
		SetHeight(processed.Height).
		Save(ctx)
	if err != nil {
		_ = uploadTx.Rollback()
		t.Fatalf("failed to insert image in upload transaction: %v", err)
	}
	if err := uploadTx.Commit(); err != nil {
		t.Fatalf("failed to commit upload: %v", err)
	}

	select {
	case err := <-deleteErr:
		if err != nil {
			t.Fatalf("DeleteProperty failed after the upload committed: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("DeleteProperty did not finish")
	}
	if got := countImages(t, db, propertyId); got != 0 {
		t.Errorf("the uploaded image should have been deleted with the property, %d left", got)
	}

	// An upload that starts after the delete finds the property gone.
	_, err = db.NewPropertyImageRepository().UploadImage(ctx, owner, propertyId, testPNG(t), "image/png", nil)
	requireServerError(t, err, server_error.KindNotFound, "PROPERTY_NOT_FOUND")
}
