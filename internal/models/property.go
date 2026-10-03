package models

import "time"

type ContactDTO struct {
	ID        *RecordId  `json:"id"`
	UserID    *RecordId  `json:"userId"`
	Name      *string    `json:"name"`
	Email     *string    `json:"email"`
	Phone     *string    `json:"phone"`
	Notes     *string    `json:"notes"`
	CreatedAt *time.Time `json:"createdAt"`
	UpdatedAt *time.Time `json:"updatedAt"`
}

type PropertyDTO struct {
	ID             *RecordId    `json:"id"`
	Title          *string      `json:"title"`
	Description    *string      `json:"description"`
	PropertyType   *string      `json:"propertyType"`
	Price          *float64     `json:"price"`
	Status         *string      `json:"status"`
	IsPublished    *bool        `json:"isPublished"`
	Address        *string      `json:"address"`
	District       *string      `json:"district"`     // Distrito (required)
	Municipality   *string      `json:"municipality"` // Concelho (required)
	Parish         *string      `json:"parish"`       // Freguesia (optional)
	PostalCode     *string      `json:"postalCode"`
	Country        *string      `json:"country"`
	Latitude       *float64     `json:"latitude"`
	Longitude      *float64     `json:"longitude"`
	Bedrooms       *int         `json:"bedrooms"`
	Bathrooms      *int         `json:"bathrooms"`
	AreaSqm        *float64     `json:"areaSqm"`
	LandAreaSqm    *float64     `json:"landAreaSqm"`
	YearBuilt      *int         `json:"yearBuilt"`
	Floor          *int         `json:"floor"`
	TotalFloors    *int         `json:"totalFloors"`
	ParkingSpaces  *int         `json:"parkingSpaces"`
	HasGarage      *bool        `json:"hasGarage"`
	HasGarden      *bool        `json:"hasGarden"`
	HasPool        *bool        `json:"hasPool"`
	HasElevator    *bool        `json:"hasElevator"`
	EnergyRating   *string      `json:"energyRating"`
	VirtualTourURL *string      `json:"virtualTourUrl"`
	ContactIDs     []RecordId   `json:"contactIds,omitempty"` // Array of contact IDs
	Contacts       []ContactDTO `json:"contacts,omitempty"`   // Array of contact objects
	PublisherID    *RecordId    `json:"publisherId"`
	ViewCount      *int         `json:"viewCount"`
	PublishedAt    *time.Time   `json:"publishedAt"`
	CreatedAt      *time.Time   `json:"createdAt"`
	UpdatedAt      *time.Time   `json:"updatedAt"`
}

type PropertyImageDTO struct {
	ID           *RecordId  `json:"id"`
	PropertyID   *RecordId  `json:"propertyId"`
	ContentType  *string    `json:"contentType"`
	FileSize     *int       `json:"fileSize"`
	Width        *int       `json:"width"`
	Height       *int       `json:"height"`
	DisplayOrder *int       `json:"displayOrder"`
	CreatedAt    *time.Time `json:"createdAt"`
}

type PropertyWithImagesDTO struct {
	Property *PropertyDTO       `json:"property"`
	Images   []PropertyImageDTO `json:"images"`
}

// FacetBucketDTO is one selectable option in the property search, carrying the
// number of published properties that remain if it is chosen. Parent is set only
// for municipalities, naming the district they belong to.
type FacetBucketDTO struct {
	Value  string `json:"value"`
	Count  int    `json:"count"`
	Parent string `json:"parent,omitempty"`
}

// PropertyFacetsDTO is returned by GET /v1/api/properties/facets.
//
// Every dimension is counted with all the active filters applied EXCEPT its own.
// That exclusion is the whole point: counted against itself, every option but the
// chosen one reads zero, and the reader learns nothing about where else to look.
// MinPrice and MaxPrice bound the properties matching everything but the price
// filter, so the range a buyer is shown is the range that is actually on offer.
type PropertyFacetsDTO struct {
	Total          int              `json:"total"`
	Districts      []FacetBucketDTO `json:"districts"`
	Municipalities []FacetBucketDTO `json:"municipalities"`
	Parishes       []FacetBucketDTO `json:"parishes"`
	PropertyTypes  []FacetBucketDTO `json:"propertyTypes"`
	Statuses       []FacetBucketDTO `json:"statuses"`
	MinPrice       *float64         `json:"minPrice"`
	MaxPrice       *float64         `json:"maxPrice"`
}

// LocationStatsDTO is returned by GET /v1/api/locations/stats
type LocationStatsDTO struct {
	District       string  `json:"district"`     // Canonical district name, e.g. "Lisboa"
	Municipality   *string `json:"municipality"` // Canonical municipality name, nil for district-level
	Total          int     `json:"total"`
	MinPrice       float64 `json:"minPrice"`
	MaxPrice       float64 `json:"maxPrice"`
	AvgPrice       float64 `json:"avgPrice"`
	MostCommonType string  `json:"mostCommonType"`
}

// PublishedLocationDTO is one entry in the published-locations response.
// District and DistrictSlug are only set for municipality entries.
type PublishedLocationDTO struct {
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	District     string    `json:"district,omitempty"`     // Set only for municipalities
	DistrictSlug string    `json:"districtSlug,omitempty"` // Set only for municipalities
	UpdatedAt    time.Time `json:"updatedAt"`
}

// PublishedLocationsDTO is returned by GET /v1/api/locations/published
type PublishedLocationsDTO struct {
	Districts      []PublishedLocationDTO `json:"districts"`
	Municipalities []PublishedLocationDTO `json:"municipalities"`
}
