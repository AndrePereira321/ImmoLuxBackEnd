package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type User struct {
	ent.Schema
}

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("first_name").NotEmpty().MaxLen(100),
		field.String("last_name").NotEmpty().MaxLen(100),
		field.String("email").Unique().NotEmpty().MaxLen(255),
		field.Bool("is_active").Default(true),
		field.Bool("is_super_user").Default(false),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("auth", UserAuth.Type).Unique(),
		edge.To("sessions", Session.Type),
		edge.To("properties", Property.Type),
		edge.To("contacts", Contact.Type),
		edge.To("auth_logs", AuthLog.Type),
	}
}

func (User) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("email"),
	}
}
