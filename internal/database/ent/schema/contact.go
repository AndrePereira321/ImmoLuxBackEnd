package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Contact struct {
	ent.Schema
}

func (Contact) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id").Positive(),
		field.String("name").NotEmpty().MaxLen(150),
		field.String("email").NotEmpty().MaxLen(255),
		field.String("phone").NotEmpty().MaxLen(20),
		field.Text("notes").Optional().Nillable(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (Contact) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("contacts").
			Unique().
			Required().
			Field("user_id"),
		edge.To("properties", Property.Type),
	}
}

func (Contact) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("user_id", "email"),
	}
}
