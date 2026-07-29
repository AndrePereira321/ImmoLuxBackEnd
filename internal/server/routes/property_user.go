package routes

import (
	"strconv"

	"immo-lux/internal/database"
	"immo-lux/internal/models"

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
	Title          string           `json:"title"`
	Description    string           `json:"description"`
	PropertyType   string           `json:"propertyType"`
	Price          float64          `json:"price"`
	Status         string           `json:"status"`
	Address        string           `json:"address"`
	District       string           `json:"district"`
	Municipality   string           `json:"municipality"`
	Parish         *string          `json:"parish"`
	PostalCode     *string          `json:"postalCode"`
	Country        string           `json:"country"`
	Latitude       *float64         `json:"latitude"`
	Longitude      *float64         `json:"longitude"`
	Bedrooms       *int             `json:"bedrooms"`
	Bathrooms      *int             `json:"bathrooms"`
	AreaSqm        *float64         `json:"areaSqm"`
	LandAreaSqm    *float64         `json:"landAreaSqm"`
	YearBuilt      *int             `json:"yearBuilt"`
	Floor          *int             `json:"floor"`
	TotalFloors    *int             `json:"totalFloors"`
	ParkingSpaces  *int             `json:"parkingSpaces"`
	HasGarage      bool             `json:"hasGarage"`
	HasGarden      bool             `json:"hasGarden"`
	HasPool        bool             `json:"hasPool"`
	HasElevator    bool             `json:"hasElevator"`
	EnergyRating   *string          `json:"energyRating"`
	VirtualTourURL *string          `json:"virtualTourUrl"`
	Contacts       []ContactPayload `json:"contacts"`
}

type UpdatePropertyPayload struct {
	Title          *string          `json:"title"`
	Description    *string          `json:"description"`
	PropertyType   *string          `json:"propertyType"`
	Price          *float64         `json:"price"`
	Status         *string          `json:"status"`
	IsPublished    *bool            `json:"isPublished"`
	Address        *string          `json:"address"`
	District       *string          `json:"district"`
	Municipality   *string          `json:"municipality"`
	Parish         *string          `json:"parish"`
	PostalCode     *string          `json:"postalCode"`
	Country        *string          `json:"country"`
	Latitude       *float64         `json:"latitude"`
	Longitude      *float64         `json:"longitude"`
	Bedrooms       *int             `json:"bedrooms"`
	Bathrooms      *int             `json:"bathrooms"`
	AreaSqm        *float64         `json:"areaSqm"`
	LandAreaSqm    *float64         `json:"landAreaSqm"`
	YearBuilt      *int             `json:"yearBuilt"`
	Floor          *int             `json:"floor"`
	TotalFloors    *int             `json:"totalFloors"`
	ParkingSpaces  *int             `json:"parkingSpaces"`
	HasGarage      *bool            `json:"hasGarage"`
	HasGarden      *bool            `json:"hasGarden"`
	HasPool        *bool            `json:"hasPool"`
	HasElevator    *bool            `json:"hasElevator"`
	EnergyRating   *string          `json:"energyRating"`
	VirtualTourURL *string          `json:"virtualTourUrl"`
	Contacts       []ContactPayload `json:"contacts"`
}

func (p *ContactPayload) toDTO() models.ContactDTO {
	dto := models.ContactDTO{
		Name:  p.Name,
		Email: p.Email,
		Phone: p.Phone,
		Notes: p.Notes,
	}
	if p.ID != nil {
		id := models.RecordId(*p.ID)
		dto.ID = &id
	}
	return dto
}

func contactPayloadsToDTOs(payloads []ContactPayload) []models.ContactDTO {
	contacts := make([]models.ContactDTO, len(payloads))
	for i, payload := range payloads {
		contacts[i] = payload.toDTO()
	}
	return contacts
}

// existingContactIds keeps only the contacts referenced by ID: property updates
// link existing contacts and ignore inline new ones.
func existingContactIds(payloads []ContactPayload) []models.RecordId {
	var ids []models.RecordId
	for _, payload := range payloads {
		if payload.ID != nil {
			ids = append(ids, models.RecordId(*payload.ID))
		}
	}
	return ids
}

func (p *CreatePropertyPayload) toDTO() *models.PropertyDTO {
	return &models.PropertyDTO{
		Title:          &p.Title,
		Description:    &p.Description,
		PropertyType:   &p.PropertyType,
		Price:          &p.Price,
		Status:         &p.Status,
		Address:        &p.Address,
		District:       &p.District,
		Municipality:   &p.Municipality,
		Parish:         p.Parish,
		PostalCode:     p.PostalCode,
		Country:        &p.Country,
		Latitude:       p.Latitude,
		Longitude:      p.Longitude,
		Bedrooms:       p.Bedrooms,
		Bathrooms:      p.Bathrooms,
		AreaSqm:        p.AreaSqm,
		LandAreaSqm:    p.LandAreaSqm,
		YearBuilt:      p.YearBuilt,
		Floor:          p.Floor,
		TotalFloors:    p.TotalFloors,
		ParkingSpaces:  p.ParkingSpaces,
		HasGarage:      &p.HasGarage,
		HasGarden:      &p.HasGarden,
		HasPool:        &p.HasPool,
		HasElevator:    &p.HasElevator,
		EnergyRating:   p.EnergyRating,
		VirtualTourURL: p.VirtualTourURL,
	}
}

func (p *UpdatePropertyPayload) toDTO() *models.PropertyDTO {
	return &models.PropertyDTO{
		Title:          p.Title,
		Description:    p.Description,
		PropertyType:   p.PropertyType,
		Price:          p.Price,
		Status:         p.Status,
		IsPublished:    p.IsPublished,
		Address:        p.Address,
		District:       p.District,
		Municipality:   p.Municipality,
		Parish:         p.Parish,
		PostalCode:     p.PostalCode,
		Country:        p.Country,
		Latitude:       p.Latitude,
		Longitude:      p.Longitude,
		Bedrooms:       p.Bedrooms,
		Bathrooms:      p.Bathrooms,
		AreaSqm:        p.AreaSqm,
		LandAreaSqm:    p.LandAreaSqm,
		YearBuilt:      p.YearBuilt,
		Floor:          p.Floor,
		TotalFloors:    p.TotalFloors,
		ParkingSpaces:  p.ParkingSpaces,
		HasGarage:      p.HasGarage,
		HasGarden:      p.HasGarden,
		HasPool:        p.HasPool,
		HasElevator:    p.HasElevator,
		EnergyRating:   p.EnergyRating,
		VirtualTourURL: p.VirtualTourURL,
	}
}

func CreateProperty(ctx *RouteContext) error {
	payload := &CreatePropertyPayload{}
	if err := ctx.ReadBody(&payload); err != nil {
		return err
	}

	userId, err := ctx.RequireUserId()
	if err != nil {
		return err
	}

	created, err := ctx.Db().NewPropertyRepository().CreateProperty(
		ctx.RequestContext(), userId, payload.toDTO(), contactPayloadsToDTOs(payload.Contacts))
	if err != nil {
		return err
	}

	return ctx.RespondData(created)
}

func ListMyProperties(ctx *RouteContext) error {
	userId, err := ctx.RequireUserId()
	if err != nil {
		return err
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

	properties, total, err := ctx.Db().NewPropertyRepository().ListProperties(ctx.RequestContext(), filters)
	if err != nil {
		return err
	}

	return ctx.RespondData(&PropertyListResponse{
		Properties: properties,
		Total:      total,
	})
}

func UpdateProperty(ctx *RouteContext) error {
	propertyId, err := ctx.ParseIdParam("id")
	if err != nil {
		return err
	}

	userId, err := ctx.RequireUserId()
	if err != nil {
		return err
	}

	payload := &UpdatePropertyPayload{}
	if err := ctx.ReadBody(&payload); err != nil {
		return err
	}

	updated, err := ctx.Db().NewPropertyRepository().UpdateProperty(
		ctx.RequestContext(), userId, propertyId, payload.toDTO(), existingContactIds(payload.Contacts))
	if err != nil {
		return err
	}

	return ctx.RespondData(updated)
}

func DeleteProperty(ctx *RouteContext) error {
	propertyId, err := ctx.ParseIdParam("id")
	if err != nil {
		return err
	}

	userId, err := ctx.RequireUserId()
	if err != nil {
		return err
	}

	if err := ctx.Db().NewPropertyRepository().DeleteProperty(ctx.RequestContext(), userId, propertyId); err != nil {
		return err
	}

	return ctx.RespondData(&fiber.Map{
		"success": true,
		"message": "Property deleted successfully",
	})
}

func PublishProperty(ctx *RouteContext) error {
	propertyId, err := ctx.ParseIdParam("id")
	if err != nil {
		return err
	}

	userId, err := ctx.RequireUserId()
	if err != nil {
		return err
	}

	if err := ctx.Db().NewPropertyRepository().PublishProperty(ctx.RequestContext(), userId, propertyId); err != nil {
		return err
	}

	return ctx.RespondData(&fiber.Map{
		"success": true,
		"message": "Property published successfully",
	})
}

func UnpublishProperty(ctx *RouteContext) error {
	propertyId, err := ctx.ParseIdParam("id")
	if err != nil {
		return err
	}

	userId, err := ctx.RequireUserId()
	if err != nil {
		return err
	}

	if err := ctx.Db().NewPropertyRepository().UnpublishProperty(ctx.RequestContext(), userId, propertyId); err != nil {
		return err
	}

	return ctx.RespondData(&fiber.Map{
		"success": true,
		"message": "Property unpublished successfully",
	})
}
