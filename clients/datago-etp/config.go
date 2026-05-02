package etp

import (
	"net/http"
	"time"
)

const (
	DefaultBaseURL      = "https://apis.data.go.kr/1160100/service/GetSecuritiesProductInfoService"
	DefaultNumOfRows    = 100
	DefaultAllNumOfRows = 1000

	ProviderDataGo              = "datago"
	GroupSecuritiesProductPrice = "securitiesProductPrice"
	OperationGetETFPriceInfo    = "getETFPriceInfo"
	OperationGetETNPriceInfo    = "getETNPriceInfo"
	OperationGetELWPriceInfo    = "getELWPriceInfo"
	DefaultHTTPClientTimeout    = 15 * time.Second
)

type Config struct {
	ServiceKey string
	BaseURL    string
	HTTPClient *http.Client
}

func (c Config) withDefaults() Config {
	if c.BaseURL == "" {
		c.BaseURL = DefaultBaseURL
	}
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: DefaultHTTPClientTimeout}
	}
	return c
}
