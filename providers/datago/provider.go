package datago

import (
	"context"
	"fmt"
	"regexp"

	datagoetp "github.com/ev3rlit/mwosa/providers/clients/datago-etp"
	provider "github.com/ev3rlit/mwosa/providers/core"
	"github.com/ev3rlit/mwosa/providers/core/dailybar"
	"github.com/ev3rlit/mwosa/providers/core/instrument"
	"github.com/ev3rlit/mwosa/providers/spec"
	"github.com/samber/oops"
)

type Config = datagoetp.Config

type priceClient interface {
	GetETFPriceInfo(context.Context, datagoetp.ETFPriceInfoQuery) (datagoetp.ETFPriceInfoResult, error)
	GetETNPriceInfo(context.Context, datagoetp.ETNPriceInfoQuery) (datagoetp.ETNPriceInfoResult, error)
	GetELWPriceInfo(context.Context, datagoetp.ELWPriceInfoQuery) (datagoetp.ELWPriceInfoResult, error)
}

type Provider struct {
	provider.Identity

	dailybar.Fetcher
	instrument.Searcher

	client priceClient
}

func New(config Config) (*Provider, error) {
	errb := oops.In("datago_adapter").With("provider", provider.ProviderDataGo, "group", provider.GroupSecuritiesProductPrice)
	client, err := datagoetp.New(config)
	if err != nil {
		return nil, errb.Wrap(err)
	}
	return NewWithClient(client), nil
}

func NewWithClient(client priceClient) *Provider {
	p := &Provider{
		Identity: provider.Identity{
			ID:          provider.ProviderDataGo,
			DisplayName: "공공데이터포털",
		},
		client: client,
	}

	p.Fetcher = spec.PreviousBusinessDayDailyBar(p.fetchDailyBars).
		Markets(provider.MarketKRX).
		SecurityTypes(
			provider.SecurityTypeETF,
			provider.SecurityTypeETN,
			provider.SecurityTypeELW,
		).
		Group(provider.GroupSecuritiesProductPrice).
		Operations(
			provider.OperationGetETFPriceInfo,
			provider.OperationGetETNPriceInfo,
			provider.OperationGetELWPriceInfo,
		).
		RequiresAuth(provider.CredentialScopeDataGo).
		RangeQuery(dailybar.RangeQuerySupported).
		CompatibilityNotes(
			"latest available basDt is typically the previous business day",
			"current trading-day data is not supported",
		).
		Priority(50).
		Limitations(
			"daily basDt data only; not a realtime or current trading-day provider",
			"latest available data is typically D-1 business day EOD",
			"ELW uses explicit security_type=elw because canonical schema policy is separate from ETF/ETN",
		).
		MustBuild()
	p.Searcher = spec.PreviousBusinessDayInstrumentSearch(p.searchInstruments).
		Markets(provider.MarketKRX).
		SecurityTypes(
			provider.SecurityTypeETF,
			provider.SecurityTypeETN,
			provider.SecurityTypeELW,
		).
		Group(provider.GroupSecuritiesProductPrice).
		Operations(
			provider.OperationGetETFPriceInfo,
			provider.OperationGetETNPriceInfo,
			provider.OperationGetELWPriceInfo,
		).
		RequiresAuth(provider.CredentialScopeDataGo).
		CompatibilityNotes(
			"instrument snapshots are derived from D-1 business day EOD price rows",
			"current trading-day data is not supported",
		).
		Priority(50).
		Limitations(
			"searches public D-1 business day EOD price rows and derives instrument snapshots",
			"not suitable for realtime or current trading-day instrument state",
			"ELW search requires explicit security_type=elw",
		).
		MustBuild()
	return p
}

func Register(registry *provider.Registry, p *Provider) error {
	return registry.RegisterProvider(p)
}

func (p *Provider) fetchDailyBars(ctx context.Context, input dailybar.FetchInput) (dailybar.FetchResult, error) {
	inputErrb := oops.In("datago_adapter").With("role", provider.RoleDailyBar, "market", input.Market, "security_type", input.SecurityType, "symbol", input.Symbol)
	if err := validateMarket(provider.RoleDailyBar, input.Market, input.Symbol, input.SecurityType); err != nil {
		return dailybar.FetchResult{}, inputErrb.Wrap(err)
	}
	operation, err := operationForSecurityType(provider.RoleDailyBar, input.SecurityType, input.Symbol)
	if err != nil {
		return dailybar.FetchResult{}, inputErrb.Wrap(err)
	}
	providerErrb := oops.In("datago_adapter").With("provider", provider.ProviderDataGo, "group", provider.GroupSecuritiesProductPrice)
	if p.client == nil {
		return dailybar.FetchResult{}, providerErrb.New("datago adapter client is nil")
	}

	query := datagoetp.SecuritiesProductPriceQuery{
		NumOfRows: numOfRowsForLimit(input.Limit),
	}
	if input.Symbol != "" {
		query.LikeSrtnCd = input.Symbol
	}
	if input.From != "" && input.From == input.To {
		query.BasDt = input.From
	} else {
		if input.From != "" {
			query.BeginBasDt = input.From
		}
		if input.To != "" {
			query.EndBasDt = input.To
		}
	}

	result, err := p.fetchPriceRecords(ctx, operationSpec{SecurityType: input.SecurityType, Operation: operation}, query)
	if err != nil {
		return dailybar.FetchResult{}, providerErrb.With("operation", operation, "market", input.Market, "security_type", input.SecurityType, "symbol", input.Symbol).Wrapf(err, "fetch datago daily bars")
	}

	bars := make([]dailybar.Bar, 0, len(result.Records))
	for _, record := range result.Records {
		bars = append(bars, normalizeDailyBar(record, input.SecurityType, operation))
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
	inputErrb := oops.In("datago_adapter").With("role", provider.RoleInstrument, "market", input.Market, "security_type", input.SecurityType, "query", input.Query)
	if err := validateMarket(provider.RoleInstrument, input.Market, input.Query, input.SecurityType); err != nil {
		return instrument.SearchResult{}, inputErrb.Wrap(err)
	}
	operations, err := operationsForSearch(input.SecurityType, input.Query)
	if err != nil {
		return instrument.SearchResult{}, inputErrb.Wrap(err)
	}
	providerErrb := oops.In("datago_adapter").With("provider", provider.ProviderDataGo, "group", provider.GroupSecuritiesProductPrice)
	if p.client == nil {
		return instrument.SearchResult{}, providerErrb.New("datago adapter client is nil")
	}

	instruments := make([]instrument.Instrument, 0)
	totalCount := 0
	for _, spec := range operations {
		query := datagoetp.SecuritiesProductPriceQuery{
			NumOfRows: numOfRowsForLimit(input.Limit),
		}
		if looksLikeSecurityCode(input.Query) {
			query.LikeSrtnCd = input.Query
		} else if input.Query != "" {
			query.LikeItmsNm = input.Query
		}

		result, err := p.fetchPriceRecords(ctx, spec, query)
		if err != nil {
			return instrument.SearchResult{}, providerErrb.With("operation", spec.Operation, "market", input.Market, "security_type", spec.SecurityType, "query", input.Query).Wrapf(err, "fetch datago instruments")
		}
		totalCount += result.TotalCount
		for _, record := range result.Records {
			instruments = append(instruments, normalizeInstrument(record, spec.SecurityType, spec.Operation))
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

type priceRecord struct {
	Common datagoetp.CommonPriceInfo
	Fields map[string]string
}

type priceRecordsResult struct {
	Records    []priceRecord
	TotalCount int
}

func operationsForSearch(securityType provider.SecurityType, symbol string) ([]operationSpec, error) {
	if securityType != "" {
		errb := oops.In("datago_adapter").With("role", provider.RoleInstrument, "security_type", securityType, "symbol", symbol)
		operation, err := operationForSecurityType(provider.RoleInstrument, securityType, symbol)
		if err != nil {
			return nil, errb.Wrap(err)
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

func numOfRowsForLimit(limit int) int {
	if limit > 0 && limit < datagoetp.DefaultNumOfRows {
		return limit
	}
	return datagoetp.DefaultNumOfRows
}

func operationIDs(specs []operationSpec) []provider.OperationID {
	operations := make([]provider.OperationID, 0, len(specs))
	for _, spec := range specs {
		operations = append(operations, spec.Operation)
	}
	return operations
}

func (p *Provider) fetchPriceRecords(ctx context.Context, spec operationSpec, query datagoetp.SecuritiesProductPriceQuery) (priceRecordsResult, error) {
	errb := oops.In("datago_adapter").With(
		"provider", provider.ProviderDataGo,
		"group", provider.GroupSecuritiesProductPrice,
		"operation", spec.Operation,
		"security_type", spec.SecurityType,
	)

	switch spec.Operation {
	case provider.OperationGetETFPriceInfo:
		result, err := p.client.GetETFPriceInfo(ctx, datagoetp.ETFPriceInfoQuery{
			SecuritiesProductPriceQuery: query,
		})
		if err != nil {
			return priceRecordsResult{}, errb.Wrap(err)
		}
		return priceRecordsResult{Records: recordsFromETF(result.Items), TotalCount: result.TotalCount}, nil
	case provider.OperationGetETNPriceInfo:
		result, err := p.client.GetETNPriceInfo(ctx, datagoetp.ETNPriceInfoQuery{
			SecuritiesProductPriceQuery: query,
		})
		if err != nil {
			return priceRecordsResult{}, errb.Wrap(err)
		}
		return priceRecordsResult{Records: recordsFromETN(result.Items), TotalCount: result.TotalCount}, nil
	case provider.OperationGetELWPriceInfo:
		result, err := p.client.GetELWPriceInfo(ctx, datagoetp.ELWPriceInfoQuery{
			SecuritiesProductPriceQuery: query,
		})
		if err != nil {
			return priceRecordsResult{}, errb.Wrap(err)
		}
		return priceRecordsResult{Records: recordsFromELW(result.Items), TotalCount: result.TotalCount}, nil
	default:
		return priceRecordsResult{}, errb.New("unsupported datago price info operation")
	}
}

func recordsFromETF(items []datagoetp.ETFPriceInfo) []priceRecord {
	records := make([]priceRecord, 0, len(items))
	for _, item := range items {
		records = append(records, priceRecord{Common: item.CommonPriceInfo, Fields: item.Fields()})
	}
	return records
}

func recordsFromETN(items []datagoetp.ETNPriceInfo) []priceRecord {
	records := make([]priceRecord, 0, len(items))
	for _, item := range items {
		records = append(records, priceRecord{Common: item.CommonPriceInfo, Fields: item.Fields()})
	}
	return records
}

func recordsFromELW(items []datagoetp.ELWPriceInfo) []priceRecord {
	records := make([]priceRecord, 0, len(items))
	for _, item := range items {
		records = append(records, priceRecord{Common: item.CommonPriceInfo, Fields: item.Fields()})
	}
	return records
}

func normalizeDailyBar(record priceRecord, securityType provider.SecurityType, operation provider.OperationID) dailybar.Bar {
	item := record.Common
	return dailybar.Bar{
		Provider:     provider.ProviderDataGo,
		Group:        provider.GroupSecuritiesProductPrice,
		Operation:    operation,
		Market:       provider.MarketKRX,
		SecurityType: securityType,
		Symbol:       item.SrtnCd,
		ISIN:         item.IsinCd,
		Name:         item.ItmsNm,
		TradingDate:  normalizeDate(item.BasDt),
		Currency:     "KRW",
		Open:         item.Mkp,
		High:         item.Hipr,
		Low:          item.Lopr,
		Close:        item.Clpr,
		Change:       item.Vs,
		ChangeRate:   item.FltRt,
		Volume:       item.Trqu,
		TradedValue:  item.TrPrc,
		MarketCap:    item.MrktTotAmt,
		Extensions:   extensionFields(record.Fields),
	}
}

func normalizeInstrument(record priceRecord, securityType provider.SecurityType, operation provider.OperationID) instrument.Instrument {
	item := record.Common
	securityCode := item.SrtnCd
	return instrument.Instrument{
		Provider:     provider.ProviderDataGo,
		Group:        provider.GroupSecuritiesProductPrice,
		Operation:    operation,
		Market:       provider.MarketKRX,
		SecurityType: securityType,
		SecurityCode: securityCode,
		ISIN:         item.IsinCd,
		Name:         item.ItmsNm,
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

func extensionFields(item map[string]string) map[string]string {
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
