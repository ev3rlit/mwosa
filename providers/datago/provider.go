package datago

import (
	"context"
	"fmt"
	"net/http"
	"time"

	datagoetp "github.com/ev3rlit/mwosa/clients/datago-etp"
	datagostock "github.com/ev3rlit/mwosa/clients/datago-stock-price"
	provider "github.com/ev3rlit/mwosa/providers/core"
	"github.com/ev3rlit/mwosa/providers/core/dailybar"
	"github.com/ev3rlit/mwosa/providers/core/instrument"
	"github.com/samber/oops"
)

type Config struct {
	ServiceKey             string
	BaseURL                string
	SecuritiesProductPrice GroupConfig
	StockPrice             GroupConfig
	HTTPClient             *http.Client
	RetryMaxAttempts       int
	RetryInitialWait       time.Duration
	RetryMaxWait           time.Duration
}

type GroupConfig struct {
	ServiceKey string
	BaseURL    string
}

type etpPriceClient interface {
	GetETFPriceInfo(context.Context, datagoetp.ETFPriceInfoQuery) (datagoetp.ETFPriceInfoResult, error)
	GetAllETFPriceInfo(context.Context, datagoetp.ETFPriceInfoQuery) (datagoetp.ETFPriceInfoResult, error)
	GetETNPriceInfo(context.Context, datagoetp.ETNPriceInfoQuery) (datagoetp.ETNPriceInfoResult, error)
	GetAllETNPriceInfo(context.Context, datagoetp.ETNPriceInfoQuery) (datagoetp.ETNPriceInfoResult, error)
	GetELWPriceInfo(context.Context, datagoetp.ELWPriceInfoQuery) (datagoetp.ELWPriceInfoResult, error)
	GetAllELWPriceInfo(context.Context, datagoetp.ELWPriceInfoQuery) (datagoetp.ELWPriceInfoResult, error)
}

type stockPriceClient interface {
	GetStockPriceInfo(context.Context, datagostock.StockPriceInfoQuery) (datagostock.StockPriceInfoResult, error)
	GetAllStockPriceInfo(context.Context, datagostock.StockPriceInfoQuery) (datagostock.StockPriceInfoResult, error)
}

type Provider struct {
	provider.Identity

	etpClient   etpPriceClient
	stockClient stockPriceClient
	groups      []provider.GroupRoleProvider
}

func New(config Config) (*Provider, error) {
	errb := oops.In("datago_adapter").With("provider", provider.ProviderDataGo)
	etpClient, err := newETPClient(config)
	if err != nil {
		return nil, errb.With("group", provider.GroupSecuritiesProductPrice).Wrap(err)
	}
	stockClient, err := newStockClient(config)
	if err != nil {
		return nil, errb.With("group", provider.GroupStockPrice).Wrap(err)
	}
	if etpClient == nil && stockClient == nil {
		return nil, errb.New("datago provider config requires at least one group service key")
	}
	return NewWithClients(etpClient, stockClient), nil
}

func newETPClient(config Config) (etpPriceClient, error) {
	serviceKey := config.SecuritiesProductPrice.ServiceKey
	if serviceKey == "" {
		serviceKey = config.ServiceKey
	}
	if serviceKey == "" {
		return nil, nil
	}
	baseURL := config.SecuritiesProductPrice.BaseURL
	if baseURL == "" {
		baseURL = config.BaseURL
	}
	return datagoetp.New(datagoetp.Config{
		ServiceKey:       serviceKey,
		BaseURL:          baseURL,
		HTTPClient:       config.HTTPClient,
		RetryMaxAttempts: config.RetryMaxAttempts,
		RetryInitialWait: config.RetryInitialWait,
		RetryMaxWait:     config.RetryMaxWait,
	})
}

func newStockClient(config Config) (stockPriceClient, error) {
	serviceKey := config.StockPrice.ServiceKey
	if serviceKey == "" {
		return nil, nil
	}
	return datagostock.New(datagostock.Config{
		ServiceKey:       serviceKey,
		BaseURL:          config.StockPrice.BaseURL,
		HTTPClient:       config.HTTPClient,
		RetryMaxAttempts: config.RetryMaxAttempts,
		RetryInitialWait: config.RetryInitialWait,
		RetryMaxWait:     config.RetryMaxWait,
	})
}

func NewWithClient(client etpPriceClient) *Provider {
	return NewWithClients(client, nil)
}

func NewWithClients(etpClient etpPriceClient, stockClient stockPriceClient) *Provider {
	p := &Provider{
		Identity: provider.Identity{
			ID:          provider.ProviderDataGo,
			DisplayName: "공공데이터포털",
		},
		etpClient:   etpClient,
		stockClient: stockClient,
	}

	if etpClient != nil {
		p.groups = append(p.groups, newSecuritiesProductPriceGroup(p.fetchDailyBars, p.searchInstruments))
	}
	if stockClient != nil {
		p.groups = append(p.groups, newStockPriceGroup(p.fetchDailyBars, p.searchInstruments))
	}
	return p
}

func (p *Provider) FetchDailyBars(ctx context.Context, input dailybar.FetchInput) (dailybar.FetchResult, error) {
	return p.fetchDailyBars(ctx, input)
}

func (p *Provider) SearchInstruments(ctx context.Context, input instrument.SearchInput) (instrument.SearchResult, error) {
	return p.searchInstruments(ctx, input)
}

func Register(registry *provider.Registry, p *Provider) error {
	return registry.RegisterProvider(p)
}

func (p *Provider) RoleRegistrations() []provider.RoleRegistration {
	if p == nil {
		return nil
	}
	registrations := make([]provider.RoleRegistration, 0)
	for _, group := range p.groups {
		registrations = append(registrations, group.RoleRegistrations()...)
	}
	return registrations
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
	group := groupForOperation(operation)
	providerErrb := oops.In("datago_adapter").With("provider", provider.ProviderDataGo, "group", group)

	query := datagoetp.SecuritiesProductPriceQuery{
		NumOfRows: numOfRowsForDailyFetch(input.Limit),
		Workers:   input.Workers,
	}
	query = query.WithInstrumentLookup(input.Symbol)
	if input.From != "" && input.From == input.To {
		query.BasDt = input.From
	} else {
		if input.From != "" {
			query.BeginBasDt = input.From
		}
		if input.To != "" {
			endBasDt, err := exclusiveEndBasDt(input.To)
			if err != nil {
				return dailybar.FetchResult{}, inputErrb.Wrap(err)
			}
			query.EndBasDt = endBasDt
		}
	}

	fetchAllPages := input.Limit <= 0
	result, err := p.fetchPriceRecords(ctx, operationSpec{SecurityType: input.SecurityType, Operation: operation}, query, fetchAllPages)
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
		Group:      group,
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
	providerErrb := oops.In("datago_adapter").With("provider", provider.ProviderDataGo)

	instruments := make([]instrument.Instrument, 0)
	totalCount := 0
	for _, spec := range operations {
		query := datagoetp.SecuritiesProductPriceQuery{
			NumOfRows: numOfRowsForSearch(input.Limit),
		}
		query = query.WithInstrumentSearch(input.Query)

		result, err := p.fetchPriceRecords(ctx, spec, query, false)
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
					Group:       groupForOperation(spec.Operation),
					Operations:  operationIDs(operations),
					TotalCount:  totalCount,
				}, nil
			}
		}
	}

	return instrument.SearchResult{
		Instruments: instruments,
		Provider:    p.Identity,
		Group:       groupForOperations(operations),
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
		{SecurityType: provider.SecurityTypeStock, Operation: provider.OperationGetStockPriceInfo},
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
	case provider.SecurityTypeStock:
		return provider.OperationGetStockPriceInfo, nil
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
			GroupID:      groupForSecurityType(securityType),
			Market:       provider.MarketKRX,
			SecurityType: securityType,
			Symbol:       symbol,
			Reason:       "security_type is not supported by datago",
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
		GroupID:      groupForSecurityType(securityType),
		Market:       market,
		SecurityType: securityType,
		Symbol:       symbol,
		Reason:       "market is not supported by datago",
	})
}

func numOfRowsForDailyFetch(limit int) int {
	if limit > 0 && limit < datagoetp.DefaultAllNumOfRows {
		return limit
	}
	return datagoetp.DefaultAllNumOfRows
}

func numOfRowsForSearch(limit int) int {
	if limit > 0 && limit < datagoetp.DefaultNumOfRows {
		return limit
	}
	return datagoetp.DefaultNumOfRows
}

func exclusiveEndBasDt(value string) (string, error) {
	parsed, err := time.Parse("20060102", value)
	if err != nil {
		return "", oops.In("datago_adapter").With("end_bas_dt", value).Wrapf(err, "parse datago exclusive endBasDt")
	}
	return parsed.AddDate(0, 0, 1).Format("20060102"), nil
}

func operationIDs(specs []operationSpec) []provider.OperationID {
	operations := make([]provider.OperationID, 0, len(specs))
	for _, spec := range specs {
		operations = append(operations, spec.Operation)
	}
	return operations
}

func groupForOperations(specs []operationSpec) provider.GroupID {
	if len(specs) == 0 {
		return ""
	}
	group := groupForOperation(specs[0].Operation)
	for _, spec := range specs[1:] {
		if groupForOperation(spec.Operation) != group {
			return ""
		}
	}
	return group
}

func groupForOperation(operation provider.OperationID) provider.GroupID {
	switch operation {
	case provider.OperationGetStockPriceInfo:
		return provider.GroupStockPrice
	default:
		return provider.GroupSecuritiesProductPrice
	}
}

func groupForSecurityType(securityType provider.SecurityType) provider.GroupID {
	if securityType == provider.SecurityTypeStock {
		return provider.GroupStockPrice
	}
	return provider.GroupSecuritiesProductPrice
}

func (p *Provider) fetchPriceRecords(ctx context.Context, spec operationSpec, query datagoetp.SecuritiesProductPriceQuery, allPages bool) (priceRecordsResult, error) {
	errb := oops.In("datago_adapter").With(
		"provider", provider.ProviderDataGo,
		"group", groupForOperation(spec.Operation),
		"operation", spec.Operation,
		"security_type", spec.SecurityType,
	)

	switch spec.Operation {
	case provider.OperationGetETFPriceInfo:
		if p.etpClient == nil {
			return priceRecordsResult{}, errb.New("datago securitiesProductPrice adapter client is nil")
		}
		query := datagoetp.ETFPriceInfoQuery{
			SecuritiesProductPriceQuery: query,
		}
		var result datagoetp.ETFPriceInfoResult
		var err error
		if allPages {
			result, err = p.etpClient.GetAllETFPriceInfo(ctx, query)
		} else {
			result, err = p.etpClient.GetETFPriceInfo(ctx, query)
		}
		if err != nil {
			return priceRecordsResult{}, errb.Wrap(err)
		}
		return priceRecordsResult{Records: recordsFromETF(result.Items), TotalCount: result.TotalCount}, nil
	case provider.OperationGetETNPriceInfo:
		if p.etpClient == nil {
			return priceRecordsResult{}, errb.New("datago securitiesProductPrice adapter client is nil")
		}
		query := datagoetp.ETNPriceInfoQuery{
			SecuritiesProductPriceQuery: query,
		}
		var result datagoetp.ETNPriceInfoResult
		var err error
		if allPages {
			result, err = p.etpClient.GetAllETNPriceInfo(ctx, query)
		} else {
			result, err = p.etpClient.GetETNPriceInfo(ctx, query)
		}
		if err != nil {
			return priceRecordsResult{}, errb.Wrap(err)
		}
		return priceRecordsResult{Records: recordsFromETN(result.Items), TotalCount: result.TotalCount}, nil
	case provider.OperationGetELWPriceInfo:
		if p.etpClient == nil {
			return priceRecordsResult{}, errb.New("datago securitiesProductPrice adapter client is nil")
		}
		query := datagoetp.ELWPriceInfoQuery{
			SecuritiesProductPriceQuery: query,
		}
		var result datagoetp.ELWPriceInfoResult
		var err error
		if allPages {
			result, err = p.etpClient.GetAllELWPriceInfo(ctx, query)
		} else {
			result, err = p.etpClient.GetELWPriceInfo(ctx, query)
		}
		if err != nil {
			return priceRecordsResult{}, errb.Wrap(err)
		}
		return priceRecordsResult{Records: recordsFromELW(result.Items), TotalCount: result.TotalCount}, nil
	case provider.OperationGetStockPriceInfo:
		if p.stockClient == nil {
			return priceRecordsResult{}, errb.New("datago stockPrice adapter client is nil")
		}
		stockQuery := stockQueryFromSecuritiesProductQuery(query)
		var result datagostock.StockPriceInfoResult
		var err error
		if allPages {
			result, err = p.stockClient.GetAllStockPriceInfo(ctx, stockQuery)
		} else {
			result, err = p.stockClient.GetStockPriceInfo(ctx, stockQuery)
		}
		if err != nil {
			return priceRecordsResult{}, errb.Wrap(err)
		}
		return priceRecordsResult{Records: recordsFromStock(result.Items), TotalCount: result.TotalCount}, nil
	default:
		return priceRecordsResult{}, errb.New("unsupported datago price info operation")
	}
}

func stockQueryFromSecuritiesProductQuery(query datagoetp.SecuritiesProductPriceQuery) datagostock.StockPriceInfoQuery {
	return datagostock.StockPriceInfoQuery{
		NumOfRows:       query.NumOfRows,
		PageNo:          query.PageNo,
		Workers:         query.Workers,
		BasDt:           query.BasDt,
		BeginBasDt:      query.BeginBasDt,
		EndBasDt:        query.EndBasDt,
		LikeBasDt:       query.LikeBasDt,
		LikeSrtnCd:      query.LikeSrtnCd,
		IsinCd:          query.IsinCd,
		LikeIsinCd:      query.LikeIsinCd,
		ItmsNm:          query.ItmsNm,
		LikeItmsNm:      query.LikeItmsNm,
		BeginVs:         query.BeginVs,
		EndVs:           query.EndVs,
		BeginTrqu:       query.BeginTrqu,
		EndTrqu:         query.EndTrqu,
		BeginTrPrc:      query.BeginTrPrc,
		EndTrPrc:        query.EndTrPrc,
		BeginMrktTotAmt: query.BeginMrktTotAmt,
		EndMrktTotAmt:   query.EndMrktTotAmt,
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

func recordsFromStock(items []datagostock.StockPriceInfo) []priceRecord {
	records := make([]priceRecord, 0, len(items))
	for _, item := range items {
		records = append(records, priceRecord{
			Common: datagoetp.CommonPriceInfo{
				BasDt:      item.BasDt,
				SrtnCd:     item.SrtnCd,
				IsinCd:     item.IsinCd,
				ItmsNm:     item.ItmsNm,
				Clpr:       item.Clpr,
				Vs:         item.Vs,
				FltRt:      item.FltRt,
				Mkp:        item.Mkp,
				Hipr:       item.Hipr,
				Lopr:       item.Lopr,
				Trqu:       item.Trqu,
				TrPrc:      item.TrPrc,
				MrktTotAmt: item.MrktTotAmt,
			},
			Fields: item.Fields(),
		})
	}
	return records
}

func normalizeDailyBar(record priceRecord, securityType provider.SecurityType, operation provider.OperationID) dailybar.Bar {
	item := record.Common
	return dailybar.Bar{
		Provider:     provider.ProviderDataGo,
		Group:        groupForOperation(operation),
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
		Group:        groupForOperation(operation),
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
