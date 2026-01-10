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
	ID             *RecordId   `json:"id"`
	Title          *string     `json:"title"`
	Description    *string     `json:"description"`
	PropertyType   *string     `json:"propertyType"`
	Price          *float64    `json:"price"`
	Status         *string     `json:"status"`
	IsPublished    *bool       `json:"isPublished"`
	Address        *string     `json:"address"`
	District       *string     `json:"district"`     // Distrito (required)
	Municipality   *string     `json:"municipality"` // Concelho (required)
	Parish         *string     `json:"parish"`       // Freguesia (optional)
	PostalCode     *string     `json:"postalCode"`
	Country        *string     `json:"country"`
	Latitude       *float64    `json:"latitude"`
	Longitude      *float64    `json:"longitude"`
	Bedrooms       *int        `json:"bedrooms"`
	Bathrooms      *int        `json:"bathrooms"`
	AreaSqm        *float64    `json:"areaSqm"`
	LandAreaSqm    *float64    `json:"landAreaSqm"`
	YearBuilt      *int        `json:"yearBuilt"`
	Floor          *int        `json:"floor"`
	TotalFloors    *int        `json:"totalFloors"`
	ParkingSpaces  *int        `json:"parkingSpaces"`
	HasGarage      *bool       `json:"hasGarage"`
	HasGarden      *bool       `json:"hasGarden"`
	HasPool        *bool       `json:"hasPool"`
	HasElevator    *bool       `json:"hasElevator"`
	EnergyRating   *string     `json:"energyRating"`
	VirtualTourURL *string     `json:"virtualTourUrl"`
	ContactID      *RecordId   `json:"contactId"`
	Contact        *ContactDTO `json:"contact,omitempty"`
	PublisherID    *RecordId   `json:"publisherId"`
	ViewCount      *int        `json:"viewCount"`
	PublishedAt    *time.Time  `json:"publishedAt"`
	CreatedAt      *time.Time  `json:"createdAt"`
	UpdatedAt      *time.Time  `json:"updatedAt"`
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
