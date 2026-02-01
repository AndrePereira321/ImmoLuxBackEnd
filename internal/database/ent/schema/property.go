package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Property struct {
	ent.Schema
}

func (Property) Fields() []ent.Field {
	return []ent.Field{
		// Core property information
		field.String("title").NotEmpty().MaxLen(200),
		field.Text("description").NotEmpty(),
		field.Enum("property_type").
			Values("house", "apartment", "villa", "townhouse", "land", "commercial").
			Default("house"),
		field.Float("price").Positive(),
		field.Enum("status").
			Values("available", "pending", "sold", "rented").
			Default("available"),
		field.Bool("is_published").Default(false),

		// Location information (Portuguese administrative levels)
		field.String("address").NotEmpty().MaxLen(255),
		field.String("district").NotEmpty().MaxLen(100),          // Distrito (e.g., Lisboa, Porto)
		field.String("municipality").NotEmpty().MaxLen(100),      // Concelho (e.g., Lisboa, Sintra)
		field.String("parish").Optional().Nillable().MaxLen(100), // Freguesia (optional)
		field.String("postal_code").Optional().Nillable().MaxLen(20),
		field.String("country").Default("PT").MaxLen(2),
		field.Float("latitude").Optional().Nillable(),
		field.Float("longitude").Optional().Nillable(),

		// Property details
		field.Int("bedrooms").Optional().Nillable().NonNegative(),
		field.Int("bathrooms").Optional().Nillable().NonNegative(),
		field.Float("area_sqm").Optional().Nillable().Positive(),
		field.Float("land_area_sqm").Optional().Nillable().Positive(),
		field.Int("year_built").Optional().Nillable(),
		field.Int("floor").Optional().Nillable(),
		field.Int("total_floors").Optional().Nillable(),
		field.Int("parking_spaces").Optional().Nillable().NonNegative(),
		field.Bool("has_garage").Default(false),
		field.Bool("has_garden").Default(false),
		field.Bool("has_pool").Default(false),
		field.Bool("has_elevator").Default(false),
		field.Enum("energy_rating").
			Values("Aplus", "A", "B", "C", "D", "E", "F", "G").
			Optional().
			Nillable(),

		// Virtual tour
		field.String("virtual_tour_url").Optional().Nillable().MaxLen(500),

		// Publisher information
		field.Int("publisher_id").Positive(),

		// Metadata
		field.Int("view_count").Default(0).NonNegative(),
		field.Time("published_at").Optional().Nillable(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (Property) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("publisher", User.Type).
			Ref("properties").
			Unique().
			Required().
			Field("publisher_id"),
		edge.From("contacts", Contact.Type).
			Ref("properties"),
		edge.To("images", PropertyImage.Type),
	}
}

func (Property) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("publisher_id"),
		index.Fields("district", "status", "is_published"),
		index.Fields("municipality", "status", "is_published"),
		index.Fields("price", "status", "is_published"),
		index.Fields("property_type", "status", "is_published"),
	}
}
