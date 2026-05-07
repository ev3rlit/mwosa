# datago-corpfin

`datago-corpfin` is a standalone provider client module for the Datago
`corporateFinance` API, published on data.go.kr as `금융위원회_기업 재무정보`.

It owns endpoint paths, `serviceKey`, explicit pagination parameters,
provider-native response parsing, and remote error context for:

- `getSummFinaStat_V2`
- `getBs_V2`
- `getIncoStat_V2`

The API looks up a company by corporation registration number (`crno`) and
fiscal year (`bizYear`). It does not resolve stock tickers to corporation
registration numbers.
