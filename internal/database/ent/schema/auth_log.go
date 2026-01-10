package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type AuthLog struct {
	ent.Schema
}

func (AuthLog) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id").Optional().Nillable(),
		field.String("email").MaxLen(255),
		field.String("event_type").MaxLen(50), // LOGIN_SUCCESS, LOGIN_FAILED, LOGOUT, SESSION_REVOKED, etc.
		field.String("ip_address").Optional().Nillable().MaxLen(45),
		field.String("user_agent").Optional().Nillable().MaxLen(500),
		field.String("failure_reason").Optional().Nillable().MaxLen(500),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (AuthLog) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("auth_logs").
			Unique().
			Field("user_id"),
	}
}

func (AuthLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "created_at"),
		index.Fields("email", "created_at"),
		index.Fields("event_type", "created_at"),
		index.Fields("ip_address", "created_at"),
	}
}
