package datago

import (
	"context"
	"fmt"
	"strings"
	"time"

	datagocorpfin "github.com/ev3rlit/mwosa/clients/datago-corpfin"
	datagoetp "github.com/ev3rlit/mwosa/clients/datago-etp"
	datagokrxlisted "github.com/ev3rlit/mwosa/clients/datago-krxlisted"
	provider "github.com/ev3rlit/mwosa/providers/core"
	"github.com/ev3rlit/mwosa/providers/core/dailybar"
	"github.com/ev3rlit/mwosa/providers/core/financials"
	"github.com/ev3rlit/mwosa/providers/core/instrument"
	"github.com/samber/oops"
)

type Config struct {
	ServiceKey       string
	BaseURL          string
	RetryMaxAttempts int
}

type CorporateFinanceConfig struct {
	ServiceKey              string
	CorporateFinanceBaseURL string
	KRXListedInfoServiceKey string
	KRXListedInfoBaseURL    string
	RetryMaxAttempts        int
}

type priceClient interface {
	GetETFPriceInfo(context.Context, datagoetp.ETFPriceInfoQuery) (datagoetp.ETFPriceInfoResult, error)
	GetAllETFPriceInfo(context.Context, datagoetp.ETFPriceInfoQuery) (datagoetp.ETFPriceInfoResult, error)
	GetETNPriceInfo(context.Context, datagoetp.ETNPriceInfoQuery) (datagoetp.ETNPriceInfoResult, error)
	GetAllETNPriceInfo(context.Context, datagoetp.ETNPriceInfoQuery) (datagoetp.ETNPriceInfoResult, error)
	GetELWPriceInfo(context.Context, datagoetp.ELWPriceInfoQuery) (datagoetp.ELWPriceInfoResult, error)
	GetAllELWPriceInfo(context.Context, datagoetp.ELWPriceInfoQuery) (datagoetp.ELWPriceInfoResult, error)
}

type financialClient interface {
	GetAllSummaryFinancialStatements(context.Context, datagocorpfin.Query) (datagocorpfin.SummaryFinancialStatementResult, error)
	GetAllBalanceSheets(context.Context, datagocorpfin.Query) (datagocorpfin.BalanceSheetResult, error)
	GetAllIncomeStatements(context.Context, datagocorpfin.Query) (datagocorpfin.IncomeStatementResult, error)
}

type listedClient interface {
	GetItemInfo(context.Context, datagokrxlisted.Query) (datagokrxlisted.ItemInfoResult, error)
}

type Provider struct {
	provider.Identity

	dailybar.Fetcher
	instrument.Searcher

	client priceClient
	groups []provider.GroupRoleProvider
}

type CorporateFinanceProvider struct {
	provider.Identity

	financials.Fetch

	financialClient financialClient
	listedClient    listedClient
	groups          []provider.GroupRoleProvider
}

func New(config Config) (*Provider, error) {
	errb := oops.In("datago_adapter").With("provider", provider.ProviderDataGo)
	priceClient, err := datagoetp.New(datagoetp.Config{
		ServiceKey:       config.ServiceKey,
		BaseURL:          config.BaseURL,
		RetryMaxAttempts: config.RetryMaxAttempts,
	})
	if err != nil {
		return nil, errb.With("group", provider.GroupSecuritiesProductPrice).Wrap(err)
	}
	return NewWithClient(priceClient), nil
}

func NewWithClient(client priceClient) *Provider {
	p := &Provider{
		Identity: provider.Identity{
			ID:          provider.ProviderDataGo,
			DisplayName: "공공데이터포털",
		},
		client: client,
	}

	group := newSecuritiesProductPriceGroup(p.fetchDailyBars, p.searchInstruments)
	p.Fetcher = group.Fetcher
	p.Searcher = group.Searcher
	p.groups = []provider.GroupRoleProvider{group}
	return p
}

func NewCorporateFinance(config CorporateFinanceConfig) (*CorporateFinanceProvider, error) {
	errb := oops.In("datago_adapter").With("provider", provider.ProviderDataGoCorporateFinance)
	financeClient, err := datagocorpfin.New(datagocorpfin.Config{
		ServiceKey:       config.ServiceKey,
		BaseURL:          config.CorporateFinanceBaseURL,
		RetryMaxAttempts: config.RetryMaxAttempts,
	})
	if err != nil {
		return nil, errb.With("group", provider.GroupCorporateFinance).Wrap(err)
	}
	listedClient, err := datagokrxlisted.New(datagokrxlisted.Config{
		ServiceKey:       config.KRXListedInfoServiceKey,
		BaseURL:          config.KRXListedInfoBaseURL,
		RetryMaxAttempts: config.RetryMaxAttempts,
	})
	if err != nil {
		return nil, errb.With("group", provider.GroupKRXListedInfo).Wrap(err)
	}
	return NewCorporateFinanceWithClients(financeClient, listedClient), nil
}

func NewCorporateFinanceWithClients(financialClient financialClient, listedClient listedClient) *CorporateFinanceProvider {
	p := &CorporateFinanceProvider{
		Identity: provider.Identity{
			ID:          provider.ProviderDataGoCorporateFinance,
			DisplayName: "공공데이터포털 기업재무정보",
		},
		financialClient: financialClient,
		listedClient:    listedClient,
	}
	group := newCorporateFinanceGroup(p.fetchFinancialStatements)
	p.Fetch = group.Fetch
	p.groups = []provider.GroupRoleProvider{group}
	return p
}

func Register(registry *provider.Registry, p provider.IdentityProvider) error {
	return registry.RegisterProvider(p)
}

func (p *Provider) RoleRegistrations() []provider.RoleRegistration {
	if p == nil {
		return nil
	}
	return roleRegistrationsFromGroups(p.groups)
}

func (p *CorporateFinanceProvider) RoleRegistrations() []provider.RoleRegistration {
	if p == nil {
		return nil
	}
	return roleRegistrationsFromGroups(p.groups)
}

func roleRegistrationsFromGroups(groups []provider.GroupRoleProvider) []provider.RoleRegistration {
	registrations := make([]provider.RoleRegistration, 0)
	for _, group := range groups {
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
	providerErrb := oops.In("datago_adapter").With("provider", provider.ProviderDataGo, "group", provider.GroupSecuritiesProductPrice)
	if p.client == nil {
		return dailybar.FetchResult{}, providerErrb.New("datago adapter client is nil")
	}

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

func (p *CorporateFinanceProvider) fetchFinancialStatements(ctx context.Context, input financials.FetchInput) (financials.FetchResult, error) {
	inputErrb := oops.In("datago_adapter").With("role", provider.RoleFinancials, "market", input.Market, "security_type", input.SecurityType, "symbol", input.Symbol, "fiscal_year", input.FiscalYear, "statement", input.Statement)
	if err := validateFinancialStatementInput(input); err != nil {
		return financials.FetchResult{}, inputErrb.Wrap(err)
	}
	if p.financialClient == nil {
		return financials.FetchResult{}, oops.In("datago_adapter").With("provider", provider.ProviderDataGoCorporateFinance, "group", provider.GroupCorporateFinance).New("datago corporate finance adapter financial client is nil")
	}
	target, err := p.resolveFinancialTarget(ctx, input.Symbol)
	if err != nil {
		return financials.FetchResult{}, inputErrb.Wrap(err)
	}

	query := datagocorpfin.Query{
		Crno:    target.Crno,
		BizYear: strings.TrimSpace(input.FiscalYear),
	}
	if input.Limit > 0 && input.Limit < datagocorpfin.DefaultAllNumOfRows {
		query.NumOfRows = input.Limit
	}

	statements := make([]financials.Statement, 0)
	totalCount := 0
	operations, err := financialOperations(input.Statement)
	if err != nil {
		return financials.FetchResult{}, inputErrb.Wrap(err)
	}
	for _, operation := range operations {
		result, err := p.fetchFinancialOperation(ctx, operation, query, input, target)
		if err != nil {
			return financials.FetchResult{}, inputErrb.With("operation", operation).Wrapf(err, "fetch datago financial statements")
		}
		statements = append(statements, result.Statements...)
		totalCount += result.TotalCount
	}

	return financials.FetchResult{
		Statements: statements,
		Provider:   p.Identity,
		Group:      provider.GroupCorporateFinance,
		TotalCount: totalCount,
	}, nil
}

type operationSpec struct {
	SecurityType provider.SecurityType
	Operation    provider.OperationID
}

type financialOperationResult struct {
	Statements []financials.Statement
	TotalCount int
}

type financialTarget struct {
	Input          string
	Crno           string
	SrtnCd         string
	IsinCd         string
	ItemName       string
	CorpName       string
	BasDt          string
	MarketCategory string
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
			ProviderID: provider.ProviderDataGoCorporateFinance,
			GroupID:    provider.GroupSecuritiesProductPrice,
			Market:     provider.MarketKRX,
			Symbol:     symbol,
			Reason:     "security_type is required for daily bars",
		})
	default:
		return "", provider.NewUnsupported(provider.UnsupportedError{
			Capability:   capability,
			ProviderID:   provider.ProviderDataGoCorporateFinance,
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
		ProviderID:   provider.ProviderDataGoCorporateFinance,
		GroupID:      provider.GroupSecuritiesProductPrice,
		Market:       market,
		SecurityType: securityType,
		Symbol:       symbol,
		Reason:       "market is not supported by datago securitiesProductPrice",
	})
}

func validateFinancialStatementInput(input financials.FetchInput) error {
	market := input.Market
	if market == "" {
		market = provider.MarketKRX
	}
	if market != provider.MarketKRX {
		return provider.NewUnsupported(provider.UnsupportedError{
			Capability:   provider.RoleFinancials,
			ProviderID:   provider.ProviderDataGoCorporateFinance,
			GroupID:      provider.GroupCorporateFinance,
			Market:       input.Market,
			SecurityType: input.SecurityType,
			Symbol:       input.Symbol,
			Reason:       "market is not supported by datago corporateFinance",
		})
	}
	if input.SecurityType != "" && input.SecurityType != provider.SecurityTypeStock {
		return provider.NewUnsupported(provider.UnsupportedError{
			Capability:   provider.RoleFinancials,
			ProviderID:   provider.ProviderDataGoCorporateFinance,
			GroupID:      provider.GroupCorporateFinance,
			Market:       market,
			SecurityType: input.SecurityType,
			Symbol:       input.Symbol,
			Reason:       "security_type is not supported by datago corporateFinance",
		})
	}
	if input.Statement == financials.StatementTypeCashFlow {
		return provider.NewUnsupported(provider.UnsupportedError{
			Capability:   provider.RoleFinancials,
			ProviderID:   provider.ProviderDataGoCorporateFinance,
			GroupID:      provider.GroupCorporateFinance,
			Market:       market,
			SecurityType: provider.SecurityTypeStock,
			Symbol:       input.Symbol,
			Reason:       "cash flow statements are not supported by datago corporateFinance",
		})
	}
	return nil
}

func financialOperations(statement financials.StatementType) ([]provider.OperationID, error) {
	switch statement {
	case "":
		return []provider.OperationID{
			provider.OperationGetSummFinaStatV2,
			provider.OperationGetBalanceSheetV2,
			provider.OperationGetIncomeStatementV2,
		}, nil
	case financials.StatementTypeSummary:
		return []provider.OperationID{provider.OperationGetSummFinaStatV2}, nil
	case financials.StatementTypeBalanceSheet:
		return []provider.OperationID{provider.OperationGetBalanceSheetV2}, nil
	case financials.StatementTypeIncomeStatement:
		return []provider.OperationID{provider.OperationGetIncomeStatementV2}, nil
	case financials.StatementTypeCashFlow:
		return nil, provider.NewUnsupported(provider.UnsupportedError{
			Capability:  provider.RoleFinancials,
			ProviderID:  provider.ProviderDataGoCorporateFinance,
			GroupID:     provider.GroupCorporateFinance,
			OperationID: provider.OperationID(statement),
			Reason:      "cash flow statements are not supported by datago corporateFinance",
		})
	default:
		return nil, provider.NewUnsupported(provider.UnsupportedError{
			Capability:  provider.RoleFinancials,
			ProviderID:  provider.ProviderDataGoCorporateFinance,
			GroupID:     provider.GroupCorporateFinance,
			OperationID: provider.OperationID(statement),
			Reason:      "financial statement type is not supported by datago corporateFinance",
		})
	}
}

func looksLikeCorporateRegistrationNumber(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 13 {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func looksLikeISIN(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 12 {
		return false
	}
	for _, r := range value {
		if !((r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')) {
			return false
		}
	}
	return true
}

func looksLikeShortCode(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) == 0 || len(value) >= 13 {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (p *CorporateFinanceProvider) resolveFinancialTarget(ctx context.Context, symbol string) (financialTarget, error) {
	symbol = strings.TrimSpace(symbol)
	if looksLikeCorporateRegistrationNumber(symbol) {
		return financialTarget{Input: symbol, Crno: symbol}, nil
	}
	if p.listedClient == nil {
		return financialTarget{}, provider.NewUnsupported(provider.UnsupportedError{
			Capability:   provider.RoleFinancials,
			ProviderID:   provider.ProviderDataGoCorporateFinance,
			GroupID:      provider.GroupKRXListedInfo,
			Market:       provider.MarketKRX,
			SecurityType: provider.SecurityTypeStock,
			Symbol:       symbol,
			Reason:       "datago financials requires a 13-digit corporation registration number or krxListedInfo symbol resolution",
		})
	}

	queries := listedInfoQueriesForSymbol(symbol)
	var item datagokrxlisted.ListedItem
	found := false
	for _, query := range queries {
		result, err := p.listedClient.GetItemInfo(ctx, query.Query)
		if err != nil {
			return financialTarget{}, oops.In("datago_adapter").With("provider", provider.ProviderDataGoCorporateFinance, "group", provider.GroupKRXListedInfo, "operation", provider.OperationGetItemInfo, "symbol", symbol, "resolver_strategy", query.Strategy).Wrapf(err, "resolve datago financial symbol")
		}
		selected, ok, err := selectListedItem(symbol, result.Items)
		if err != nil {
			return financialTarget{}, err
		}
		if ok {
			item = selected
			found = true
			break
		}
	}
	if !found {
		return financialTarget{}, oops.In("datago_adapter").With("provider", provider.ProviderDataGoCorporateFinance, "group", provider.GroupKRXListedInfo, "operation", provider.OperationGetItemInfo, "symbol", symbol).New("datago krxListedInfo returned no items")
	}
	if !looksLikeCorporateRegistrationNumber(item.Crno) {
		return financialTarget{}, provider.NewUnsupported(provider.UnsupportedError{
			Capability:   provider.RoleFinancials,
			ProviderID:   provider.ProviderDataGoCorporateFinance,
			GroupID:      provider.GroupKRXListedInfo,
			Market:       provider.MarketKRX,
			SecurityType: provider.SecurityTypeStock,
			Symbol:       symbol,
			Reason:       "datago krxListedInfo did not return a domestic corporation registration number for this symbol",
		})
	}
	return financialTarget{
		Input:          symbol,
		Crno:           item.Crno,
		SrtnCd:         item.SrtnCd,
		IsinCd:         item.IsinCd,
		ItemName:       item.ItmsNm,
		CorpName:       item.CorpNm,
		BasDt:          item.BasDt,
		MarketCategory: item.MrktCtg,
	}, nil
}

type listedInfoQuery struct {
	Strategy string
	Query    datagokrxlisted.Query
}

func listedInfoQueriesForSymbol(symbol string) []listedInfoQuery {
	queries := []listedInfoQuery{
		{
			Strategy: "item_name",
			Query: datagokrxlisted.Query{
				NumOfRows: 10,
				ItmsNm:    symbol,
			},
		},
	}
	if looksLikeShortCode(symbol) {
		queries = append(queries, listedInfoQuery{
			Strategy: "short_code",
			Query: datagokrxlisted.Query{
				NumOfRows:  10,
				LikeSrtnCd: symbol,
			},
		})
	}
	if looksLikeISIN(symbol) {
		queries = append(queries, listedInfoQuery{
			Strategy: "isin",
			Query: datagokrxlisted.Query{
				NumOfRows: 10,
				IsinCd:    strings.ToUpper(symbol),
			},
		})
	}
	return queries
}

func selectListedItem(symbol string, items []datagokrxlisted.ListedItem) (datagokrxlisted.ListedItem, bool, error) {
	if len(items) == 0 {
		return datagokrxlisted.ListedItem{}, false, nil
	}
	exact := make([]datagokrxlisted.ListedItem, 0, 1)
	for _, item := range items {
		if listedItemMatches(symbol, item) {
			exact = append(exact, item)
		}
	}
	if len(exact) == 1 {
		return exact[0], true, nil
	}
	if len(exact) > 1 {
		return datagokrxlisted.ListedItem{}, false, oops.In("datago_adapter").With("provider", provider.ProviderDataGoCorporateFinance, "group", provider.GroupKRXListedInfo, "operation", provider.OperationGetItemInfo, "symbol", symbol, "matches", len(exact)).New("datago krxListedInfo returned multiple exact matches")
	}
	if len(items) == 1 {
		return items[0], true, nil
	}
	return datagokrxlisted.ListedItem{}, false, oops.In("datago_adapter").With("provider", provider.ProviderDataGoCorporateFinance, "group", provider.GroupKRXListedInfo, "operation", provider.OperationGetItemInfo, "symbol", symbol, "matches", len(items)).New("datago krxListedInfo returned multiple candidate items")
}

func listedItemMatches(symbol string, item datagokrxlisted.ListedItem) bool {
	symbol = strings.TrimSpace(symbol)
	return strings.EqualFold(symbol, strings.TrimSpace(item.SrtnCd)) ||
		strings.EqualFold(symbol, strings.TrimSpace(item.IsinCd)) ||
		strings.EqualFold(symbol, strings.TrimSpace(item.ItmsNm)) ||
		strings.EqualFold(symbol, strings.TrimSpace(item.CorpNm)) ||
		strings.EqualFold(symbol, strings.TrimSpace(item.Crno))
}

func numOfRowsForDailyFetch(limit int) int {
	if limit > 0 && limit < datagoetp.DefaultAllNumOfRows {
		return limit
	}
	return datagoetp.DefaultAllNumOfRows
}

func withDefaultFinancialPeriod(period financials.PeriodType) financials.PeriodType {
	if period == "" {
		return financials.PeriodTypeAnnual
	}
	return period
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

func (p *Provider) fetchPriceRecords(ctx context.Context, spec operationSpec, query datagoetp.SecuritiesProductPriceQuery, allPages bool) (priceRecordsResult, error) {
	errb := oops.In("datago_adapter").With(
		"provider", provider.ProviderDataGo,
		"group", provider.GroupSecuritiesProductPrice,
		"operation", spec.Operation,
		"security_type", spec.SecurityType,
	)

	switch spec.Operation {
	case provider.OperationGetETFPriceInfo:
		query := datagoetp.ETFPriceInfoQuery{
			SecuritiesProductPriceQuery: query,
		}
		var result datagoetp.ETFPriceInfoResult
		var err error
		if allPages {
			result, err = p.client.GetAllETFPriceInfo(ctx, query)
		} else {
			result, err = p.client.GetETFPriceInfo(ctx, query)
		}
		if err != nil {
			return priceRecordsResult{}, errb.Wrap(err)
		}
		return priceRecordsResult{Records: recordsFromETF(result.Items), TotalCount: result.TotalCount}, nil
	case provider.OperationGetETNPriceInfo:
		query := datagoetp.ETNPriceInfoQuery{
			SecuritiesProductPriceQuery: query,
		}
		var result datagoetp.ETNPriceInfoResult
		var err error
		if allPages {
			result, err = p.client.GetAllETNPriceInfo(ctx, query)
		} else {
			result, err = p.client.GetETNPriceInfo(ctx, query)
		}
		if err != nil {
			return priceRecordsResult{}, errb.Wrap(err)
		}
		return priceRecordsResult{Records: recordsFromETN(result.Items), TotalCount: result.TotalCount}, nil
	case provider.OperationGetELWPriceInfo:
		query := datagoetp.ELWPriceInfoQuery{
			SecuritiesProductPriceQuery: query,
		}
		var result datagoetp.ELWPriceInfoResult
		var err error
		if allPages {
			result, err = p.client.GetAllELWPriceInfo(ctx, query)
		} else {
			result, err = p.client.GetELWPriceInfo(ctx, query)
		}
		if err != nil {
			return priceRecordsResult{}, errb.Wrap(err)
		}
		return priceRecordsResult{Records: recordsFromELW(result.Items), TotalCount: result.TotalCount}, nil
	default:
		return priceRecordsResult{}, errb.New("unsupported datago price info operation")
	}
}

func (p *CorporateFinanceProvider) fetchFinancialOperation(ctx context.Context, operation provider.OperationID, query datagocorpfin.Query, input financials.FetchInput, target financialTarget) (financialOperationResult, error) {
	errb := oops.In("datago_adapter").With(
		"provider", provider.ProviderDataGoCorporateFinance,
		"group", provider.GroupCorporateFinance,
		"operation", operation,
	)

	switch operation {
	case provider.OperationGetSummFinaStatV2:
		result, err := p.financialClient.GetAllSummaryFinancialStatements(ctx, query)
		if err != nil {
			return financialOperationResult{}, errb.Wrap(err)
		}
		return financialOperationResult{
			Statements: []financials.Statement{enrichFinancialStatement(normalizeSummaryFinancialStatement(result.Items, input, operation), target)},
			TotalCount: result.TotalCount,
		}, nil
	case provider.OperationGetBalanceSheetV2:
		result, err := p.financialClient.GetAllBalanceSheets(ctx, query)
		if err != nil {
			return financialOperationResult{}, errb.Wrap(err)
		}
		return financialOperationResult{
			Statements: []financials.Statement{enrichFinancialStatement(normalizeAccountStatement(recordsFromBalanceSheets(result.Items), financials.StatementTypeBalanceSheet, input, operation), target)},
			TotalCount: result.TotalCount,
		}, nil
	case provider.OperationGetIncomeStatementV2:
		result, err := p.financialClient.GetAllIncomeStatements(ctx, query)
		if err != nil {
			return financialOperationResult{}, errb.Wrap(err)
		}
		return financialOperationResult{
			Statements: []financials.Statement{enrichFinancialStatement(normalizeAccountStatement(recordsFromIncomeStatements(result.Items), financials.StatementTypeIncomeStatement, input, operation), target)},
			TotalCount: result.TotalCount,
		}, nil
	default:
		return financialOperationResult{}, errb.New("unsupported datago financial statement operation")
	}
}

func enrichFinancialStatement(statement financials.Statement, target financialTarget) financials.Statement {
	if target.Crno != "" {
		statement.Symbol = target.Crno
	}
	if statement.Name == "" {
		statement.Name = target.displayName()
	}
	extensions := extensionFieldsForFinancialTarget(target)
	if len(extensions) == 0 {
		return statement
	}
	if statement.Extensions == nil {
		statement.Extensions = make(map[string]string, len(extensions))
	}
	for key, value := range extensions {
		if _, exists := statement.Extensions[key]; !exists {
			statement.Extensions[key] = value
		}
	}
	return statement
}

func (t financialTarget) displayName() string {
	if strings.TrimSpace(t.CorpName) != "" {
		return strings.TrimSpace(t.CorpName)
	}
	return strings.TrimSpace(t.ItemName)
}

func extensionFieldsForFinancialTarget(target financialTarget) map[string]string {
	values := map[string]string{
		"request_symbol": target.Input,
		"resolved_crno":  target.Crno,
		"srtnCd":         target.SrtnCd,
		"isinCd":         target.IsinCd,
		"itmsNm":         target.ItemName,
		"corpNm":         target.CorpName,
		"listedBasDt":    normalizeDate(target.BasDt),
		"marketCategory": target.MarketCategory,
	}
	if target.hasListedInfo() {
		values["resolver_group"] = string(provider.GroupKRXListedInfo)
		values["resolver_source"] = string(provider.OperationGetItemInfo)
	}
	extensions := make(map[string]string, len(values))
	for key, value := range values {
		if strings.TrimSpace(value) != "" {
			extensions[key] = value
		}
	}
	return extensions
}

func (t financialTarget) hasListedInfo() bool {
	return t.SrtnCd != "" || t.IsinCd != "" || t.ItemName != "" || t.CorpName != "" || t.BasDt != "" || t.MarketCategory != ""
}

func recordsFromETF(items []datagoetp.ETFPriceInfo) []priceRecord {
	records := make([]priceRecord, 0, len(items))
	for _, item := range items {
		records = append(records, priceRecord{Common: item.CommonPriceInfo, Fields: item.Fields()})
	}
	return records
}

func recordsFromBalanceSheets(items []datagocorpfin.BalanceSheetItem) []datagocorpfin.AccountStatementItem {
	records := make([]datagocorpfin.AccountStatementItem, 0, len(items))
	for _, item := range items {
		records = append(records, item.AccountStatementItem)
	}
	return records
}

func recordsFromIncomeStatements(items []datagocorpfin.IncomeStatementItem) []datagocorpfin.AccountStatementItem {
	records := make([]datagocorpfin.AccountStatementItem, 0, len(items))
	for _, item := range items {
		records = append(records, item.AccountStatementItem)
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
		Provider:     provider.ProviderDataGoCorporateFinance,
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

func normalizeSummaryFinancialStatement(items []datagocorpfin.SummaryFinancialStatement, input financials.FetchInput, operation provider.OperationID) financials.Statement {
	statement := financials.Statement{
		Statement:    financials.StatementTypeSummary,
		Symbol:       strings.TrimSpace(input.Symbol),
		FiscalYear:   strings.TrimSpace(input.FiscalYear),
		Period:       withDefaultFinancialPeriod(input.Period),
		Provider:     provider.ProviderDataGoCorporateFinance,
		Group:        provider.GroupCorporateFinance,
		Operation:    operation,
		Market:       provider.MarketKRX,
		SecurityType: provider.SecurityTypeStock,
		Lines:        make([]financials.LineItem, 0),
	}
	if len(items) == 0 {
		return statement
	}
	item := items[0]
	statement.Symbol = item.Crno
	statement.FiscalYear = item.BizYear
	statement.ReportedAt = normalizeDate(item.BasDt)
	statement.Currency = item.CurCd
	statement.Extensions = extensionFieldsExcept(item.Fields(),
		"basDt", "bizYear", "crno", "curCd",
		"enpBzopPft", "enpCptlAmt", "enpCrtmNpf", "enpSaleAmt", "enpTastAmt", "enpTdbtAmt", "enpTcptAmt", "fnclDebtRto", "iclsPalClcAmt",
	)
	for _, line := range []financials.LineItem{
		summaryLine("enpSaleAmt", "Revenue", item.EnpSaleAmt, item.CurCd),
		summaryLine("enpBzopPft", "Operating profit", item.EnpBzopPft, item.CurCd),
		summaryLine("enpCrtmNpf", "Net profit", item.EnpCrtmNpf, item.CurCd),
		summaryLine("enpTastAmt", "Total assets", item.EnpTastAmt, item.CurCd),
		summaryLine("enpTdbtAmt", "Total liabilities", item.EnpTdbtAmt, item.CurCd),
		summaryLine("enpTcptAmt", "Total equity", item.EnpTcptAmt, item.CurCd),
		summaryLine("enpCptlAmt", "Capital stock", item.EnpCptlAmt, item.CurCd),
		summaryLine("fnclDebtRto", "Debt ratio", item.FnclDebtRto, ""),
		summaryLine("iclsPalClcAmt", "Profit before income tax", item.IclsPalClcAmt, item.CurCd),
	} {
		if line.Value != "" {
			statement.Lines = append(statement.Lines, line)
		}
	}
	return statement
}

func summaryLine(id string, name string, value string, currency string) financials.LineItem {
	return financials.LineItem{
		AccountID:   id,
		AccountName: name,
		Value:       value,
		Currency:    currency,
	}
}

func normalizeAccountStatement(items []datagocorpfin.AccountStatementItem, statementType financials.StatementType, input financials.FetchInput, operation provider.OperationID) financials.Statement {
	statement := financials.Statement{
		Statement:    statementType,
		Symbol:       strings.TrimSpace(input.Symbol),
		FiscalYear:   strings.TrimSpace(input.FiscalYear),
		Period:       withDefaultFinancialPeriod(input.Period),
		Provider:     provider.ProviderDataGoCorporateFinance,
		Group:        provider.GroupCorporateFinance,
		Operation:    operation,
		Market:       provider.MarketKRX,
		SecurityType: provider.SecurityTypeStock,
		Lines:        make([]financials.LineItem, 0, len(items)),
	}
	if len(items) == 0 {
		return statement
	}
	first := items[0]
	statement.Symbol = first.Crno
	statement.FiscalYear = first.BizYear
	statement.ReportedAt = normalizeDate(first.BasDt)
	statement.Currency = first.CurCd
	statement.Extensions = extensionFieldsExcept(first.Fields(), "acitId", "acitNm", "basDt", "bizYear", "crno", "crtmAcitAmt", "curCd", "thqrAcitAmt")
	for _, item := range items {
		line := financials.LineItem{
			AccountID:   item.AcitID,
			AccountName: item.AcitNm,
			Value:       amountForPeriod(item, statement.Period),
			Currency:    item.CurCd,
			Extensions:  extensionFieldsExcept(item.Fields(), "acitId", "acitNm", "curCd"),
		}
		if line.Value == "" {
			line.Value = item.CrtmAcitAmt
		}
		statement.Lines = append(statement.Lines, line)
	}
	return statement
}

func amountForPeriod(item datagocorpfin.AccountStatementItem, period financials.PeriodType) string {
	switch period {
	case financials.PeriodTypeQuarter:
		return item.ThqrAcitAmt
	default:
		return item.CrtmAcitAmt
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

func extensionFieldsExcept(item map[string]string, excluded ...string) map[string]string {
	excludedSet := make(map[string]struct{}, len(excluded))
	for _, key := range excluded {
		excludedSet[key] = struct{}{}
	}
	extensions := make(map[string]string)
	for key, value := range item {
		if _, ok := excludedSet[key]; ok {
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
