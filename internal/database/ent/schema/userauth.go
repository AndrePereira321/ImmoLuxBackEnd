package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type UserAuth struct {
	ent.Schema
}

func (UserAuth) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id").Unique().Positive(),
		field.String("hash").NotEmpty().Sensitive(),
		field.Time("password_changed_at").Optional().Nillable(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (UserAuth) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("auth").
			Unique().
			Required().
			Field("user_id"),
	}
}

func (UserAuth) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
	}
}
