package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type ScreenRunItem struct {
	ent.Schema
}

func (ScreenRunItem) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Annotation{Table: "screen_run_items"},
	}
}

func (ScreenRunItem) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").NotEmpty().Unique().Immutable(),
		field.String("screen_run_id").NotEmpty(),
		field.Int("ordinal").NonNegative(),
		field.String("symbol").Default(""),
		field.Bytes("payload_json").NotEmpty(),
	}
}

func (ScreenRunItem) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("screen_run_id", "ordinal").
			Unique().
			StorageKey("screen_run_items_run_ordinal_unique"),
		index.Fields("symbol").StorageKey("idx_screen_run_items_symbol"),
	}
}
