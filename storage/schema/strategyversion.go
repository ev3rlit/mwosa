package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type StrategyVersion struct {
	ent.Schema
}

func (StrategyVersion) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Annotation{Table: "strategy_versions"},
	}
}

func (StrategyVersion) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").NotEmpty().Unique().Immutable(),
		field.String("strategy_id").NotEmpty(),
		field.Int("version").Positive(),
		field.String("query_text").NotEmpty(),
		field.String("query_hash").NotEmpty(),
		field.String("input_dataset").NotEmpty(),
		field.Int("input_schema_version").Positive(),
		field.Bytes("params_json").Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.String("note").Default(""),
	}
}

func (StrategyVersion) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("strategy_id", "version").
			Unique().
			StorageKey("strategy_versions_strategy_version_unique"),
		index.Fields("query_hash").StorageKey("idx_strategy_versions_query_hash"),
	}
}
