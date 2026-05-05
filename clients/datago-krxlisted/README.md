# datago-krxlisted

`datago-krxlisted` is a standalone provider client module for the Datago
`금융위원회_KRX상장종목정보` OpenAPI.

It handles:

- `getItemInfo`
- Data.go.kr `serviceKey` authentication
- `resultType=json`
- `numOfRows` / `pageNo` pagination metadata
- `body.items.item` as either a single object or an array
- typed KRX listed item rows with raw field access

The default service base URL is:

```text
https://apis.data.go.kr/1160100/service/GetKrxListedInfoService
```
