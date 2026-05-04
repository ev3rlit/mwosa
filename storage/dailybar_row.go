package storage

import (
	"time"

	"github.com/uptrace/bun"
)

type DailyBarRow struct {
	bun.BaseModel `bun:"table:daily_bar,alias:daily_bar"`

	ID                               int64     `bun:"id,pk,autoincrement"`
	Provider                         string    `bun:"provider,notnull"`
	ProviderGroup                    string    `bun:"provider_group,notnull"`
	Operation                        string    `bun:"operation,notnull"`
	Market                           string    `bun:"market,notnull"`
	SecurityType                     string    `bun:"security_type,notnull"`
	Symbol                           string    `bun:"symbol,notnull"`
	ISIN                             string    `bun:"isin,notnull,default:''"`
	Name                             string    `bun:"name,notnull,default:''"`
	TradingDate                      string    `bun:"trading_date,notnull"`
	Currency                         string    `bun:"currency,notnull,default:''"`
	OpeningPrice                     string    `bun:"opening_price,notnull,default:''"`
	HighestPrice                     string    `bun:"highest_price,notnull,default:''"`
	LowestPrice                      string    `bun:"lowest_price,notnull,default:''"`
	ClosingPrice                     string    `bun:"closing_price,notnull,default:''"`
	PriceChangeFromPreviousClose     string    `bun:"price_change_from_previous_close,notnull,default:''"`
	PriceChangeRateFromPreviousClose string    `bun:"price_change_rate_from_previous_close,notnull,default:''"`
	TradedVolume                     string    `bun:"traded_volume,notnull,default:''"`
	TradedAmount                     string    `bun:"traded_amount,notnull,default:''"`
	MarketCapitalization             string    `bun:"market_capitalization,notnull,default:''"`
	ExtensionsJSON                   string    `bun:"extensions_json,notnull,default:'{}'"`
	CreatedAt                        time.Time `bun:"created_at,notnull,default:CURRENT_TIMESTAMP"`
	UpdatedAt                        time.Time `bun:"updated_at,notnull,default:CURRENT_TIMESTAMP"`
}
