package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Session struct {
	ent.Schema
}

func (Session) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id").Positive(),
		field.String("session_token").Unique().NotEmpty().Sensitive().MaxLen(255),
		field.Bool("is_active").Default(true),
		field.Time("expires_at"),
		field.Time("invalidated_at").Optional().Nillable(),
		field.String("invalidated_reason").Optional().Nillable().MaxLen(500),
		field.String("ip_address").Optional().Nillable().MaxLen(45), // IPv6 max length
		field.String("user_agent").Optional().Nillable().MaxLen(500),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (Session) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("sessions").
			Unique().
			Required().
			Field("user_id"),
	}
}

// Indexes of the Session.
func (Session) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "is_active"),
		index.Fields("session_token"),
	}
}
