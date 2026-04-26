package datago

import (
	"context"
	"fmt"
	"net/url"
	"regexp"

	provider "github.com/ev3rlit/mwosa/providers/core"
	"github.com/ev3rlit/mwosa/providers/core/dailybar"
	"github.com/ev3rlit/mwosa/providers/core/instrument"
)

type Provider struct {
	provider.Identity

	dailybar.Fetch
	instrument.Search

	client *Client
}

func New(config Config) (*Provider, error) {
	client, err := NewClient(config)
	if err != nil {
		return nil, err
	}
	return NewWithClient(client), nil
}

func NewWithClient(client *Client) *Provider {
	p := &Provider{
		Identity: provider.Identity{
			ID:          provider.ProviderDataGo,
			DisplayName: "공공데이터포털",
		},
		client: client,
	}

	p.Fetch = dailybar.NewFetch(dailyBarProfile(), p.fetchDailyBars)
	p.Search = instrument.NewSearch(instrumentSearchProfile(), p.searchInstruments)
	return p
}

func (p *Provider) RoleRegistrations() []provider.RoleRegistration {
	return []provider.RoleRegistration{
		{
			Profile: p.DailyBarProfile().RoleProfile(),
			Impl:    p,
		},
		{
			Profile: p.InstrumentSearchProfile().RoleProfile(),
			Impl:    p,
		},
	}
}

func Register(registry *provider.Registry, p *Provider) error {
	return registry.Register(p, p.RoleRegistrations()...)
}

func dailyBarProfile() dailybar.Profile {
	return dailybar.Profile{
		Markets: []provider.Market{provider.MarketKRX},
		SecurityTypes: []provider.SecurityType{
			provider.SecurityTypeETF,
			provider.SecurityTypeETN,
			provider.SecurityTypeELW,
		},
		Group: provider.GroupSecuritiesProductPrice,
		Operations: []provider.OperationID{
			provider.OperationGetETFPriceInfo,
			provider.OperationGetETNPriceInfo,
			provider.OperationGetELWPriceInfo,
		},
		AuthScope:    provider.CredentialScopeDataGo,
		RangeQuery:   dailybar.RangeQuerySupported,
		Freshness:    provider.FreshnessDaily,
		RequiresAuth: true,
		Priority:     50,
		Limitations: []string{
			"daily basDt data only; not a realtime quote snapshot provider",
			"ELW uses explicit security_type=elw because canonical schema policy is separate from ETF/ETN",
		},
	}
}

func instrumentSearchProfile() instrument.Profile {
	return instrument.Profile{
		Markets: []provider.Market{provider.MarketKRX},
		SecurityTypes: []provider.SecurityType{
			provider.SecurityTypeETF,
			provider.SecurityTypeETN,
			provider.SecurityTypeELW,
		},
		Group: provider.GroupSecuritiesProductPrice,
		Operations: []provider.OperationID{
			provider.OperationGetETFPriceInfo,
			provider.OperationGetETNPriceInfo,
			provider.OperationGetELWPriceInfo,
		},
		AuthScope:    provider.CredentialScopeDataGo,
		Freshness:    provider.FreshnessDaily,
		RequiresAuth: true,
		Priority:     50,
		Limitations: []string{
			"searches public price rows and derives instrument snapshots",
			"ELW search requires explicit security_type=elw",
		},
	}
}

func (p *Provider) fetchDailyBars(ctx context.Context, input dailybar.FetchInput) (dailybar.FetchResult, error) {
	if err := validateMarket(provider.RoleDailyBar, input.Market, input.Symbol, input.SecurityType); err != nil {
		return dailybar.FetchResult{}, err
	}
	operation, err := operationForSecurityType(provider.RoleDailyBar, input.SecurityType, input.Symbol)
	if err != nil {
		return dailybar.FetchResult{}, err
	}

	params := url.Values{}
	if input.Symbol != "" {
		params.Set("likeSrtnCd", input.Symbol)
	}
	if input.From != "" && input.From == input.To {
		params.Set("basDt", input.From)
	} else {
		if input.From != "" {
			params.Set("beginBasDt", input.From)
		}
		if input.To != "" {
			params.Set("endBasDt", input.To)
		}
	}

	result, err := p.client.fetchPrices(ctx, priceQuery{
		Operation: operation,
		Params:    params,
		Limit:     input.Limit,
	})
	if err != nil {
		return dailybar.FetchResult{}, err
	}

	bars := make([]dailybar.Bar, 0, len(result.Items))
	for _, item := range result.Items {
		bars = append(bars, normalizeDailyBar(item, input.SecurityType, operation))
	}

	return dailybar.FetchResult{
		Bars:       bars,
		Provider:   p.Identity,
		Group:      provider.GroupSecuritiesProductPrice,
		Operation:  operation,
		TotalCount: result.TotalCount,
	}, nil
}

func (p *Provider) searchInstruments(ctx context.Context, input instrument.SearchInput) (instrument.SearchResult, error) {
	if err := validateMarket(provider.RoleInstrument, input.Market, input.Query, input.SecurityType); err != nil {
		return instrument.SearchResult{}, err
	}
	operations, err := operationsForSearch(input.SecurityType, input.Query)
	if err != nil {
		return instrument.SearchResult{}, err
	}

	instruments := make([]instrument.Instrument, 0)
	totalCount := 0
	for _, spec := range operations {
		params := url.Values{}
		if looksLikeSecurityCode(input.Query) {
			params.Set("likeSrtnCd", input.Query)
		} else if input.Query != "" {
			params.Set("likeItmsNm", input.Query)
		}

		result, err := p.client.fetchPrices(ctx, priceQuery{
			Operation: spec.Operation,
			Params:    params,
			Limit:     input.Limit,
		})
		if err != nil {
			return instrument.SearchResult{}, err
		}
		totalCount += result.TotalCount
		for _, item := range result.Items {
			instruments = append(instruments, normalizeInstrument(item, spec.SecurityType, spec.Operation))
			if input.Limit > 0 && len(instruments) >= input.Limit {
				return instrument.SearchResult{
					Instruments: instruments,
					Provider:    p.Identity,
					Group:       provider.GroupSecuritiesProductPrice,
					Operations:  operationIDs(operations),
					TotalCount:  totalCount,
				}, nil
			}
		}
	}

	return instrument.SearchResult{
		Instruments: instruments,
		Provider:    p.Identity,
		Group:       provider.GroupSecuritiesProductPrice,
		Operations:  operationIDs(operations),
		TotalCount:  totalCount,
	}, nil
}

type operationSpec struct {
	SecurityType provider.SecurityType
	Operation    provider.OperationID
}

func operationsForSearch(securityType provider.SecurityType, symbol string) ([]operationSpec, error) {
	if securityType != "" {
		operation, err := operationForSecurityType(provider.RoleInstrument, securityType, symbol)
		if err != nil {
			return nil, err
		}
		return []operationSpec{{SecurityType: securityType, Operation: operation}}, nil
	}

	return []operationSpec{
		{SecurityType: provider.SecurityTypeETF, Operation: provider.OperationGetETFPriceInfo},
		{SecurityType: provider.SecurityTypeETN, Operation: provider.OperationGetETNPriceInfo},
	}, nil
}

func operationForSecurityType(capability provider.Role, securityType provider.SecurityType, symbol string) (provider.OperationID, error) {
	switch securityType {
	case provider.SecurityTypeETF:
		return provider.OperationGetETFPriceInfo, nil
	case provider.SecurityTypeETN:
		return provider.OperationGetETNPriceInfo, nil
	case provider.SecurityTypeELW:
		return provider.OperationGetELWPriceInfo, nil
	case "":
		return "", provider.NewUnsupported(provider.UnsupportedError{
			Capability: capability,
			ProviderID: provider.ProviderDataGo,
			GroupID:    provider.GroupSecuritiesProductPrice,
			Market:     provider.MarketKRX,
			Symbol:     symbol,
			Reason:     "security_type is required for daily bars",
		})
	default:
		return "", provider.NewUnsupported(provider.UnsupportedError{
			Capability:   capability,
			ProviderID:   provider.ProviderDataGo,
			GroupID:      provider.GroupSecuritiesProductPrice,
			Market:       provider.MarketKRX,
			SecurityType: securityType,
			Symbol:       symbol,
			Reason:       "security_type is not supported by datago securitiesProductPrice",
		})
	}
}

func validateMarket(capability provider.Role, market provider.Market, symbol string, securityType provider.SecurityType) error {
	if market == "" || market == provider.MarketKRX {
		return nil
	}
	return provider.NewUnsupported(provider.UnsupportedError{
		Capability:   capability,
		ProviderID:   provider.ProviderDataGo,
		GroupID:      provider.GroupSecuritiesProductPrice,
		Market:       market,
		SecurityType: securityType,
		Symbol:       symbol,
		Reason:       "market is not supported by datago securitiesProductPrice",
	})
}

var securityCodePattern = regexp.MustCompile(`^[0-9A-Za-z]{3,12}$`)

func looksLikeSecurityCode(query string) bool {
	return securityCodePattern.MatchString(query)
}

func operationIDs(specs []operationSpec) []provider.OperationID {
	operations := make([]provider.OperationID, 0, len(specs))
	for _, spec := range specs {
		operations = append(operations, spec.Operation)
	}
	return operations
}

func normalizeDailyBar(item priceItem, securityType provider.SecurityType, operation provider.OperationID) dailybar.Bar {
	return dailybar.Bar{
		Provider:     provider.ProviderDataGo,
		Group:        provider.GroupSecuritiesProductPrice,
		Operation:    operation,
		Market:       provider.MarketKRX,
		SecurityType: securityType,
		Symbol:       item["srtnCd"],
		ISIN:         item["isinCd"],
		Name:         item["itmsNm"],
		TradingDate:  normalizeDate(item["basDt"]),
		Currency:     "KRW",
		Open:         item["mkp"],
		High:         item["hipr"],
		Low:          item["lopr"],
		Close:        item["clpr"],
		Change:       item["vs"],
		ChangeRate:   item["fltRt"],
		Volume:       item["trqu"],
		TradedValue:  item["trPrc"],
		MarketCap:    item["mrktTotAmt"],
		Extensions:   extensionFields(item),
	}
}

func normalizeInstrument(item priceItem, securityType provider.SecurityType, operation provider.OperationID) instrument.Instrument {
	securityCode := item["srtnCd"]
	return instrument.Instrument{
		Provider:     provider.ProviderDataGo,
		Group:        provider.GroupSecuritiesProductPrice,
		Operation:    operation,
		Market:       provider.MarketKRX,
		SecurityType: securityType,
		SecurityCode: securityCode,
		ISIN:         item["isinCd"],
		Name:         item["itmsNm"],
		ExchangeCode: "KRX",
		CountryCode:  "KR",
		Timezone:     "Asia/Seoul",
		Extensions: map[string]string{
			"security_key":         fmt.Sprintf("krx:%s", securityCode),
			"canonical_record_key": fmt.Sprintf("instrument:krx:%s:current", securityCode),
			"market_segment":       string(securityType),
		},
	}
}

func normalizeDate(value string) string {
	if len(value) != 8 {
		return value
	}
	return fmt.Sprintf("%s-%s-%s", value[:4], value[4:6], value[6:8])
}

func extensionFields(item priceItem) map[string]string {
	extensions := make(map[string]string)
	for key, value := range item {
		if isCommonDailyBarField(key) {
			continue
		}
		extensions[key] = value
	}
	return extensions
}

func isCommonDailyBarField(key string) bool {
	switch key {
	case "basDt", "srtnCd", "isinCd", "itmsNm", "clpr", "vs", "fltRt", "mkp", "hipr", "lopr", "trqu", "trPrc", "mrktTotAmt":
		return true
	default:
		return false
	}
}
