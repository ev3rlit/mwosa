package datago

import (
	"net/http"
	"time"
)

const (
	defaultBaseURL   = "https://apis.data.go.kr/1160100/service/GetSecuritiesProductInfoService"
	defaultNumOfRows = 100
)

type Config struct {
	ServiceKey string
	BaseURL    string
	HTTPClient *http.Client
	NumOfRows  int
}

func (c Config) withDefaults() Config {
	if c.BaseURL == "" {
		c.BaseURL = defaultBaseURL
	}
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 15 * time.Second}
	}
	if c.NumOfRows <= 0 {
		c.NumOfRows = defaultNumOfRows
	}
	return c
}
