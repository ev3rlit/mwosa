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
	DefaultRetryMaxAttempts     = 3
	DefaultRetryInitialWait     = 200 * time.Millisecond
	DefaultRetryMaxWait         = 2 * time.Second
)

type Config struct {
	ServiceKey       string
	BaseURL          string
	HTTPClient       *http.Client
	RetryMaxAttempts int
	RetryInitialWait time.Duration
	RetryMaxWait     time.Duration
}

func (c Config) withDefaults() Config {
	if c.BaseURL == "" {
		c.BaseURL = DefaultBaseURL
	}
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: DefaultHTTPClientTimeout}
	}
	if c.RetryMaxAttempts <= 0 {
		c.RetryMaxAttempts = DefaultRetryMaxAttempts
	}
	if c.RetryInitialWait <= 0 {
		c.RetryInitialWait = DefaultRetryInitialWait
	}
	if c.RetryMaxWait <= 0 {
		c.RetryMaxWait = DefaultRetryMaxWait
	}
	return c
}
