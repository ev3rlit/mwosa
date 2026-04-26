package core

type ProviderID string
type GroupID string
type OperationID string
type Market string
type SecurityType string
type CredentialScope string
type Freshness string
type Role string

const (
	ProviderDataGo ProviderID = "datago"

	GroupSecuritiesProductPrice GroupID = "securitiesProductPrice"

	OperationGetETFPriceInfo OperationID = "getETFPriceInfo"
	OperationGetETNPriceInfo OperationID = "getETNPriceInfo"
	OperationGetELWPriceInfo OperationID = "getELWPriceInfo"

	MarketKRX Market = "krx"

	SecurityTypeETF   SecurityType = "etf"
	SecurityTypeETN   SecurityType = "etn"
	SecurityTypeELW   SecurityType = "elw"
	SecurityTypeStock SecurityType = "stock"

	CredentialScopeDataGo CredentialScope = "datago"

	FreshnessDaily Freshness = "daily"

	RoleDailyBar   Role = "daily_bar"
	RoleInstrument Role = "instrument"
	RoleQuote      Role = "quote_snapshot"
)

type Identity struct {
	ID          ProviderID
	DisplayName string
}

func (i Identity) ProviderIdentity() Identity {
	return i
}

type IdentityProvider interface {
	ProviderIdentity() Identity
}
