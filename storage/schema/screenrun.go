package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type ScreenRun struct {
	ent.Schema
}

func (ScreenRun) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Annotation{Table: "screen_runs"},
	}
}

func (ScreenRun) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").NotEmpty().Unique().Immutable(),
		field.String("alias").Optional().Nillable().Unique(),
		field.String("strategy_id").NotEmpty(),
		field.String("strategy_version_id").NotEmpty(),
		field.String("query_hash").NotEmpty(),
		field.String("input_dataset").NotEmpty(),
		field.Int("input_schema_version").Positive(),
		field.Bytes("params_json").Optional(),
		field.String("data_from").Default(""),
		field.String("data_to").Default(""),
		field.String("data_as_of").Default(""),
		field.Time("started_at").Default(time.Now).Immutable(),
		field.Time("finished_at").Optional().Nillable(),
		field.String("status").NotEmpty(),
		field.Int("result_count").NonNegative(),
		field.String("result_hash").Default(""),
		field.Int64("result_size_bytes").NonNegative(),
		field.Bytes("summary_json").Optional(),
		field.String("error_message").Default(""),
	}
}

func (ScreenRun) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("started_at").StorageKey("idx_screen_runs_started_at"),
		index.Fields("strategy_id", "started_at").StorageKey("idx_screen_runs_strategy_started"),
	}
}
