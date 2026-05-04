package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Strategy struct {
	ent.Schema
}

func (Strategy) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Annotation{Table: "strategies"},
	}
}

func (Strategy) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").NotEmpty().Unique().Immutable(),
		field.String("name").NotEmpty().Unique(),
		field.String("engine").NotEmpty(),
		field.String("active_version_id").NotEmpty(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
		field.Time("archived_at").Optional().Nillable(),
	}
}

func (Strategy) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("archived_at").StorageKey("idx_strategies_archived_at"),
	}
}
