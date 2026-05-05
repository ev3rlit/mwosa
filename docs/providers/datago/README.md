# datago Provider

## 개요

`datago` 계열 provider 는 공공데이터포털의 OpenAPI 를 `mwosa` provider 로 연결하는 adapter 다.

공공데이터포털은 하나의 종합 API 보다 개별 API 서비스를 따로 제공하고, 실제 활용 신청도 OpenAPI 단위로 나뉜다. 그래서 하나의 `datago` provider 가 모든 OpenAPI client 를 들고 있으면 사용자가 승인하지 않은 API 까지 등록된 것처럼 보인다. 현재 구현은 활용 신청 단위를 provider id 로 분리하고, 원천 API 식별은 provider group 으로 남긴다.

현재 구현된 provider 는 두 개다.

- `datago`: `금융위원회_증권상품시세정보` OpenAPI 승인 범위에 해당하며 ETF, ETN, ELW 시세 데이터를 수집한다.
- `datago-corpfin`: `금융위원회_기업 재무정보` OpenAPI 승인 범위에 해당하며, 이름/단축코드 해석을 위해 `금융위원회_KRX상장종목정보` OpenAPI 승인도 필요하다.

원본 OpenAPI 스펙은 provider client module 안의
`clients/datago-etp/docs/securitiesProductPrice.openapi.yaml` 에 보관한다.

이 provider 의 client 구현체는 `mwosa` workspace 안의 **독립 Go module** 로 관리한다. `mwosa` repository root 의 `go.work` 로 CLI module 과 함께 개발하고, 필요하면 나중에 별도 repository 로 분리할 수 있다.

`datago` provider 는 canonical schema 관점에서 다음 역할을 가진다.

- ETF / ETN / ELW 종목의 일별 시세 공급
- 종목 메타데이터 snapshot 공급
- provider-neutral 검색 소스 중 하나로 동작

초기 버전에서는 실시간 호가나 분봉 provider 가 아니라, `basDt` 기준의 일별 시세 provider 로 취급한다.
또한 공공데이터포털의 `securitiesProductPrice` 데이터는 현재 거래일 데이터를 제공하는 feed 가 아니라,
일반적으로 **D-1 영업일 EOD 데이터**까지 제공되는 지연 데이터로 본다.

## 위치

- provider 문서: `docs/providers/datago/README.md`
- 원본 스펙: `clients/datago-etp/docs/securitiesProductPrice.openapi.yaml`
- 기업 재무정보 client: `clients/datago-corpfin`
- KRX 상장종목정보 client: `clients/datago-krxlisted`
- 공통 저장 계약: `docs/canonical-schema.md`

권장 패키지 분리:

- provider client module:
  - 예: `clients/datago-etp`
- in-CLI adapter:
  - 예: `providers/datago`

## 패키지 분리 원칙

`datago` provider 는 다음처럼 나눈다.

### Provider client module 에 둘 것

- OpenAPI endpoint 경로
- query parameter builder
- `serviceKey` 인증 처리
- `numOfRows`, `pageNo`, `totalCount` pagination metadata 처리
- JSON/XML 응답 파싱 정책
- `item` 단건/배열 shape 처리
- provider-native error model
- provider group 별 operation dispatch
- request builder, parser, error mapping 단위 테스트

### CLI 안 provider adapter 에 둘 것

- CLI config 를 provider client config 로 바꾸는 코드
- provider client 결과를 canonical normalizer 로 넘기는 코드
- provider group 을 포함한 registry 등록 코드
- fallback / provider priority 메타데이터
- CLI 옵션과 provider 호출 옵션의 연결

이 구조를 택하면 CLI 코어는 provider 세부사항을 직접 알 필요가 없고, `datago` client 도 adapter 등록 전에 독립적으로 테스트할 수 있다.

## Provider id 와 group

provider id 는 활용 신청 단위와 사용자가 설정해야 하는 credential 단위를 반영한다.

| provider id | 상태 | 공공데이터포털 API | 주요 group | capability |
| --- | --- | --- | --- | --- |
| `datago` | `core` | 금융위원회_증권상품시세정보 | `securitiesProductPrice` | `daily_bar`, `instrument` |
| `datago-corpfin` | `core` | 금융위원회_기업 재무정보, 금융위원회_KRX상장종목정보 | `corporateFinance`, `krxListedInfo` | `financials` |

group 은 provenance 와 내부 operation 식별에 사용한다.

group 은 다음 기준으로 나눈다.

- 공공데이터포털의 OpenAPI 서비스 단위
- 별도 활용신청 또는 승인 상태를 가질 수 있는 단위
- 인증, rate limit, 응답 envelope, pagination 정책을 따로 설명해야 하는 단위
- provenance 에 남겼을 때 원천 API 를 식별할 수 있는 단위

현재 group 계획:

| group | 상태 | 공공데이터포털 API | 주요 operation | capability |
| --- | --- | --- | --- | --- |
| `securitiesProductPrice` | `core` | 금융위원회_증권상품시세정보 | `getETFPriceInfo`, `getETNPriceInfo`, `getELWPriceInfo` | `daily_bar`, `instrument` |
| `corporateFinance` | `core` | 금융위원회_기업 재무정보 | `getSummFinaStat_V2`, `getBs_V2`, `getIncoStat_V2` | `financials` |
| `krxListedInfo` | `core` | 금융위원회_KRX상장종목정보 | `getItemInfo` | `financials` identifier resolution |
| `stockPrice` | `planned` | 금융위원회_주식시세정보 | `getStockPriceInfo` | `daily_bar`, `instrument` |

config 는 provider id 별로 둔다. `datago-corpfin` 은 이름/단축코드 기반 조회를 위해 `krxListedInfo` 의존 설정을 함께 가진다.

```json
{
  "providers": {
    "datago": {
      "enabled": true,
      "auth": {
        "service_key": "..."
      }
    },
    "datago-corpfin": {
      "enabled": true,
      "auth": {
        "service_key": "..."
      },
      "dependencies": {
        "krxListedInfo": {
          "auth": {
            "service_key": "..."
          }
        }
      }
    }
  }
}
```

CLI 설정 예:

```text
mwosa login provider datago --service-key <securitiesProductPrice-key>
mwosa login provider datago-corpfin --service-key <corporateFinance-key> --krx-listed-service-key <krxListedInfo-key>
```

## `securitiesProductPrice` API 표면

OpenAPI 스펙 기준으로 노출된 operation 은 3개다.

- `GET /getETFPriceInfo`
- `GET /getETNPriceInfo`
- `GET /getELWPriceInfo`

세 operation 모두 공통적으로 다음 성격을 가진다.

- `basDt`, `beginBasDt`, `endBasDt` 기준 조회 가능
- `srtnCd`, `isinCd`, `itmsNm` 기반 검색 가능
- 결과는 페이지네이션과 조건 검색을 지원
- 본문은 `body.items.item` 배열 또는 단건 객체로 내려올 수 있음
- 최신 사용 가능 `basDt` 는 일반적으로 오늘이 아니라 D-1 영업일이다.

즉, 이 group 은 단건 `quote` API 라기보다 **검색 가능한 일별 시세 목록 API** 에 가깝다.

## 데이터 호환성

`securitiesProductPrice` group 의 provider compatibility 는 다음처럼 본다.

- data latency: `previous_business_day`
- lag business days: `1`
- current trading-day supported: `false`
- intended use: D-1 영업일 EOD / historical daily bar / instrument snapshot
- not intended use: realtime quote, intraday decision feed, current trading-day quote

따라서 `basDt=오늘` 로 호출했을 때 `totalCount=0` 이 나오는 것은 provider 실패가 아니라
데이터 공개 지연 특성으로 취급한다. 오늘 투자 판단에 필요한 current price provider 와는 별도로
다뤄야 한다.

## v1 지원 범위

`datago` provider 의 초기 지원 범위는 `securitiesProductPrice` group 기준으로 다음과 같다.

- `daily_bar`
- `instrument`

초기 비지원 범위:

- 진짜 실시간 `quote_snapshot`
- 현재 거래일 daily bar 보장
- 분봉, 틱 데이터
- 주문/계좌 관련 데이터

이유는 OpenAPI 스펙상 `basDt` 기준의 일별 시세 필드가 명확하고, 현재 canonical schema v1 도 `instrument`, `daily_bar`, `quote_snapshot` 3개만 정의하고 있기 때문이다.

## Canonical endpoint 매핑

아래 매핑은 `securitiesProductPrice` group 기준이다.

### ETF

- source operation: `getETFPriceInfo`
- canonical security type: `etf`
- canonical record:
  - `daily_bar`
  - `instrument`

### ETN

- source operation: `getETNPriceInfo`
- canonical security type: `etn`
- canonical record:
  - `daily_bar`
  - `instrument`

### ELW

- source operation: `getELWPriceInfo`
- canonical security type:
  - 초기값은 `elw`
- canonical record:
  - `daily_bar`
  - `instrument`

주의:

- 현재 `docs/canonical-schema.md` 의 `security_type` 예시는 `stock`, `etf`, `etn` 중심이다.
- `datago` 를 ELW 까지 공식 지원하려면 canonical `security_type` enum 에 `elw` 를 추가하는 후속 결정이 필요하다.
- 그 결정 전까지는 구현 범위를 ETF, ETN 우선으로 제한하는 편이 안전하다.

## 요청 모델

provider adapter 는 public CLI 요청을 내부적으로 OpenAPI query 로 변환한다.

정확히는, CLI 안의 provider adapter 가 provider client 요청 모델로 변환하고, provider client module 이 실제 OpenAPI query 를 만든다.

### `get/ensure/sync/backfill daily`

canonical read request:

```text
get daily <security_code> --from <YYYYMMDD> --to <YYYYMMDD>
```

`get daily` 는 저장된 canonical data 를 조회한다. 저장된 데이터가 없으면 자동 대량 수집을 수행하지 않고, `ensure`, `sync`, `backfill` 명령을 안내한다.

canonical collect requests:

```text
ensure daily <security_code> --from <YYYYMMDD> --to <YYYYMMDD>
sync daily --market krx --security-type etf --as-of <YYYYMMDD>
backfill daily --market krx --security-type etf --from <YYYYMMDD> --to <YYYYMMDD>
```

`datago` 변환 규칙:

- `sync daily` 는 `as-of` -> `basDt` 로 변환해 해당 날짜의 시장/자산군 batch 를 수집한다.
- `backfill daily` 는 `from/to` 를 `beginBasDt/endBasDt` 로 변환해 기간 batch 를 수집한다. CLI 의 `--to` 는 포함 범위지만, Datago `endBasDt` 는 미만 조건이라 adapter 에서 `to + 1일` 로 보낸다. 주말/휴장일은 Datago 응답에 자연스럽게 포함되지 않으므로 별도 calendar 필터가 필요 없다.
- `ensure daily <security_code>` 는 필요한 날짜가 없으면 해당 날짜의 batch 를 먼저 수집한 뒤 저장소에서 `security_code` 를 조회한다.
- `resultType=json`
- page-level client 는 `numOfRows`, `pageNo`, `totalCount` 를 노출한다. `sync/backfill/ensure` 의 batch 수집은 all-page helper 를 사용해 남은 page 를 끝까지 순회하고, `--workers` 는 `backfill` 의 remaining page fetch 병렬도다.
- `provider=datago`, `provider_group=securitiesProductPrice`, 실제 operation 을 provenance 로 남긴다.
- 최신 데이터 요청은 오늘이 아니라 latest available basDt, 즉 보통 D-1 영업일 기준으로 해석한다.

주의:

- 이 API 는 단건 ticker endpoint 보다 날짜별 batch endpoint 로 쓰는 편이 효율적이다.
- 현재 거래일 데이터는 제공되지 않을 수 있으므로 realtime/current-day provider 로 사용하지 않는다.
- `get daily` 는 조회 명령이므로 데이터가 없을 때 빈 성공을 반환하지 않는다.
- ETF/ETN 은 기본 지원 대상으로 두고, ELW 는 `--security-type elw` 처럼 명시적으로 다룬다.

### `search instrument`

canonical request:

```text
search instrument "<keyword>"
```

`datago` 변환 규칙:

- 이름 검색 -> `likeItmsNm`
- 종목코드 검색 -> `likeSrtnCd`
- 필요 시 `isinCd` 도 지원 가능

## 응답 구조 해석

response envelope:

- `header.resultCode`
- `header.resultMsg`
- `body.numOfRows`
- `body.pageNo`
- `body.totalCount`
- `body.items.item`

`item` 은 단건 object 또는 array 일 수 있으므로 normalizer 는 둘 다 처리해야 한다.

## `corporateFinance` API 표면

공공데이터포털의 `금융위원회_기업 재무정보` API 는 다음 operation 을 제공한다.

- `GET /getSummFinaStat_V2`
- `GET /getBs_V2`
- `GET /getIncoStat_V2`

공통 요청 파라미터는 다음과 같다.

- `serviceKey`
- `numOfRows`
- `pageNo`
- `resultType=json`
- `crno`: 법인등록번호
- `bizYear`: 사업연도

이 API 는 종목코드나 ISIN 이 아니라 법인등록번호를 기준으로 조회한다. 하지만 사용자-facing CLI 는 법인등록번호를 1순위 입력으로 요구하지 않는다. `datago-corpfin` provider 의 `financials` role 은 `mwosa get financials <company>` 호출 시 먼저 회사명/종목명을 `crno` 로 해석한 뒤 `corporateFinance` API 를 호출한다.

해석 규칙은 다음과 같다.

- 1순위: 종목명/회사명 기반 조회. `krxListedInfo/getItemInfo` 의 `itmsNm` 로 조회한 뒤 exact match 의 `crno` 를 사용한다.
- 2순위: KRX 단축코드. 이름 조회가 실패하고 입력이 숫자 단축코드처럼 보이면 `likeSrtnCd` 로 조회한다.
- 3순위: ISIN. 이름 조회가 실패하고 입력이 12자리 ISIN처럼 보이면 `isinCd` 로 조회한다.
- 고급 입력: 13자리 숫자는 이미 `crno` 로 보고 바로 `corporateFinance` 를 호출한다.

즉, 재무제표 조회 경로는 `datago-corpfin` provider 안에서 `krxListedInfo` client 와 `corporateFinance` client 를 연계한다. provenance 의 provider 는 `datago-corpfin`, 재무제표 group 은 `corporateFinance` 로 남기고, symbol resolution 정보는 statement extension 에 `request_symbol`, `srtnCd`, `isinCd`, `corpNm`, `resolver_group=krxListedInfo`, `resolver_source=getItemInfo` 로 보관한다.

`corporateFinance` group 의 provider compatibility 는 다음처럼 본다.

- data latency: `historical`
- current trading-day supported: `false`
- intended use: filing-derived company financial statement lookup
- not intended use: realtime fundamentals
- dependency: KRX listed item lookup when the caller provides ticker-like identifiers

주의:

- 이 API 는 현금흐름표 operation 을 제공하지 않는다.
- 외국회사는 국내 법인등록번호가 비어 있을 수 있으며, 이 경우 `financials` 조회는 unsupported 로 실패한다.
- `--statement cash_flow` 는 `unsupported provider capability` 로 실패해야 한다.
- `--statement` 을 생략하면 요약재무제표, 재무상태표, 손익계산서를 차례로 조회한다.

## `krxListedInfo` API 표면

공공데이터포털의 `금융위원회_KRX상장종목정보` API 는 단일 operation 을 제공한다.

- `GET /getItemInfo`

공통 요청 파라미터는 다음과 같다.

- `serviceKey`
- `numOfRows`
- `pageNo`
- `resultType=json`
- `basDt`, `beginBasDt`, `endBasDt`, `likeBasDt`
- `likeSrtnCd`: KRX 단축코드 검색
- `isinCd`, `likeIsinCd`: ISIN 검색
- `itmsNm`, `likeItmsNm`: 종목명 검색
- `crno`: 법인등록번호 검색
- `corpNm`, `likeCorpNm`: 법인명 검색

응답은 기준일자, 단축코드, ISIN, 시장구분, 종목명, 법인등록번호, 법인명을 제공한다. 데이터는 일 1회 갱신되며 금융위원회 API 특성상 실시간 데이터가 아니라 기준일자 다음 영업일 오후 1시 이후 공개되는 지연 데이터로 취급한다.

## Field to canonical mapping

### 공통 기본 매핑

| source field | canonical field | 비고 |
| --- | --- | --- |
| `basDt` | `trading_date` | `YYYYMMDD` -> `YYYY-MM-DD` |
| `srtnCd` | `security_code` | KRX 단축코드 |
| `itmsNm` | `security_name` | 종목명 |
| `clpr` | `closing_price` | 종가 |
| `vs` | `price_change_from_previous_close` | 전일 대비 |
| `fltRt` | `price_change_rate_from_previous_close` | 등락률 |
| `mkp` | `opening_price` | 시가 |
| `hipr` | `highest_price` | 고가 |
| `lopr` | `lowest_price` | 저가 |
| `trqu` | `traded_volume` | 거래량 |
| `trPrc` | `traded_amount` | 거래대금 |
| `mrktTotAmt` | `market_capitalization` | `daily_bar` 공통 본문에는 직접 저장하지 않고 extension 후보 |

### ETF 추가 매핑

| source field | target | 비고 |
| --- | --- | --- |
| `isinCd` | provenance or instrument metadata | canonical common field 아님 |
| `nPptTotAmt` | provider extension | ETF 순자산총액 |
| `stLstgCnt` | provider extension | ETF 상장좌수 |
| `nav` | provider extension | ETF NAV |
| `bssIdxIdxNm` | instrument metadata extension | 기초지수명 |
| `bssIdxClpr` | provider extension | 기초지수 종가 |

### ETN 추가 매핑

| source field | target | 비고 |
| --- | --- | --- |
| `isinCd` | provenance or instrument metadata | canonical common field 아님 |
| `indcVal` | provider extension | ETN IV |
| `indcValTotAmt` | provider extension | ETN 지표가치총액 |
| `lstgScrtCnt` | provider extension | ETN 상장증권수 |
| `bssIdxIdxNm` | instrument metadata extension | 기초지수명 |
| `bssIdxClpr` | provider extension | 기초지수 종가 |

### ELW 추가 매핑

ELW 역시 `basDt`, `srtnCd`, `itmsNm`, `clpr`, `mkp`, `hipr`, `lopr`, `trqu`, `trPrc`, `mrktTotAmt` 기반의 `daily_bar` 로 정규화할 수 있다. 다만 증권 유형 확정과 확장 필드 계약은 별도 결정이 필요하다.

## Canonical 변환 규칙

### `daily_bar`

`datago` provider 가 생성하는 `daily_bar` 는 다음 규칙을 따른다.

- `market = "krx"`
- `provider = "datago"`
- `provider_group = "securitiesProductPrice"`
- `currency_code = "KRW"`
- `market_session = "regular"`
- `price_adjustment_type = "raw"`
- `price_currency_code = "KRW"`
- `trading_date = basDt` 의 ISO date 변환값
- `canonical_record_key = daily_bar:krx:<security_code>:<trading_date>`

중요:

- 이 API 스펙에는 수정주가 여부가 드러나지 않으므로 `price_adjustment_type` 는 기본적으로 `raw` 로 둔다.
- `previous_closing_price` 는 응답에 없으므로 v1 에서는 계산해서 채우지 않고 `null` 을 허용한다.

### `instrument`

`instrument` 는 각 row 에서 파생해 upsert 한다.

기본 규칙:

- `security_key = krx:<security_code>`
- `canonical_record_key = instrument:krx:<security_code>:current`
- `security_type` 는 operation 에 따라 `etf`, `etn`, `elw`
- `market_segment` 는 operation category 와 동일하게 둔다
- `country_code = "KR"`
- `timezone = "Asia/Seoul"`
- `exchange_code = "KRX"`
- `security_status`, `listed_on`, `delisted_on` 은 스펙에 없으면 `null`

## Provider extension 처리

`nav`, `indcVal`, `bssIdxIdxNm` 같은 필드는 v1 canonical common field 에 없다.

따라서 초기 방침은 다음과 같다.

- 공통 조회/분석에 필수인 값만 canonical body 에 반영
- provider 특화 값은 provenance metadata 또는 별도 provider extension block 으로 저장
- 공통 스키마에 넣을 가치가 확인되면 다음 canonical schema version 에 승격

이 방침을 택하는 이유는 공통 저장 계약을 provider별 필드로 오염시키지 않기 위해서다.

## Provider 신뢰도와 우선순위

`datago` provider 는 다음 특성을 가진다.

- 장점:
  - provider 이름을 `datago` 하나로 유지하면서 API 서비스별 group 을 확장할 수 있다.
  - 공공데이터포털의 개별 API 승인 범위를 group 단위로 점검할 수 있다.
  - 검색 기반의 넓은 범위 조회가 가능
  - ETF / ETN / ELW 를 하나의 계열 API 에서 제공
  - KIS credentials 없이도 동작 가능한 공공 OpenAPI 경로
- 단점:
  - 실시간 snapshot provider 로 보기 어렵다
  - rate limit 과 공공 API 안정성 영향을 받을 수 있다
  - 페이지네이션과 item shape variation 을 처리해야 한다

초기 우선순위 정책 제안:

- `daily_bar`
  - `datago` 를 1순위 또는 KIS 와 동급 후보로 고려 가능
- `quote_snapshot`
  - 비활성
- `instrument search`
  - 활성

## CLI 관점에서의 사용 예

사용자는 provider를 직접 의식하지 않아야 한다.

예:

- `mwosa get daily 491820 --from 20240101 --to 20240415`
- `mwosa sync daily --market krx --security-type etf --as-of 20260424 --provider datago`
- `mwosa backfill daily --market krx --security-type etf --from 20240101 --to 20240415 --workers 4 --provider datago`
- `mwosa search instrument "KODEX 미국채"`
- `mwosa ensure daily 069500 --from 20240101 --to 20240415`

내부에서만 다음 정책이 적용된다.

- `daily_bar` 는 `datago` 또는 `kis` 중 가능한 provider 선택
- `quote_snapshot` 는 `datago` 를 후보에서 제외
- instrument search 는 `datago` 를 포함한 provider fan-out 가능
- provenance 에는 `provider=datago`, `provider_group=securitiesProductPrice`, 실제 operation 을 함께 남김

## 오류 및 edge case

구현 시 반드시 처리해야 하는 항목:

- `item` 이 object 하나로 내려오는 경우
- `item` 이 array 로 내려오는 경우
- `resultType` 기본값이 `xml` 이므로 항상 `json` 을 명시해야 하는 점
- 페이지네이션 필요 시 `pageNo`, `numOfRows` 반복 호출
- 검색 결과가 여러 종목일 때 `security_code` exact match 와 fuzzy result 를 구분해야 하는 점

## 구현 체크포인트

이 provider 문서를 기준으로 다음 구현을 진행한다.

1. provider client module `clients/datago-etp` 생성
2. client module 내부 provider group / operation registry 작성
3. `securitiesProductPrice` OpenAPI query builder 작성
4. client module 내부 item normalization 구현
5. Go CLI adapter `providers/datago` 추가
6. `daily_bar` 와 `instrument` upsert 구현
7. provider group 과 operation provenance 저장 위치 결정
8. provider extension metadata 저장 위치 결정
9. KIS provider 와 함께 registry 우선순위 정책 연결
