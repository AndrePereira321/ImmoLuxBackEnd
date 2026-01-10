package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type RateLimit struct {
	ent.Schema
}

func (RateLimit) Fields() []ent.Field {
	return []ent.Field{
		field.String("email").MaxLen(255),
		field.String("ip_address").MaxLen(45),
		field.String("action").MaxLen(50), // LOGIN, REGISTER, etc.
		field.Int("attempt_count").Default(1),
		field.Time("window_start").Default(time.Now),
		field.Time("blocked_until").Optional().Nillable(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (RateLimit) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("email", "ip_address", "action").Unique(),
		index.Fields("window_start"),
		index.Fields("blocked_until"),
	}
}
