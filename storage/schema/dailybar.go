package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type DailyBar struct {
	ent.Schema
}

func (DailyBar) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Annotation{Table: "daily_bar"},
	}
}

func (DailyBar) Fields() []ent.Field {
	return []ent.Field{
		field.String("provider").NotEmpty(),
		field.String("provider_group").NotEmpty(),
		field.String("operation").NotEmpty(),
		field.String("market").NotEmpty(),
		field.String("security_type").NotEmpty(),
		field.String("symbol").NotEmpty(),
		field.String("isin").Default(""),
		field.String("name").Default(""),
		field.String("trading_date").NotEmpty(),
		field.String("currency").Default(""),
		field.String("opening_price").Default(""),
		field.String("highest_price").Default(""),
		field.String("lowest_price").Default(""),
		field.String("closing_price").Default(""),
		field.String("price_change_from_previous_close").Default(""),
		field.String("price_change_rate_from_previous_close").Default(""),
		field.String("traded_volume").Default(""),
		field.String("traded_amount").Default(""),
		field.String("market_capitalization").Default(""),
		field.String("extensions_json").Default("{}"),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (DailyBar) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("market", "security_type", "trading_date", "symbol", "provider", "provider_group").
			Unique().
			StorageKey("daily_bar_natural_key"),
		index.Fields("market", "security_type", "trading_date").
			StorageKey("idx_daily_bar_date"),
		index.Fields("market", "security_type", "symbol", "trading_date").
			StorageKey("idx_daily_bar_symbol_date"),
	}
}
