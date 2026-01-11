package routes

import (
	"context"
	"fmt"
	"strconv"

	"immo-lux/internal/database"
	"immo-lux/internal/database/ent/client"
	"immo-lux/internal/models"
	"immo-lux/internal/server_error"

	"github.com/gofiber/fiber/v3"
)

type ContactPayload struct {
	ID    *int64  `json:"id"`
	Name  *string `json:"name"`
	Email *string `json:"email"`
	Phone *string `json:"phone"`
	Notes *string `json:"notes"`
}

type CreatePropertyPayload struct {
	Title          string          `json:"title"`
	Description    string          `json:"description"`
	PropertyType   string          `json:"propertyType"`
	Price          float64         `json:"price"`
	Status         string          `json:"status"`
	Address        string          `json:"address"`
	District       string          `json:"district"`
	Municipality   string          `json:"municipality"`
	Parish         *string         `json:"parish"`
	PostalCode     *string         `json:"postalCode"`
	Country        string          `json:"country"`
	Latitude       *float64        `json:"latitude"`
	Longitude      *float64        `json:"longitude"`
	Bedrooms       *int            `json:"bedrooms"`
	Bathrooms      *int            `json:"bathrooms"`
	AreaSqm        *float64        `json:"areaSqm"`
	LandAreaSqm    *float64        `json:"landAreaSqm"`
	YearBuilt      *int            `json:"yearBuilt"`
	Floor          *int            `json:"floor"`
	TotalFloors    *int            `json:"totalFloors"`
	ParkingSpaces  *int            `json:"parkingSpaces"`
	HasGarage      bool            `json:"hasGarage"`
	HasGarden      bool            `json:"hasGarden"`
	HasPool        bool            `json:"hasPool"`
	HasElevator    bool            `json:"hasElevator"`
	EnergyRating   *string         `json:"energyRating"`
	VirtualTourURL *string         `json:"virtualTourUrl"`
	Contact        *ContactPayload `json:"contact"`
}

type UpdatePropertyPayload struct {
	Title          *string  `json:"title"`
	Description    *string  `json:"description"`
	PropertyType   *string  `json:"propertyType"`
	Price          *float64 `json:"price"`
	Status         *string  `json:"status"`
	IsPublished    *bool    `json:"isPublished"`
	Address        *string  `json:"address"`
	District       *string  `json:"district"`
	Municipality   *string  `json:"municipality"`
	Parish         *string  `json:"parish"`
	PostalCode     *string  `json:"postalCode"`
	Country        *string  `json:"country"`
	Latitude       *float64 `json:"latitude"`
	Longitude      *float64 `json:"longitude"`
	Bedrooms       *int     `json:"bedrooms"`
	Bathrooms      *int     `json:"bathrooms"`
	AreaSqm        *float64 `json:"areaSqm"`
	LandAreaSqm    *float64 `json:"landAreaSqm"`
	YearBuilt      *int     `json:"yearBuilt"`
	Floor          *int     `json:"floor"`
	TotalFloors    *int     `json:"totalFloors"`
	ParkingSpaces  *int     `json:"parkingSpaces"`
	HasGarage      *bool    `json:"hasGarage"`
	HasGarden      *bool    `json:"hasGarden"`
	HasPool        *bool    `json:"hasPool"`
	HasElevator    *bool    `json:"hasElevator"`
	EnergyRating   *string  `json:"energyRating"`
	VirtualTourURL *string  `json:"virtualTourUrl"`
	ContactID      *int64   `json:"contactId"`
}

func CreateProperty(ctx *RouteContext) error {
	payload := &CreatePropertyPayload{}
	if err := ctx.ReadBody(&payload); err != nil {
		return err
	}

	userId := ctx.GetUserId()
	if !userId.IsValid() {
		return ctx.RespondError(fiber.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated")
	}

	if payload.Contact == nil {
		return ctx.BadRequest("Contact information is required")
	}

	var contactId models.RecordId
	var propertyId models.RecordId

	err := ctx.Db().WithTransaction(func(txCtx context.Context, tx *client.Tx) error {
		if payload.Contact.ID != nil {
			contactId = models.RecordId(*payload.Contact.ID)

			existingContact, err := ctx.Db().NewContactRepository().GetContactById(txCtx, contactId)
			if err != nil {
				return err
			}

			if *existingContact.UserID != userId {
				return server_error.New("CONTACT_ACCESS_DENIED", "You can only use your own contacts")
			}
		} else {
			if payload.Contact.Name == nil || payload.Contact.Email == nil || payload.Contact.Phone == nil {
				return server_error.New("CONTACT_VALIDATION", "Contact name, email, and phone are required")
			}

			contactDTO := &models.ContactDTO{
				UserID: &userId,
				Name:   payload.Contact.Name,
				Email:  payload.Contact.Email,
				Phone:  payload.Contact.Phone,
				Notes:  payload.Contact.Notes,
			}

			id, err := ctx.Db().NewContactRepository().CreateContact(txCtx, tx, contactDTO)
			if err != nil {
				return err
			}
			contactId = id
		}

		isPublished := false
		propertyDTO := &models.PropertyDTO{
			Title:          &payload.Title,
			Description:    &payload.Description,
			PropertyType:   &payload.PropertyType,
			Price:          &payload.Price,
			Status:         &payload.Status,
			IsPublished:    &isPublished,
			Address:        &payload.Address,
			District:       &payload.District,
			Municipality:   &payload.Municipality,
			Parish:         payload.Parish,
			PostalCode:     payload.PostalCode,
			Country:        &payload.Country,
			Latitude:       payload.Latitude,
			Longitude:      payload.Longitude,
			Bedrooms:       payload.Bedrooms,
			Bathrooms:      payload.Bathrooms,
			AreaSqm:        payload.AreaSqm,
			LandAreaSqm:    payload.LandAreaSqm,
			YearBuilt:      payload.YearBuilt,
			Floor:          payload.Floor,
			TotalFloors:    payload.TotalFloors,
			ParkingSpaces:  payload.ParkingSpaces,
			HasGarage:      &payload.HasGarage,
			HasGarden:      &payload.HasGarden,
			HasPool:        &payload.HasPool,
			HasElevator:    &payload.HasElevator,
			EnergyRating:   payload.EnergyRating,
			VirtualTourURL: payload.VirtualTourURL,
			ContactID:      &contactId,
			PublisherID:    &userId,
		}

		id, err := ctx.Db().NewPropertyRepository().CreateProperty(txCtx, tx, propertyDTO)
		if err != nil {
			return err
		}
		propertyId = id
		return nil
	})

	if err != nil {
		ctx.Logger().Error(fmt.Sprintf("Failed to create property: %s", err.Error()))
		return err
	}

	ctx.Logger().Info(fmt.Sprintf("Property created by user %d: %d", userId, propertyId))

	createdProperty, err := ctx.Db().NewPropertyRepository().GetPropertyById(context.Background(), propertyId)
	if err != nil {
		return err
	}

	contact, err := ctx.Db().NewContactRepository().GetContactById(context.Background(), contactId)
	if err == nil {
		createdProperty.Contact = contact
	}

	return ctx.RespondData(createdProperty)
}

func ListMyProperties(ctx *RouteContext) error {
	userId := ctx.GetUserId()
	if !userId.IsValid() {
		return ctx.RespondError(fiber.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated")
	}

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
		PublisherID: &userId,
		Limit:       limit,
		Offset:      offset,
	}

	if orderBy := ctx.Ctx().Query("orderBy"); orderBy != "" {
		filters.OrderBy = &orderBy
	}

	properties, total, err := ctx.Db().NewPropertyRepository().ListProperties(context.Background(), filters)
	if err != nil {
		ctx.Logger().Error(fmt.Sprintf("Failed to list user properties: %s", err.Error()))
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

func UpdateProperty(ctx *RouteContext) error {
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
		return ctx.RespondError(fiber.StatusForbidden, "ACCESS_DENIED", "You can only update your own properties")
	}

	payload := &UpdatePropertyPayload{}
	if err := ctx.ReadBody(&payload); err != nil {
		return err
	}

	if payload.ContactID != nil {
		contactId := models.RecordId(*payload.ContactID)
		existingContact, err := ctx.Db().NewContactRepository().GetContactById(context.Background(), contactId)
		if err != nil {
			return ctx.BadRequest("Invalid contact ID")
		}
		if *existingContact.UserID != userId {
			return ctx.RespondError(fiber.StatusForbidden, "CONTACT_ACCESS_DENIED", "You can only use your own contacts")
		}
	}

	propertyDTO := &models.PropertyDTO{
		Title:          payload.Title,
		Description:    payload.Description,
		PropertyType:   payload.PropertyType,
		Price:          payload.Price,
		Status:         payload.Status,
		IsPublished:    payload.IsPublished,
		Address:        payload.Address,
		District:       payload.District,
		Municipality:   payload.Municipality,
		Parish:         payload.Parish,
		PostalCode:     payload.PostalCode,
		Country:        payload.Country,
		Latitude:       payload.Latitude,
		Longitude:      payload.Longitude,
		Bedrooms:       payload.Bedrooms,
		Bathrooms:      payload.Bathrooms,
		AreaSqm:        payload.AreaSqm,
		LandAreaSqm:    payload.LandAreaSqm,
		YearBuilt:      payload.YearBuilt,
		Floor:          payload.Floor,
		TotalFloors:    payload.TotalFloors,
		ParkingSpaces:  payload.ParkingSpaces,
		HasGarage:      payload.HasGarage,
		HasGarden:      payload.HasGarden,
		HasPool:        payload.HasPool,
		HasElevator:    payload.HasElevator,
		EnergyRating:   payload.EnergyRating,
		VirtualTourURL: payload.VirtualTourURL,
	}

	if payload.ContactID != nil {
		contactId := models.RecordId(*payload.ContactID)
		propertyDTO.ContactID = &contactId
	}

	err = ctx.Db().WithTransaction(func(txCtx context.Context, tx *client.Tx) error {
		return ctx.Db().NewPropertyRepository().UpdateProperty(txCtx, tx, propertyId, propertyDTO)
	})

	if err != nil {
		ctx.Logger().Error(fmt.Sprintf("Failed to update property %d: %s", propertyId, err.Error()))
		return err
	}

	ctx.Logger().Info(fmt.Sprintf("Property updated by user %d: %d", userId, propertyId))

	updatedProperty, err := ctx.Db().NewPropertyRepository().GetPropertyById(context.Background(), propertyId)
	if err != nil {
		return err
	}

	if updatedProperty.ContactID != nil {
		contact, err := ctx.Db().NewContactRepository().GetContactById(context.Background(), *updatedProperty.ContactID)
		if err == nil {
			updatedProperty.Contact = contact
		}
	}

	return ctx.RespondData(updatedProperty)
}

func DeleteProperty(ctx *RouteContext) error {
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
		return ctx.RespondError(fiber.StatusForbidden, "ACCESS_DENIED", "You can only delete your own properties")
	}

	err = ctx.Db().WithTransaction(func(txCtx context.Context, tx *client.Tx) error {
		if err := ctx.Db().NewPropertyImageRepository().DeleteImagesByPropertyId(txCtx, tx, propertyId); err != nil {
			return err
		}
		return ctx.Db().NewPropertyRepository().DeleteProperty(txCtx, tx, propertyId)
	})

	if err != nil {
		ctx.Logger().Error(fmt.Sprintf("Failed to delete property %d: %s", propertyId, err.Error()))
		return err
	}

	ctx.Logger().Info(fmt.Sprintf("Property deleted by user %d: %d", userId, propertyId))

	return ctx.RespondData(&fiber.Map{
		"success": true,
		"message": "Property deleted successfully",
	})
}

func PublishProperty(ctx *RouteContext) error {
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
		return ctx.RespondError(fiber.StatusForbidden, "ACCESS_DENIED", "You can only publish your own properties")
	}

	err = ctx.Db().WithTransaction(func(txCtx context.Context, tx *client.Tx) error {
		return ctx.Db().NewPropertyRepository().PublishProperty(txCtx, tx, propertyId)
	})

	if err != nil {
		ctx.Logger().Error(fmt.Sprintf("Failed to publish property %d: %s", propertyId, err.Error()))
		return err
	}

	ctx.Logger().Info(fmt.Sprintf("Property published by user %d: %d", userId, propertyId))

	return ctx.RespondData(&fiber.Map{
		"success": true,
		"message": "Property published successfully",
	})
}

func UnpublishProperty(ctx *RouteContext) error {
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
		return ctx.RespondError(fiber.StatusForbidden, "ACCESS_DENIED", "You can only unpublish your own properties")
	}

	err = ctx.Db().WithTransaction(func(txCtx context.Context, tx *client.Tx) error {
		return ctx.Db().NewPropertyRepository().UnpublishProperty(txCtx, tx, propertyId)
	})

	if err != nil {
		ctx.Logger().Error(fmt.Sprintf("Failed to unpublish property %d: %s", propertyId, err.Error()))
		return err
	}

	ctx.Logger().Info(fmt.Sprintf("Property unpublished by user %d: %d", userId, propertyId))

	return ctx.RespondData(&fiber.Map{
		"success": true,
		"message": "Property unpublished successfully",
	})
}
