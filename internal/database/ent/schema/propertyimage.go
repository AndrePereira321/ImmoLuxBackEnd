package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type PropertyImage struct {
	ent.Schema
}

func (PropertyImage) Fields() []ent.Field {
	return []ent.Field{
		field.Int("property_id").Positive(),
		field.Bytes("image_data").NotEmpty().Sensitive(),
		field.String("content_type").NotEmpty().MaxLen(50),
		field.Int("file_size").Positive(),
		field.Int("width").Positive(),
		field.Int("height").Positive(),
		field.Int("display_order").Default(0).NonNegative(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (PropertyImage) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("property", Property.Type).
			Ref("images").
			Unique().
			Required().
			Field("property_id"),
	}
}

func (PropertyImage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("property_id", "display_order"),
	}
}
