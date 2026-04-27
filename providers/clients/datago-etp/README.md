# datago-etp

`datago-etp` is a standalone provider client module for the Datago
`securitiesProductPrice` API. It owns endpoint paths, `serviceKey`, explicit
pagination parameters, provider-native response parsing, and remote error
context.

## Live e2e test

Create a local config file from the example.

```sh
cp config.example.json config.local.json
```

Put your Datago service key in `config.local.json`.

Use the decoded service key value when possible. The client sends `serviceKey`
through Go query encoding.

The live tests use explicit query fixtures in `e2e_test.go`. They avoid empty
requests because this API can return a successful JSON envelope with
`totalCount=0` when no useful search condition is supplied.
`basDt` is part of each API query fixture in the test code, not local config.
`num_of_rows` is copied into the public query structs as the Datago `numOfRows`
parameter, not into client configuration.
The client fetches only the requested `pageNo`; callers must opt in to any
multi-page loop outside the client.

The client exposes both page-level and all-page helpers:

- `GetETFPriceInfo`, `GetETNPriceInfo`, `GetELWPriceInfo`: fetch one requested page.
- `GetETFPriceInfoMetadata`, `GetETNPriceInfoMetadata`, `GetELWPriceInfoMetadata`:
  fetch a one-row probe and return `totalCount`, planned page size, and page count.
- `GetAllETFPriceInfo`, `GetAllETNPriceInfo`, `GetAllELWPriceInfo`: fetch page 1
  first, read `totalCount`, then fetch only the remaining pages. When `NumOfRows`
  is omitted, all-page helpers use `DefaultAllNumOfRows`.

Run the live e2e tests explicitly with `DATAGO_ETP_E2E=1`.

```sh
DATAGO_ETP_E2E=1 go test -run '^TestLive' -count=1
```

Run one operation.

```sh
DATAGO_ETP_E2E=1 go test -run '^TestLiveETFPriceInfo$' -count=1
```

Run the pagination row timing test.

```sh
DATAGO_ETP_E2E=1 DATAGO_ETP_PAGINATION_TIMING=1 go test -run '^TestLivePaginationRowTiming$' -count=1 -v
```

The timing test samples only page 1 for each row size and estimates the full
collection cost from `totalCount`. This protects the Datago daily quota of
10,000 calls while still making row-size comparison possible. By default it
tests `numOfRows=10,50,100,200` across ETF, ETN, and ELW, for 12 calls total.
Tune it with:

- `DATAGO_ETP_PAGINATION_ROWS=10,50,100,200,500`
- `DATAGO_ETP_PAGINATION_OPS=etf,etn,elw`
- `DATAGO_ETP_PAGINATION_REPEATS=2`
- `DATAGO_ETP_PAGINATION_MAX_CALLS=30`
- `DATAGO_ETP_PAGINATION_OUTPUT=e2e-output-pagination.json`

The e2e tests use the fixed Datago base URL from `DefaultBaseURL`, so the local
config file does not override the endpoint. The tests do not print the service
key. The client always sends `resultType=json`, because Datago defaults to XML
when that query parameter is omitted.
