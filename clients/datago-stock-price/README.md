# datago-stock-price

Client module for the public data portal `stockPrice` provider group.

Source API:

- Dataset: `Financial Services Commission_stock price information`
- Service URL: `https://apis.data.go.kr/1160100/service/GetStockSecuritiesInfoService`
- Operation: `getStockPriceInfo`

The client owns request building, service key authentication, pagination,
JSON envelope parsing, and provider-native error context. Adapter-level
canonical mapping lives in `providers/datago`.
