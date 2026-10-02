package database

import (
	"context"
	"fmt"
	"sort"
	"strings"
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
	Query        *string // Free text, matched against the title, description, address and place names
	IsPublished  *bool
	PublisherID  *models.RecordId
	Limit        int
	Offset       int
	OrderBy      *string // Options: "price_asc", "price_desc", "created_asc", "created_desc", "popularity", "location", "status"
}

// Dimension names accepted by applyFilters' skip list.
const (
	dimDistrict     = "district"
	dimMunicipality = "municipality"
	dimParish       = "parish"
	dimPropertyType = "propertyType"
	dimStatus       = "status"
	dimPrice        = "price"
)

// applyFilters narrows a query by every active filter except the dimensions named
// in skip. Listing passes no skips; faceting skips the dimension it is counting,
// so that each option reports what choosing it would actually yield.
func (rep *PropertyRepository) applyFilters(query *client.PropertyQuery, filters PropertyFilters, skip ...string) *client.PropertyQuery {
	skipped := make(map[string]bool, len(skip))
	for _, dimension := range skip {
		skipped[dimension] = true
	}

	if filters.District != nil && !skipped[dimDistrict] {
		query = query.Where(property.DistrictEQ(*filters.District))
	}
	if filters.Municipality != nil && !skipped[dimMunicipality] {
		query = query.Where(property.MunicipalityEQ(*filters.Municipality))
	}
	if filters.Parish != nil && !skipped[dimParish] {
		query = query.Where(property.ParishEQ(*filters.Parish))
	}
	if filters.PropertyType != nil && !skipped[dimPropertyType] {
		query = query.Where(property.PropertyTypeEQ(property.PropertyType(*filters.PropertyType)))
	}
	if filters.Status != nil && !skipped[dimStatus] {
		query = query.Where(property.StatusEQ(property.Status(*filters.Status)))
	}
	if filters.MinPrice != nil && !skipped[dimPrice] {
		query = query.Where(property.PriceGTE(*filters.MinPrice))
	}
	if filters.MaxPrice != nil && !skipped[dimPrice] {
		query = query.Where(property.PriceLTE(*filters.MaxPrice))
	}
	if filters.Query != nil {
		// Somebody typing "lousada" means the town, and typing "moradia" means the
		// word in the title: one box has to reach both, so it reaches every text
		// column the property owns.
		term := strings.TrimSpace(*filters.Query)
		if term != "" {
			query = query.Where(property.Or(
				property.TitleContainsFold(term),
				property.DescriptionContainsFold(term),
				property.AddressContainsFold(term),
				property.DistrictContainsFold(term),
				property.MunicipalityContainsFold(term),
				property.ParishContainsFold(term),
			))
		}
	}
	if filters.IsPublished != nil {
		query = query.Where(property.IsPublishedEQ(*filters.IsPublished))
	}
	if filters.PublisherID != nil {
		query = query.Where(property.PublisherIDEQ(int(*filters.PublisherID)))
	}

	return query
}

// ensureOwned verifies the property exists and belongs to ownerId, translating
// the outcome into not-found / forbidden kinds. It takes no lock (the verdict
// holds because publisher_id never changes); use lockOwned when concurrent
// writers on the same property must be serialised.
func (rep *PropertyRepository) ensureOwned(ctx context.Context, tx *client.Tx, ownerId, propertyId models.RecordId) error {
	if !propertyId.IsValid() {
		return server_error.Invalid("INVALID_PROPERTY_ID", "property ID is invalid")
	}

	owned, err := tx.Property.Query().
		Where(property.IDEQ(int(propertyId)), property.PublisherIDEQ(int(ownerId))).
		Exist(ctx)
	if err != nil {
		return server_error.Wrap("PROPERTY_QUERY", "failed to check property ownership", err)
	}
	if owned {
		return nil
	}

	return rep.ownershipVerdict(ctx, func(c context.Context) (bool, error) {
		return tx.Property.Query().Where(property.IDEQ(int(propertyId))).Exist(c)
	})
}

// lockOwned is ensureOwned for writes that must be serialised per property: the
// owner-predicated update (touching updated_at) holds the property's row lock
// until tx ends, so concurrent transactions on the same property queue here.
func (rep *PropertyRepository) lockOwned(ctx context.Context, tx *client.Tx, ownerId, propertyId models.RecordId) error {
	if !propertyId.IsValid() {
		return server_error.Invalid("INVALID_PROPERTY_ID", "property ID is invalid")
	}

	affected, err := tx.Property.Update().
		Where(property.IDEQ(int(propertyId)), property.PublisherIDEQ(int(ownerId))).
		SetUpdatedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return server_error.Wrap("PROPERTY_UPDATE", "failed to lock property", err)
	}
	if affected > 0 {
		return nil
	}

	return rep.ownershipVerdict(ctx, func(c context.Context) (bool, error) {
		return tx.Property.Query().Where(property.IDEQ(int(propertyId))).Exist(c)
	})
}

// ownershipVerdict wraps the shared classifier with the property store's codes.
func (rep *PropertyRepository) ownershipVerdict(ctx context.Context, exists func(context.Context) (bool, error)) error {
	return ownershipVerdict(ctx, exists,
		"PROPERTY_QUERY",
		server_error.NotFound("PROPERTY_NOT_FOUND", "property not found"),
		server_error.Forbidden("ACCESS_DENIED", "you can only modify your own properties"),
	)
}

// resolveContacts turns the caller's mixed list (existing IDs and new entries)
// into contact IDs, verifying every existing contact belongs to ownerId and
// creating the new ones under ownerId.
func (rep *PropertyRepository) resolveContacts(ctx context.Context, tx *client.Tx, ownerId models.RecordId, contacts []models.ContactDTO) ([]models.RecordId, error) {
	contactRepo := NewContactRepository(rep.db)
	contactIDs := make([]models.RecordId, 0, len(contacts))
	var existingIDs []models.RecordId
	for _, contactDTO := range contacts {
		if contactDTO.ID != nil {
			existingIDs = append(existingIDs, *contactDTO.ID)
			contactIDs = append(contactIDs, *contactDTO.ID)
			continue
		}

		created, err := contactRepo.createContact(ctx, tx, ownerId, &contactDTO)
		if err != nil {
			return nil, err
		}
		contactIDs = append(contactIDs, models.RecordId(created.ID))
	}

	if err := contactRepo.ensureAllOwned(ctx, tx, ownerId, existingIDs); err != nil {
		return nil, err
	}
	return contactIDs, nil
}

// CreateProperty runs the whole property-intake operation for ownerId: contacts
// are verified or created, the property is inserted unpublished, and the fresh
// record is read back.
func (rep *PropertyRepository) CreateProperty(ctx context.Context, ownerId models.RecordId, propertyDTO *models.PropertyDTO, contacts []models.ContactDTO) (*models.PropertyDTO, error) {
	var propertyId models.RecordId
	err := rep.db.WithTransaction(ctx, func(txCtx context.Context, tx *client.Tx) error {
		contactIDs, err := rep.resolveContacts(txCtx, tx, ownerId, contacts)
		if err != nil {
			return err
		}
		propertyDTO.ContactIDs = contactIDs

		if err := rep.validatePropertyInput(propertyDTO); err != nil {
			return err
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
			SetPublisherID(int(ownerId)).
			SetNillableParish(propertyDTO.Parish).
			SetNillablePostalCode(propertyDTO.PostalCode).
			SetNillableLatitude(propertyDTO.Latitude).
			SetNillableLongitude(propertyDTO.Longitude).
			SetNillableBedrooms(propertyDTO.Bedrooms).
			SetNillableBathrooms(propertyDTO.Bathrooms).
			SetNillableAreaSqm(propertyDTO.AreaSqm).
			SetNillableLandAreaSqm(propertyDTO.LandAreaSqm).
			SetNillableYearBuilt(propertyDTO.YearBuilt).
			SetNillableFloor(propertyDTO.Floor).
			SetNillableTotalFloors(propertyDTO.TotalFloors).
			SetNillableParkingSpaces(propertyDTO.ParkingSpaces).
			SetNillableHasGarage(propertyDTO.HasGarage).
			SetNillableHasGarden(propertyDTO.HasGarden).
			SetNillableHasPool(propertyDTO.HasPool).
			SetNillableHasElevator(propertyDTO.HasElevator).
			SetNillableEnergyRating((*property.EnergyRating)(propertyDTO.EnergyRating)).
			SetNillableVirtualTourURL(propertyDTO.VirtualTourURL)

		if len(propertyDTO.ContactIDs) > 0 {
			contactIDs := make([]int, len(propertyDTO.ContactIDs))
			for i, contactID := range propertyDTO.ContactIDs {
				contactIDs[i] = int(contactID)
			}
			builder.AddContactIDs(contactIDs...)
		}

		createdProperty, err := builder.Save(txCtx)
		if err != nil {
			rep.db.Logger().Error(fmt.Sprintf("Failed to create property [%s]: %s", *propertyDTO.Title, err.Error()))
			return server_error.Wrap("PROPERTY_INSERT", "failed to insert property", err)
		}

		propertyId = models.RecordId(createdProperty.ID)
		return nil
	})
	if err != nil {
		return nil, err
	}

	rep.db.Logger().Info(fmt.Sprintf("Property created successfully: %d", propertyId))
	return rep.GetPropertyById(ctx, propertyId)
}

func (rep *PropertyRepository) GetPropertyById(ctx context.Context, propertyId models.RecordId) (*models.PropertyDTO, error) {
	if !propertyId.IsValid() {
		return nil, server_error.Invalid("INVALID_PROPERTY_ID", "property ID is invalid")
	}

	prop, err := rep.db.client.Property.Query().
		Where(property.IDEQ(int(propertyId))).
		WithContacts().
		Only(ctx)

	if err != nil {
		if client.IsNotFound(err) {
			return nil, server_error.NotFound("PROPERTY_NOT_FOUND", "property not found")
		}
		return nil, server_error.Wrap("PROPERTY_QUERY", "failed to query property", err)
	}

	return rep.entToDTO(prop), nil
}

// UpdateProperty applies the given field changes to a property ownerId owns and,
// when contactIDs is non-empty, replaces the linked contacts after verifying
// each one belongs to ownerId. Returns the updated record.
func (rep *PropertyRepository) UpdateProperty(ctx context.Context, ownerId, propertyId models.RecordId, propertyDTO *models.PropertyDTO, contactIDs []models.RecordId) (*models.PropertyDTO, error) {
	err := rep.db.WithTransaction(ctx, func(txCtx context.Context, tx *client.Tx) error {
		if err := rep.ensureOwned(txCtx, tx, ownerId, propertyId); err != nil {
			return err
		}

		if err := NewContactRepository(rep.db).ensureAllOwned(txCtx, tx, ownerId, contactIDs); err != nil {
			return err
		}

		rep.db.Logger().Debug(fmt.Sprintf("Updating property: %d", propertyId))

		builder := tx.Property.UpdateOneID(int(propertyId)).
			SetNillableTitle(propertyDTO.Title).
			SetNillableDescription(propertyDTO.Description).
			SetNillablePropertyType((*property.PropertyType)(propertyDTO.PropertyType)).
			SetNillablePrice(propertyDTO.Price).
			SetNillableStatus((*property.Status)(propertyDTO.Status)).
			SetNillableIsPublished(propertyDTO.IsPublished).
			SetNillableAddress(propertyDTO.Address).
			SetNillableDistrict(propertyDTO.District).
			SetNillableMunicipality(propertyDTO.Municipality).
			SetNillableParish(propertyDTO.Parish).
			SetNillablePostalCode(propertyDTO.PostalCode).
			SetNillableCountry(propertyDTO.Country).
			SetNillableLatitude(propertyDTO.Latitude).
			SetNillableLongitude(propertyDTO.Longitude).
			SetNillableBedrooms(propertyDTO.Bedrooms).
			SetNillableBathrooms(propertyDTO.Bathrooms).
			SetNillableAreaSqm(propertyDTO.AreaSqm).
			SetNillableLandAreaSqm(propertyDTO.LandAreaSqm).
			SetNillableYearBuilt(propertyDTO.YearBuilt).
			SetNillableFloor(propertyDTO.Floor).
			SetNillableTotalFloors(propertyDTO.TotalFloors).
			SetNillableParkingSpaces(propertyDTO.ParkingSpaces).
			SetNillableHasGarage(propertyDTO.HasGarage).
			SetNillableHasGarden(propertyDTO.HasGarden).
			SetNillableHasPool(propertyDTO.HasPool).
			SetNillableHasElevator(propertyDTO.HasElevator).
			SetNillableEnergyRating((*property.EnergyRating)(propertyDTO.EnergyRating)).
			SetNillableVirtualTourURL(propertyDTO.VirtualTourURL)

		if err := builder.Exec(txCtx); err != nil {
			rep.db.Logger().Error(fmt.Sprintf("Failed to update property [%d]: %s", propertyId, err.Error()))
			return server_error.Wrap("PROPERTY_UPDATE", "failed to update property", err)
		}

		if len(contactIDs) > 0 {
			if err := rep.updateContacts(txCtx, tx, propertyId, contactIDs); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	rep.db.Logger().Info(fmt.Sprintf("Property updated successfully: %d", propertyId))
	return rep.GetPropertyById(ctx, propertyId)
}

// DeleteProperty removes a property ownerId owns together with its images.
func (rep *PropertyRepository) DeleteProperty(ctx context.Context, ownerId, propertyId models.RecordId) error {
	return rep.db.WithTransaction(ctx, func(txCtx context.Context, tx *client.Tx) error {
		// Locking (not just checking) the property makes a concurrent image
		// upload either finish first, so its image is deleted below, or wait and
		// then find the property gone, instead of breaking the final delete on
		// the image foreign key.
		if err := rep.lockOwned(txCtx, tx, ownerId, propertyId); err != nil {
			return err
		}

		rep.db.Logger().Debug(fmt.Sprintf("Deleting property: %d", propertyId))

		if err := NewPropertyImageRepository(rep.db).deleteImagesByPropertyId(txCtx, tx, propertyId); err != nil {
			return err
		}

		if err := tx.Property.DeleteOneID(int(propertyId)).Exec(txCtx); err != nil {
			rep.db.Logger().Error(fmt.Sprintf("Failed to delete property [%d]: %s", propertyId, err.Error()))
			return server_error.Wrap("PROPERTY_DELETE", "failed to delete property", err)
		}

		rep.db.Logger().Info(fmt.Sprintf("Property deleted successfully: %d", propertyId))
		return nil
	})
}

func (rep *PropertyRepository) ListProperties(ctx context.Context, filters PropertyFilters) ([]models.PropertyDTO, int, error) {
	query := rep.applyFilters(rep.db.client.Property.Query(), filters)

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

	orderBy := "created_desc"
	if filters.PublisherID != nil {
		orderBy = "status"
	}
	if filters.OrderBy != nil {
		orderBy = *filters.OrderBy
	}

	if orderBy == "status" {
		allProperties, err := query.WithContacts().All(ctx)
		if err != nil {
			return nil, 0, server_error.Wrap("PROPERTY_LIST", "failed to list properties", err)
		}

		sort.Slice(allProperties, func(i, j int) bool {
			statusPriority := func(s property.Status) int {
				switch s {
				case property.StatusAvailable:
					return 1
				case property.StatusPending:
					return 2
				case property.StatusSold:
					return 3
				case property.StatusRented:
					return 4
				default:
					return 5
				}
			}

			priorityI := statusPriority(allProperties[i].Status)
			priorityJ := statusPriority(allProperties[j].Status)

			if priorityI != priorityJ {
				return priorityI < priorityJ
			}

			return allProperties[i].CreatedAt.After(allProperties[j].CreatedAt)
		})

		start := filters.Offset
		end := filters.Offset + filters.Limit
		if start > len(allProperties) {
			start = len(allProperties)
		}
		if end > len(allProperties) {
			end = len(allProperties)
		}

		pagedProperties := allProperties[start:end]
		dtos := make([]models.PropertyDTO, len(pagedProperties))
		for i, prop := range pagedProperties {
			dtos[i] = *rep.entToDTO(prop)
		}

		return dtos, total, nil
	}

	switch orderBy {
	case "price_asc":
		query = query.Order(client.Asc(property.FieldPrice))
	case "price_desc":
		query = query.Order(client.Desc(property.FieldPrice))
	case "created_asc":
		query = query.Order(client.Asc(property.FieldCreatedAt))
	case "created_desc":
		query = query.Order(client.Desc(property.FieldCreatedAt))
	case "popularity":
		query = query.Order(client.Desc(property.FieldViewCount), client.Desc(property.FieldCreatedAt))
	case "location":
		query = query.Order(client.Asc(property.FieldDistrict), client.Asc(property.FieldMunicipality), client.Asc(property.FieldParish))
	default:
		// Default to newest first
		query = query.Order(client.Desc(property.FieldCreatedAt))
	}

	properties, err := query.
		WithContacts().
		Limit(filters.Limit).
		Offset(filters.Offset).
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

// GetPropertyFacets counts, per dimension, what each remaining option would yield
// under the current filters. It is what lets the search offer only choices that
// lead somewhere: an option absent from these buckets has nothing behind it.
func (rep *PropertyRepository) GetPropertyFacets(ctx context.Context, filters PropertyFilters) (*models.PropertyFacetsDTO, error) {
	facets := &models.PropertyFacetsDTO{
		Districts:      []models.FacetBucketDTO{},
		Municipalities: []models.FacetBucketDTO{},
		Parishes:       []models.FacetBucketDTO{},
		PropertyTypes:  []models.FacetBucketDTO{},
		Statuses:       []models.FacetBucketDTO{},
	}

	total, err := rep.applyFilters(rep.db.client.Property.Query(), filters).Count(ctx)
	if err != nil {
		return nil, server_error.Wrap("PROPERTY_FACETS", "failed to count matching properties", err)
	}
	facets.Total = total

	// Each dimension scans into its own struct: Ent matches result columns to struct
	// fields by name, so the column being grouped has to be named at compile time.

	// A municipality only exists inside its district, so counting districts has to
	// let go of the narrower location filters too — otherwise picking Lousada would
	// report every other district as empty, which is true only of that selection.
	var districts []struct {
		District string `json:"district"`
		Count    int    `json:"count"`
	}
	if err := rep.applyFilters(rep.db.client.Property.Query(), filters, dimDistrict, dimMunicipality, dimParish).
		GroupBy(property.FieldDistrict).Aggregate(client.Count()).Scan(ctx, &districts); err != nil {
		return nil, server_error.Wrap("PROPERTY_FACETS", "failed to count properties by district", err)
	}
	for _, row := range districts {
		facets.Districts = append(facets.Districts, models.FacetBucketDTO{Value: row.District, Count: row.Count})
	}

	var municipalities []struct {
		Municipality string `json:"municipality"`
		District     string `json:"district"`
		Count        int    `json:"count"`
	}
	if err := rep.applyFilters(rep.db.client.Property.Query(), filters, dimMunicipality, dimParish).
		GroupBy(property.FieldMunicipality, property.FieldDistrict).Aggregate(client.Count()).Scan(ctx, &municipalities); err != nil {
		return nil, server_error.Wrap("PROPERTY_FACETS", "failed to count properties by municipality", err)
	}
	for _, row := range municipalities {
		facets.Municipalities = append(facets.Municipalities,
			models.FacetBucketDTO{Value: row.Municipality, Parent: row.District, Count: row.Count})
	}

	// Parish is optional on a property. Grouping without excluding the unset ones
	// yields a NULL bucket, which is neither scannable nor a choice anyone can make.
	var parishes []struct {
		Parish string `json:"parish"`
		Count  int    `json:"count"`
	}
	if err := rep.applyFilters(rep.db.client.Property.Query(), filters, dimParish).
		Where(property.ParishNotNil(), property.ParishNEQ("")).
		GroupBy(property.FieldParish).Aggregate(client.Count()).Scan(ctx, &parishes); err != nil {
		return nil, server_error.Wrap("PROPERTY_FACETS", "failed to count properties by parish", err)
	}
	for _, row := range parishes {
		facets.Parishes = append(facets.Parishes, models.FacetBucketDTO{Value: row.Parish, Count: row.Count})
	}

	var propertyTypes []struct {
		PropertyType string `json:"property_type"`
		Count        int    `json:"count"`
	}
	if err := rep.applyFilters(rep.db.client.Property.Query(), filters, dimPropertyType).
		GroupBy(property.FieldPropertyType).Aggregate(client.Count()).Scan(ctx, &propertyTypes); err != nil {
		return nil, server_error.Wrap("PROPERTY_FACETS", "failed to count properties by type", err)
	}
	for _, row := range propertyTypes {
		facets.PropertyTypes = append(facets.PropertyTypes, models.FacetBucketDTO{Value: row.PropertyType, Count: row.Count})
	}

	var statuses []struct {
		Status string `json:"status"`
		Count  int    `json:"count"`
	}
	if err := rep.applyFilters(rep.db.client.Property.Query(), filters, dimStatus).
		GroupBy(property.FieldStatus).Aggregate(client.Count()).Scan(ctx, &statuses); err != nil {
		return nil, server_error.Wrap("PROPERTY_FACETS", "failed to count properties by status", err)
	}
	for _, row := range statuses {
		facets.Statuses = append(facets.Statuses, models.FacetBucketDTO{Value: row.Status, Count: row.Count})
	}

	sortBuckets(facets.Districts)
	sortBuckets(facets.Municipalities)
	sortBuckets(facets.Parishes)
	sortBuckets(facets.PropertyTypes)
	sortBuckets(facets.Statuses)

	// Bounds ignore the price filter itself, so the range shown stays the range on
	// offer rather than collapsing onto whatever the buyer last typed.
	var bounds []struct {
		Min *float64 `json:"min"`
		Max *float64 `json:"max"`
	}
	err = rep.applyFilters(rep.db.client.Property.Query(), filters, dimPrice).
		Aggregate(client.Min(property.FieldPrice), client.Max(property.FieldPrice)).
		Scan(ctx, &bounds)
	if err != nil {
		return nil, server_error.Wrap("PROPERTY_FACETS", "failed to compute price bounds", err)
	}
	if len(bounds) > 0 {
		facets.MinPrice = bounds[0].Min
		facets.MaxPrice = bounds[0].Max
	}

	return facets, nil
}

// sortBuckets puts the fullest option first, so the rail reads as an inventory
// ordered by what there is most of, with ties settled alphabetically for stability.
func sortBuckets(buckets []models.FacetBucketDTO) {
	sort.Slice(buckets, func(i, j int) bool {
		if buckets[i].Count != buckets[j].Count {
			return buckets[i].Count > buckets[j].Count
		}
		return buckets[i].Value < buckets[j].Value
	})
}

// PublishProperty marks a property ownerId owns as published, with one
// owner-predicated statement; the probe runs only when nothing matched.
func (rep *PropertyRepository) PublishProperty(ctx context.Context, ownerId, propertyId models.RecordId) error {
	if !propertyId.IsValid() {
		return server_error.Invalid("INVALID_PROPERTY_ID", "property ID is invalid")
	}

	updated, err := rep.db.client.Property.Update().
		Where(property.IDEQ(int(propertyId)), property.PublisherIDEQ(int(ownerId))).
		SetIsPublished(true).
		SetPublishedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return server_error.Wrap("PROPERTY_PUBLISH", "failed to publish property", err)
	}
	if updated == 0 {
		return rep.ownershipVerdict(ctx, func(c context.Context) (bool, error) {
			return rep.db.client.Property.Query().Where(property.IDEQ(int(propertyId))).Exist(c)
		})
	}

	rep.db.Logger().Info(fmt.Sprintf("Property published: %d", propertyId))
	return nil
}

// UnpublishProperty takes a property ownerId owns off the public listing, with
// one owner-predicated statement; the probe runs only when nothing matched.
func (rep *PropertyRepository) UnpublishProperty(ctx context.Context, ownerId, propertyId models.RecordId) error {
	if !propertyId.IsValid() {
		return server_error.Invalid("INVALID_PROPERTY_ID", "property ID is invalid")
	}

	updated, err := rep.db.client.Property.Update().
		Where(property.IDEQ(int(propertyId)), property.PublisherIDEQ(int(ownerId))).
		SetIsPublished(false).
		Save(ctx)
	if err != nil {
		return server_error.Wrap("PROPERTY_UNPUBLISH", "failed to unpublish property", err)
	}
	if updated == 0 {
		return rep.ownershipVerdict(ctx, func(c context.Context) (bool, error) {
			return rep.db.client.Property.Query().Where(property.IDEQ(int(propertyId))).Exist(c)
		})
	}

	rep.db.Logger().Info(fmt.Sprintf("Property unpublished: %d", propertyId))
	return nil
}

func (rep *PropertyRepository) updateContacts(ctx context.Context, tx *client.Tx, propertyId models.RecordId, contactIDs []models.RecordId) error {
	intContactIDs := make([]int, len(contactIDs))
	for i, contactID := range contactIDs {
		intContactIDs[i] = int(contactID)
	}

	err := tx.Property.UpdateOneID(int(propertyId)).
		ClearContacts().
		AddContactIDs(intContactIDs...).
		Exec(ctx)
	if err != nil {
		rep.db.Logger().Error(fmt.Sprintf("Failed to update property contacts [%d]: %s", propertyId, err.Error()))
		return server_error.Wrap("PROPERTY_UPDATE_CONTACTS", "failed to update property contacts", err)
	}

	return nil
}

// ViewProperty returns a public property and counts the view; the count is
// best-effort and never fails the read.
func (rep *PropertyRepository) ViewProperty(ctx context.Context, propertyId models.RecordId) (*models.PropertyDTO, error) {
	propertyDTO, err := rep.GetPropertyById(ctx, propertyId)
	if err != nil {
		return nil, err
	}

	if err := rep.incrementViewCount(ctx, propertyId); err != nil {
		rep.db.Logger().Warn(fmt.Sprintf("Failed to increment view count for property %d: %s", propertyId, err.Error()))
	}

	return propertyDTO, nil
}

func (rep *PropertyRepository) incrementViewCount(ctx context.Context, propertyId models.RecordId) error {
	_, err := rep.db.client.Property.Update().
		Where(property.IDEQ(int(propertyId))).
		AddViewCount(1).
		Save(ctx)
	if err != nil {
		return server_error.Wrap("PROPERTY_UPDATE_VIEWS", "failed to increment view count", err)
	}
	return nil
}

func (rep *PropertyRepository) validatePropertyInput(propertyDTO *models.PropertyDTO) error {
	if propertyDTO.Title == nil || *propertyDTO.Title == "" {
		return server_error.Invalid("PROPERTY_VALIDATION", "title is required")
	}
	if propertyDTO.Description == nil || *propertyDTO.Description == "" {
		return server_error.Invalid("PROPERTY_VALIDATION", "description is required")
	}
	if propertyDTO.PropertyType == nil || *propertyDTO.PropertyType == "" {
		return server_error.Invalid("PROPERTY_VALIDATION", "property type is required")
	}
	if propertyDTO.Price == nil || *propertyDTO.Price <= 0 {
		return server_error.Invalid("PROPERTY_VALIDATION", "price must be greater than zero")
	}
	if propertyDTO.Address == nil || *propertyDTO.Address == "" {
		return server_error.Invalid("PROPERTY_VALIDATION", "address is required")
	}
	if propertyDTO.District == nil || *propertyDTO.District == "" {
		return server_error.Invalid("PROPERTY_VALIDATION", "district is required")
	}
	if propertyDTO.Municipality == nil || *propertyDTO.Municipality == "" {
		return server_error.Invalid("PROPERTY_VALIDATION", "municipality is required")
	}
	if len(propertyDTO.ContactIDs) == 0 {
		return server_error.Invalid("PROPERTY_VALIDATION", "at least one contact is required")
	}
	for _, contactID := range propertyDTO.ContactIDs {
		if !contactID.IsValid() {
			return server_error.Invalid("PROPERTY_VALIDATION", "invalid contact ID provided")
		}
	}

	validator, err := utils.GetLocationValidator()
	if err != nil {
		rep.db.Logger().Error(fmt.Sprintf("Failed to initialize location validator: %s", err.Error()))
		return server_error.Invalid("VALIDATION_ERROR", "failed to validate location")
	}

	if !validator.ValidateDistrict(*propertyDTO.District) {
		return server_error.Invalid("INVALID_DISTRICT", fmt.Sprintf("district '%s' is not a valid Portuguese district", *propertyDTO.District))
	}

	if !validator.ValidateMunicipality(*propertyDTO.Municipality) {
		return server_error.Invalid("INVALID_MUNICIPALITY", fmt.Sprintf("municipality '%s' is not a valid Portuguese municipality", *propertyDTO.Municipality))
	}

	if propertyDTO.Parish != nil && *propertyDTO.Parish != "" {
		if !validator.ValidateParish(*propertyDTO.Parish) {
			return server_error.Invalid("INVALID_PARISH", fmt.Sprintf("parish '%s' is not a valid Portuguese parish", *propertyDTO.Parish))
		}
	}

	if propertyDTO.PostalCode != nil && *propertyDTO.PostalCode != "" {
		if !validator.ValidatePostalCode(*propertyDTO.PostalCode) {
			return server_error.Invalid("INVALID_POSTAL_CODE", "postal code must be in format XXXX-XXX")
		}
	}

	return nil
}

func (rep *PropertyRepository) entToDTO(prop *client.Property) *models.PropertyDTO {
	recordId := models.RecordId(prop.ID)
	publisherId := models.RecordId(prop.PublisherID)

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
		PublisherID:  &publisherId,
		ViewCount:    &prop.ViewCount,
		CreatedAt:    &prop.CreatedAt,
		UpdatedAt:    &prop.UpdatedAt,
	}

	if len(prop.Edges.Contacts) > 0 {
		contactIDs := make([]models.RecordId, len(prop.Edges.Contacts))
		contacts := make([]models.ContactDTO, len(prop.Edges.Contacts))

		contactRepo := NewContactRepository(rep.db)
		for i, c := range prop.Edges.Contacts {
			contactIDs[i] = models.RecordId(c.ID)
			contacts[i] = *contactRepo.entToDTO(c)
		}

		dto.ContactIDs = contactIDs
		dto.Contacts = contacts
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

// GetLocationStats returns aggregate stats for all published properties in a district
// (and optionally a municipality). district must be the canonical DB value (e.g. "Lisboa").
func (rep *PropertyRepository) GetLocationStats(ctx context.Context, district string, municipality *string) (*models.LocationStatsDTO, error) {
	query := rep.db.client.Property.Query().
		Where(
			property.IsPublishedEQ(true),
			property.DistrictEQ(district),
		)
	if municipality != nil {
		query = query.Where(property.MunicipalityEQ(*municipality))
	}

	total, err := query.Count(ctx)
	if err != nil {
		return nil, server_error.Wrap("LOCATION_STATS", "failed to count properties", err)
	}
	if total == 0 {
		dto := &models.LocationStatsDTO{District: district, Municipality: municipality, Total: 0}
		return dto, nil
	}

	props, err := query.All(ctx)
	if err != nil {
		return nil, server_error.Wrap("LOCATION_STATS", "failed to fetch properties for stats", err)
	}

	minPrice := props[0].Price
	maxPrice := props[0].Price
	var sumPrice float64
	typeCounts := make(map[string]int)

	for _, p := range props {
		if p.Price < minPrice {
			minPrice = p.Price
		}
		if p.Price > maxPrice {
			maxPrice = p.Price
		}
		sumPrice += p.Price
		typeCounts[string(p.PropertyType)]++
	}

	mostCommonType := "house"
	maxCount := 0
	for t, count := range typeCounts {
		if count > maxCount {
			maxCount = count
			mostCommonType = t
		}
	}

	return &models.LocationStatsDTO{
		District:       district,
		Municipality:   municipality,
		Total:          total,
		MinPrice:       minPrice,
		MaxPrice:       maxPrice,
		AvgPrice:       sumPrice / float64(total),
		MostCommonType: mostCommonType,
	}, nil
}

// GetPublishedLocations returns all districts and municipalities that have at least
// one published property, with the most recent updatedAt per location.
func (rep *PropertyRepository) GetPublishedLocations(ctx context.Context) (*models.PublishedLocationsDTO, error) {
	props, err := rep.db.client.Property.Query().
		Where(property.IsPublishedEQ(true)).
		All(ctx)
	if err != nil {
		return nil, server_error.Wrap("PUBLISHED_LOCATIONS", "failed to fetch published properties", err)
	}

	districtMap := make(map[string]time.Time)
	municipalityMap := make(map[string]time.Time) // key = "District|Municipality"

	for _, p := range props {
		if t, ok := districtMap[p.District]; !ok || p.UpdatedAt.After(t) {
			districtMap[p.District] = p.UpdatedAt
		}
		key := p.District + "|" + p.Municipality
		if t, ok := municipalityMap[key]; !ok || p.UpdatedAt.After(t) {
			municipalityMap[key] = p.UpdatedAt
		}
	}

	districts := make([]models.PublishedLocationDTO, 0, len(districtMap))
	for name, updatedAt := range districtMap {
		districts = append(districts, models.PublishedLocationDTO{
			Name:      name,
			Slug:      utils.Slugify(name),
			UpdatedAt: updatedAt,
		})
	}
	sort.Slice(districts, func(i, j int) bool { return districts[i].Name < districts[j].Name })

	municipalities := make([]models.PublishedLocationDTO, 0, len(municipalityMap))
	for key, updatedAt := range municipalityMap {
		parts := strings.SplitN(key, "|", 2)
		if len(parts) != 2 {
			continue
		}
		districtName := parts[0]
		municipalityName := parts[1]
		municipalities = append(municipalities, models.PublishedLocationDTO{
			Name:         municipalityName,
			Slug:         utils.Slugify(municipalityName),
			District:     districtName,
			DistrictSlug: utils.Slugify(districtName),
			UpdatedAt:    updatedAt,
		})
	}
	sort.Slice(municipalities, func(i, j int) bool { return municipalities[i].Name < municipalities[j].Name })

	return &models.PublishedLocationsDTO{
		Districts:      districts,
		Municipalities: municipalities,
	}, nil
}
