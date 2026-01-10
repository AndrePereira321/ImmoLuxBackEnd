package database

import (
	"context"
	"fmt"
	"time"

	"immo-lux/internal/database/ent/client"
	"immo-lux/internal/database/ent/client/property"
	"immo-lux/internal/models"
	"immo-lux/internal/server_error"
	"immo-lux/internal/utils"
)

type PropertyRepository struct {
	db *Database
}

func NewPropertyRepository(db *Database) *PropertyRepository {
	return &PropertyRepository{db: db}
}

type PropertyFilters struct {
	District     *string
	Municipality *string
	Parish       *string
	PropertyType *string
	Status       *string
	MinPrice     *float64
	MaxPrice     *float64
	IsPublished  *bool
	PublisherID  *models.RecordId
	Limit        int
	Offset       int
}

func (rep *PropertyRepository) CreateProperty(ctx context.Context, tx *client.Tx, propertyDTO *models.PropertyDTO) (models.RecordId, error) {
	if err := rep.validatePropertyInput(propertyDTO); err != nil {
		return models.InvalidRecordId, err
	}

	rep.db.Logger().Debug(fmt.Sprintf("Creating property: %s", *propertyDTO.Title))

	builder := tx.Property.Create().
		SetTitle(*propertyDTO.Title).
		SetDescription(*propertyDTO.Description).
		SetPropertyType(property.PropertyType(*propertyDTO.PropertyType)).
		SetPrice(*propertyDTO.Price).
		SetStatus(property.Status(*propertyDTO.Status)).
		SetAddress(*propertyDTO.Address).
		SetDistrict(*propertyDTO.District).
		SetMunicipality(*propertyDTO.Municipality).
		SetCountry(*propertyDTO.Country).
		SetContactID(int(*propertyDTO.ContactID)).
		SetPublisherID(int(*propertyDTO.PublisherID))

	if propertyDTO.IsPublished != nil {
		builder.SetIsPublished(*propertyDTO.IsPublished)
	}
	if propertyDTO.Parish != nil {
		builder.SetParish(*propertyDTO.Parish)
	}
	if propertyDTO.PostalCode != nil {
		builder.SetPostalCode(*propertyDTO.PostalCode)
	}
	if propertyDTO.Latitude != nil {
		builder.SetLatitude(*propertyDTO.Latitude)
	}
	if propertyDTO.Longitude != nil {
		builder.SetLongitude(*propertyDTO.Longitude)
	}
	if propertyDTO.Bedrooms != nil {
		builder.SetBedrooms(*propertyDTO.Bedrooms)
	}
	if propertyDTO.Bathrooms != nil {
		builder.SetBathrooms(*propertyDTO.Bathrooms)
	}
	if propertyDTO.AreaSqm != nil {
		builder.SetAreaSqm(*propertyDTO.AreaSqm)
	}
	if propertyDTO.LandAreaSqm != nil {
		builder.SetLandAreaSqm(*propertyDTO.LandAreaSqm)
	}
	if propertyDTO.YearBuilt != nil {
		builder.SetYearBuilt(*propertyDTO.YearBuilt)
	}
	if propertyDTO.Floor != nil {
		builder.SetFloor(*propertyDTO.Floor)
	}
	if propertyDTO.TotalFloors != nil {
		builder.SetTotalFloors(*propertyDTO.TotalFloors)
	}
	if propertyDTO.ParkingSpaces != nil {
		builder.SetParkingSpaces(*propertyDTO.ParkingSpaces)
	}
	if propertyDTO.HasGarage != nil {
		builder.SetHasGarage(*propertyDTO.HasGarage)
	}
	if propertyDTO.HasGarden != nil {
		builder.SetHasGarden(*propertyDTO.HasGarden)
	}
	if propertyDTO.HasPool != nil {
		builder.SetHasPool(*propertyDTO.HasPool)
	}
	if propertyDTO.HasElevator != nil {
		builder.SetHasElevator(*propertyDTO.HasElevator)
	}
	if propertyDTO.EnergyRating != nil {
		builder.SetEnergyRating(property.EnergyRating(*propertyDTO.EnergyRating))
	}
	if propertyDTO.VirtualTourURL != nil {
		builder.SetVirtualTourURL(*propertyDTO.VirtualTourURL)
	}

	createdProperty, err := builder.Save(ctx)
	if err != nil {
		rep.db.Logger().Error(fmt.Sprintf("Failed to create property [%s]: %s", *propertyDTO.Title, err.Error()))
		return models.InvalidRecordId, server_error.Wrap("PROPERTY_INSERT", "failed to insert property", err)
	}

	rep.db.Logger().Info(fmt.Sprintf("Property created successfully: %d", createdProperty.ID))
	return models.RecordId(createdProperty.ID), nil
}

func (rep *PropertyRepository) GetPropertyById(ctx context.Context, propertyId models.RecordId) (*models.PropertyDTO, error) {
	if !propertyId.IsValid() {
		return nil, server_error.New("INVALID_PROPERTY_ID", "property ID is invalid")
	}

	prop, err := rep.db.client.Property.Query().
		Where(property.IDEQ(int(propertyId))).
		Only(ctx)

	if err != nil {
		if client.IsNotFound(err) {
			return nil, server_error.New("PROPERTY_NOT_FOUND", "property not found")
		}
		return nil, server_error.Wrap("PROPERTY_QUERY", "failed to query property", err)
	}

	return rep.entToDTO(prop), nil
}

func (rep *PropertyRepository) UpdateProperty(ctx context.Context, tx *client.Tx, propertyId models.RecordId, propertyDTO *models.PropertyDTO) error {
	if !propertyId.IsValid() {
		return server_error.New("INVALID_PROPERTY_ID", "property ID is invalid")
	}

	rep.db.Logger().Debug(fmt.Sprintf("Updating property: %d", propertyId))

	builder := tx.Property.UpdateOneID(int(propertyId))

	if propertyDTO.Title != nil {
		builder.SetTitle(*propertyDTO.Title)
	}
	if propertyDTO.Description != nil {
		builder.SetDescription(*propertyDTO.Description)
	}
	if propertyDTO.PropertyType != nil {
		builder.SetPropertyType(property.PropertyType(*propertyDTO.PropertyType))
	}
	if propertyDTO.Price != nil {
		builder.SetPrice(*propertyDTO.Price)
	}
	if propertyDTO.Status != nil {
		builder.SetStatus(property.Status(*propertyDTO.Status))
	}
	if propertyDTO.IsPublished != nil {
		builder.SetIsPublished(*propertyDTO.IsPublished)
	}
	if propertyDTO.Address != nil {
		builder.SetAddress(*propertyDTO.Address)
	}
	if propertyDTO.District != nil {
		builder.SetDistrict(*propertyDTO.District)
	}
	if propertyDTO.Municipality != nil {
		builder.SetMunicipality(*propertyDTO.Municipality)
	}
	if propertyDTO.Parish != nil {
		builder.SetNillableParish(propertyDTO.Parish)
	}
	if propertyDTO.PostalCode != nil {
		builder.SetNillablePostalCode(propertyDTO.PostalCode)
	}
	if propertyDTO.Country != nil {
		builder.SetCountry(*propertyDTO.Country)
	}
	if propertyDTO.Latitude != nil {
		builder.SetNillableLatitude(propertyDTO.Latitude)
	}
	if propertyDTO.Longitude != nil {
		builder.SetNillableLongitude(propertyDTO.Longitude)
	}
	if propertyDTO.Bedrooms != nil {
		builder.SetNillableBedrooms(propertyDTO.Bedrooms)
	}
	if propertyDTO.Bathrooms != nil {
		builder.SetNillableBathrooms(propertyDTO.Bathrooms)
	}
	if propertyDTO.AreaSqm != nil {
		builder.SetNillableAreaSqm(propertyDTO.AreaSqm)
	}
	if propertyDTO.LandAreaSqm != nil {
		builder.SetNillableLandAreaSqm(propertyDTO.LandAreaSqm)
	}
	if propertyDTO.YearBuilt != nil {
		builder.SetNillableYearBuilt(propertyDTO.YearBuilt)
	}
	if propertyDTO.Floor != nil {
		builder.SetNillableFloor(propertyDTO.Floor)
	}
	if propertyDTO.TotalFloors != nil {
		builder.SetNillableTotalFloors(propertyDTO.TotalFloors)
	}
	if propertyDTO.ParkingSpaces != nil {
		builder.SetNillableParkingSpaces(propertyDTO.ParkingSpaces)
	}
	if propertyDTO.HasGarage != nil {
		builder.SetHasGarage(*propertyDTO.HasGarage)
	}
	if propertyDTO.HasGarden != nil {
		builder.SetHasGarden(*propertyDTO.HasGarden)
	}
	if propertyDTO.HasPool != nil {
		builder.SetHasPool(*propertyDTO.HasPool)
	}
	if propertyDTO.HasElevator != nil {
		builder.SetHasElevator(*propertyDTO.HasElevator)
	}
	if propertyDTO.EnergyRating != nil {
		builder.SetNillableEnergyRating((*property.EnergyRating)(propertyDTO.EnergyRating))
	}
	if propertyDTO.VirtualTourURL != nil {
		builder.SetNillableVirtualTourURL(propertyDTO.VirtualTourURL)
	}
	if propertyDTO.ContactID != nil {
		builder.SetContactID(int(*propertyDTO.ContactID))
	}

	err := builder.Exec(ctx)
	if err != nil {
		if client.IsNotFound(err) {
			return server_error.New("PROPERTY_NOT_FOUND", "property not found")
		}
		rep.db.Logger().Error(fmt.Sprintf("Failed to update property [%d]: %s", propertyId, err.Error()))
		return server_error.Wrap("PROPERTY_UPDATE", "failed to update property", err)
	}

	rep.db.Logger().Info(fmt.Sprintf("Property updated successfully: %d", propertyId))
	return nil
}

func (rep *PropertyRepository) DeleteProperty(ctx context.Context, tx *client.Tx, propertyId models.RecordId) error {
	if !propertyId.IsValid() {
		return server_error.New("INVALID_PROPERTY_ID", "property ID is invalid")
	}

	rep.db.Logger().Debug(fmt.Sprintf("Deleting property: %d", propertyId))

	err := tx.Property.DeleteOneID(int(propertyId)).Exec(ctx)
	if err != nil {
		if client.IsNotFound(err) {
			return server_error.New("PROPERTY_NOT_FOUND", "property not found")
		}
		rep.db.Logger().Error(fmt.Sprintf("Failed to delete property [%d]: %s", propertyId, err.Error()))
		return server_error.Wrap("PROPERTY_DELETE", "failed to delete property", err)
	}

	rep.db.Logger().Info(fmt.Sprintf("Property deleted successfully: %d", propertyId))
	return nil
}

func (rep *PropertyRepository) ListProperties(ctx context.Context, filters PropertyFilters) ([]models.PropertyDTO, int, error) {
	query := rep.db.client.Property.Query()

	if filters.District != nil {
		query = query.Where(property.DistrictEQ(*filters.District))
	}
	if filters.Municipality != nil {
		query = query.Where(property.MunicipalityEQ(*filters.Municipality))
	}
	if filters.Parish != nil {
		query = query.Where(property.ParishEQ(*filters.Parish))
	}
	if filters.PropertyType != nil {
		query = query.Where(property.PropertyTypeEQ(property.PropertyType(*filters.PropertyType)))
	}
	if filters.Status != nil {
		query = query.Where(property.StatusEQ(property.Status(*filters.Status)))
	}
	if filters.MinPrice != nil {
		query = query.Where(property.PriceGTE(*filters.MinPrice))
	}
	if filters.MaxPrice != nil {
		query = query.Where(property.PriceLTE(*filters.MaxPrice))
	}
	if filters.IsPublished != nil {
		query = query.Where(property.IsPublishedEQ(*filters.IsPublished))
	}
	if filters.PublisherID != nil {
		query = query.Where(property.PublisherIDEQ(int(*filters.PublisherID)))
	}

	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, server_error.Wrap("PROPERTY_COUNT", "failed to count properties", err)
	}

	if filters.Limit <= 0 {
		filters.Limit = 20
	}
	if filters.Offset < 0 {
		filters.Offset = 0
	}

	properties, err := query.
		Limit(filters.Limit).
		Offset(filters.Offset).
		Order(client.Desc(property.FieldCreatedAt)).
		All(ctx)

	if err != nil {
		return nil, 0, server_error.Wrap("PROPERTY_LIST", "failed to list properties", err)
	}

	dtos := make([]models.PropertyDTO, len(properties))
	for i, prop := range properties {
		dtos[i] = *rep.entToDTO(prop)
	}

	return dtos, total, nil
}

func (rep *PropertyRepository) PublishProperty(ctx context.Context, tx *client.Tx, propertyId models.RecordId) error {
	if !propertyId.IsValid() {
		return server_error.New("INVALID_PROPERTY_ID", "property ID is invalid")
	}

	now := time.Now()
	err := tx.Property.UpdateOneID(int(propertyId)).
		SetIsPublished(true).
		SetPublishedAt(now).
		Exec(ctx)

	if err != nil {
		if client.IsNotFound(err) {
			return server_error.New("PROPERTY_NOT_FOUND", "property not found")
		}
		return server_error.Wrap("PROPERTY_PUBLISH", "failed to publish property", err)
	}

	rep.db.Logger().Info(fmt.Sprintf("Property published: %d", propertyId))
	return nil
}

func (rep *PropertyRepository) UnpublishProperty(ctx context.Context, tx *client.Tx, propertyId models.RecordId) error {
	if !propertyId.IsValid() {
		return server_error.New("INVALID_PROPERTY_ID", "property ID is invalid")
	}

	err := tx.Property.UpdateOneID(int(propertyId)).
		SetIsPublished(false).
		Exec(ctx)

	if err != nil {
		if client.IsNotFound(err) {
			return server_error.New("PROPERTY_NOT_FOUND", "property not found")
		}
		return server_error.Wrap("PROPERTY_UNPUBLISH", "failed to unpublish property", err)
	}

	rep.db.Logger().Info(fmt.Sprintf("Property unpublished: %d", propertyId))
	return nil
}

func (rep *PropertyRepository) IncrementViewCount(ctx context.Context, propertyId models.RecordId) error {
	if !propertyId.IsValid() {
		return server_error.New("INVALID_PROPERTY_ID", "property ID is invalid")
	}

	prop, err := rep.db.client.Property.Get(ctx, int(propertyId))
	if err != nil {
		if client.IsNotFound(err) {
			return server_error.New("PROPERTY_NOT_FOUND", "property not found")
		}
		return server_error.Wrap("PROPERTY_QUERY", "failed to query property", err)
	}

	err = rep.db.client.Property.UpdateOneID(int(propertyId)).
		SetViewCount(prop.ViewCount + 1).
		Exec(ctx)

	if err != nil {
		return server_error.Wrap("PROPERTY_UPDATE_VIEWS", "failed to increment view count", err)
	}

	return nil
}

func (rep *PropertyRepository) validatePropertyInput(propertyDTO *models.PropertyDTO) error {
	if propertyDTO.Title == nil || *propertyDTO.Title == "" {
		return server_error.New("PROPERTY_VALIDATION", "title is required")
	}
	if propertyDTO.Description == nil || *propertyDTO.Description == "" {
		return server_error.New("PROPERTY_VALIDATION", "description is required")
	}
	if propertyDTO.PropertyType == nil || *propertyDTO.PropertyType == "" {
		return server_error.New("PROPERTY_VALIDATION", "property type is required")
	}
	if propertyDTO.Price == nil || *propertyDTO.Price <= 0 {
		return server_error.New("PROPERTY_VALIDATION", "price must be greater than zero")
	}
	if propertyDTO.Address == nil || *propertyDTO.Address == "" {
		return server_error.New("PROPERTY_VALIDATION", "address is required")
	}
	if propertyDTO.District == nil || *propertyDTO.District == "" {
		return server_error.New("PROPERTY_VALIDATION", "district is required")
	}
	if propertyDTO.Municipality == nil || *propertyDTO.Municipality == "" {
		return server_error.New("PROPERTY_VALIDATION", "municipality is required")
	}
	if propertyDTO.ContactID == nil || !propertyDTO.ContactID.IsValid() {
		return server_error.New("PROPERTY_VALIDATION", "contact ID is required")
	}
	if propertyDTO.PublisherID == nil || !propertyDTO.PublisherID.IsValid() {
		return server_error.New("PROPERTY_VALIDATION", "publisher ID is required")
	}

	// Validate location data
	validator, err := utils.GetLocationValidator()
	if err != nil {
		rep.db.Logger().Error(fmt.Sprintf("Failed to initialize location validator: %s", err.Error()))
		return server_error.New("VALIDATION_ERROR", "failed to validate location")
	}

	// Validate district (must be a valid district in Portugal)
	if !validator.ValidateDistrict(*propertyDTO.District) {
		return server_error.New("INVALID_DISTRICT", fmt.Sprintf("district '%s' is not a valid Portuguese district", *propertyDTO.District))
	}

	// Validate municipality (must be a valid municipality in Portugal)
	if !validator.ValidateMunicipality(*propertyDTO.Municipality) {
		return server_error.New("INVALID_MUNICIPALITY", fmt.Sprintf("municipality '%s' is not a valid Portuguese municipality", *propertyDTO.Municipality))
	}

	// Validate parish if provided (optional)
	if propertyDTO.Parish != nil && *propertyDTO.Parish != "" {
		if !validator.ValidateParish(*propertyDTO.Parish) {
			return server_error.New("INVALID_PARISH", fmt.Sprintf("parish '%s' is not a valid Portuguese parish", *propertyDTO.Parish))
		}
	}

	// Validate postal code format if provided
	if propertyDTO.PostalCode != nil && *propertyDTO.PostalCode != "" {
		if !validator.ValidatePostalCode(*propertyDTO.PostalCode) {
			return server_error.New("INVALID_POSTAL_CODE", "postal code must be in format XXXX-XXX")
		}
	}

	return nil
}

func (rep *PropertyRepository) entToDTO(prop *client.Property) *models.PropertyDTO {
	recordId := models.RecordId(prop.ID)
	publisherId := models.RecordId(prop.PublisherID)
	contactId := models.RecordId(prop.ContactID)

	propertyType := string(prop.PropertyType)
	status := string(prop.Status)
	country := prop.Country

	dto := &models.PropertyDTO{
		ID:           &recordId,
		Title:        &prop.Title,
		Description:  &prop.Description,
		PropertyType: &propertyType,
		Price:        &prop.Price,
		Status:       &status,
		IsPublished:  &prop.IsPublished,
		Address:      &prop.Address,
		District:     &prop.District,
		Municipality: &prop.Municipality,
		Country:      &country,
		ContactID:    &contactId,
		PublisherID:  &publisherId,
		ViewCount:    &prop.ViewCount,
		CreatedAt:    &prop.CreatedAt,
		UpdatedAt:    &prop.UpdatedAt,
	}

	if prop.Parish != nil {
		dto.Parish = prop.Parish
	}
	if prop.PostalCode != nil {
		dto.PostalCode = prop.PostalCode
	}
	if prop.Latitude != nil {
		dto.Latitude = prop.Latitude
	}
	if prop.Longitude != nil {
		dto.Longitude = prop.Longitude
	}
	if prop.Bedrooms != nil {
		dto.Bedrooms = prop.Bedrooms
	}
	if prop.Bathrooms != nil {
		dto.Bathrooms = prop.Bathrooms
	}
	if prop.AreaSqm != nil {
		dto.AreaSqm = prop.AreaSqm
	}
	if prop.LandAreaSqm != nil {
		dto.LandAreaSqm = prop.LandAreaSqm
	}
	if prop.YearBuilt != nil {
		dto.YearBuilt = prop.YearBuilt
	}
	if prop.Floor != nil {
		dto.Floor = prop.Floor
	}
	if prop.TotalFloors != nil {
		dto.TotalFloors = prop.TotalFloors
	}
	if prop.ParkingSpaces != nil {
		dto.ParkingSpaces = prop.ParkingSpaces
	}
	dto.HasGarage = &prop.HasGarage
	dto.HasGarden = &prop.HasGarden
	dto.HasPool = &prop.HasPool
	dto.HasElevator = &prop.HasElevator
	if prop.EnergyRating != nil {
		energyRating := string(*prop.EnergyRating)
		dto.EnergyRating = &energyRating
	}
	if prop.VirtualTourURL != nil {
		dto.VirtualTourURL = prop.VirtualTourURL
	}
	if prop.PublishedAt != nil {
		dto.PublishedAt = prop.PublishedAt
	}

	return dto
}
