# External Provider Go Package Contract

## 목적

이 문서는 외부 provider Go package 들이 따라야 하는 공통 계약을 정의한다.

대상 예시:

- `github.com/<org>/marketdata-provider-kis`
- `github.com/<org>/marketdata-provider-data-go-etf`
- `github.com/<org>/marketdata-provider-krx`

이 문서의 목적은 다음과 같다.

- CLI 코어가 provider별 세부사항을 몰라도 되게 한다
- provider package 간 capability, request, result 형태를 가능한 한 통일한다
- CLI 내부 bridge 계층이 얇게 유지되도록 한다

## 범위

이 문서는 **외부 provider package 계약**을 다룬다.

다루는 것:

- package 책임
- 공통 interface
- request / result shape
- error model
- capability model

다루지 않는 것:

- canonical storage
- SurrealDB metadata
- CLI flags
- delete / reindex 구현

## 핵심 원칙

### 1. Provider package 는 외부 API와만 대화한다

provider package 의 1차 책임은 외부 API 호출, 인증, 응답 파싱이다.

provider package 가 해서는 안 되는 일:

- 로컬 canonical 파일 저장
- SurrealDB 갱신
- CLI 출력 formatting
- 지표 계산

### 2. Provider package 는 provider-native result 를 반환한다

외부 package 는 꼭 canonical record 를 직접 만들 필요는 없다.  
오히려 초기에는 provider-native result 를 반환하고, CLI bridge 또는 canonical normalizer 가 canonical 모델로 바꾸는 편이 낫다.

이유:

- provider 패키지를 다른 런타임에서 재사용 가능
- canonical schema 변경 시 provider package 영향 축소
- provider-specific extension data 보존이 쉬움

### 3. Context-aware API 를 사용한다

모든 네트워크 호출은 `context.Context` 를 받아야 한다.

### 4. Capability 는 코드로 드러나야 한다

문서만 보고 provider를 고르지 않는다. package 가 자신이 지원하는 endpoint 와 제약을 코드로 알려줘야 한다.

## 권장 package 표면

최소 권장 인터페이스:

```go
package provider

import "context"

type Package interface {
	Name() string
	Capabilities() Capabilities
	FetchDailyPrices(ctx context.Context, input DailyPriceRequest) (DailyPriceResult, error)
	FetchQuoteSnapshot(ctx context.Context, input QuoteSnapshotRequest) (QuoteSnapshotResult, error)
	SearchSecurities(ctx context.Context, input SecuritySearchRequest) (SecuritySearchResult, error)
}
```

주의:

- 어떤 package 는 `FetchQuoteSnapshot` 를 실제로 지원하지 않을 수 있다
- 이 경우 capability 에서 비지원으로 표시하고, 호출 시 명확한 unsupported error 를 반환한다

## Capabilities

```go
type Capabilities struct {
	SupportsDailyPrices    bool
	SupportsQuoteSnapshot  bool
	SupportsSecuritySearch bool
	SupportedMarkets       []string
	SupportedSecurityTypes []string
	SupportsDateRangeQuery bool
	SupportsPagination     bool
}
```

추가 확장 가능 항목:

- `NeedsAuth`
- `RealtimeQuality`
- `HistoricalQuality`
- `RateLimitHint`

## 공통 요청 모델

### `DailyPriceRequest`

```go
type DailyPriceRequest struct {
	Market       string
	SecurityType string
	SecurityCode string
	FromDate     string
	ToDate       string
	PageSizeHint int
}
```

규칙:

- `FromDate`, `ToDate` 는 `YYYYMMDD`
- market / security type 은 canonical vocabulary 를 따른다
- provider가 일부 필드를 무시하더라도 요청 구조는 유지한다

### `QuoteSnapshotRequest`

```go
type QuoteSnapshotRequest struct {
	Market       string
	SecurityType string
	SecurityCode string
}
```

### `SecuritySearchRequest`

```go
type SecuritySearchRequest struct {
	Market       string
	SecurityType string
	Query        string
	LimitHint    int
}
```

## 공통 결과 모델

외부 provider package 는 canonical record 대신 **provider-native but stable** 한 결과 모델을 반환한다.

### `DailyPriceResult`

```go
type DailyPriceResult struct {
	ProviderName string
	Items        []DailyPriceItem
	NextPage     *string
	RawPageCount int
}
```

### `DailyPriceItem`

```go
type DailyPriceItem struct {
	ObservedDate               string
	SecurityCode               string
	SecurityName               string
	OpeningPrice               *float64
	HighestPrice               *float64
	LowestPrice                *float64
	ClosingPrice               *float64
	TradedVolume               *float64
	TradedAmount               *float64
	PreviousCloseChange        *float64
	PreviousCloseChangeRate    *float64
	ProviderPayload            map[string]any
}
```

이 구조는 이미 canonical 에 가까워 보일 수 있지만, 다음 차이를 유지한다.

- field 는 provider package 관점의 neutral result 일 뿐 canonical envelope 는 아님
- `ProviderPayload` 로 provider 확장 필드를 보존한다
- CLI bridge 가 `canonical_record_key`, `security_key`, `stored_at` 등을 채운다

### `QuoteSnapshotResult`

```go
type QuoteSnapshotResult struct {
	ProviderName string
	Item         QuoteSnapshotItem
}
```

```go
type QuoteSnapshotItem struct {
	ObservedAt                  string
	SecurityCode                string
	SecurityName                string
	SnapshotPrice               *float64
	OpeningPrice                *float64
	HighestPrice                *float64
	LowestPrice                 *float64
	CumulativeTradedVolume      *float64
	CumulativeTradedAmount      *float64
	PreviousClosingPrice        *float64
	PreviousCloseChange         *float64
	PreviousCloseChangeRate     *float64
	UpperPriceLimit             *float64
	LowerPriceLimit             *float64
	MarketCapitalization        *float64
	ProviderPayload             map[string]any
}
```

### `SecuritySearchResult`

```go
type SecuritySearchResult struct {
	ProviderName string
	Items        []SecuritySearchItem
}
```

```go
type SecuritySearchItem struct {
	SecurityCode   string
	SecurityName   string
	SecurityType   string
	Market         string
	ISINCode       *string
	ExchangeCode   *string
	ProviderPayload map[string]any
}
```

## Error model

권장 에러 분류:

```go
type ErrorCode string

const (
	ErrUnsupported       ErrorCode = "unsupported"
	ErrUnauthorized      ErrorCode = "unauthorized"
	ErrRateLimited       ErrorCode = "rate_limited"
	ErrRemoteUnavailable ErrorCode = "remote_unavailable"
	ErrInvalidRequest    ErrorCode = "invalid_request"
	ErrDecodeFailure     ErrorCode = "decode_failure"
)

type ProviderError struct {
	Code       ErrorCode
	Provider   string
	Operation  string
	Message    string
	Retryable  bool
	Cause      error
}
```

규칙:

- unsupported 는 capability 와 일관돼야 한다
- 외부 API 에러 코드는 가능하면 보존한다
- CLI bridge 는 이 에러를 보고 fallback 여부를 결정할 수 있어야 한다

## Bridge가 맡을 일

CLI 내부 bridge 는 provider package 결과를 다음으로 변환한다.

- canonical request -> provider request
- provider result -> canonical record
- provider error -> CLI/service error
- capability -> registry metadata

즉, 외부 provider package 는 “원격 데이터 취득”, bridge 는 “CLI 코어와의 접속면”이다.

## Versioning 원칙

- provider package 는 semantic versioning 을 따른다
- request / result type 에 breaking change 가 생기면 major version 을 올린다
- CLI는 지원하는 provider package major version 범위를 문서화한다

## `data-go-etf` 적용 예

`marketdata-provider-data-go-etf` 는 다음처럼 읽으면 된다.

- `FetchDailyPrices`
  - 지원
- `FetchQuoteSnapshot`
  - 비지원 또는 unsupported 반환
- `SearchSecurities`
  - 지원

bridge 는 다음을 수행한다.

- `beginBasDt/endBasDt` query 빌드 위임
- `DailyPriceItem` 을 `daily_bar` canonical record 로 변환
- `SecuritySearchItem` 을 `instrument` upsert 흐름으로 연결

## 초기 구현 우선순위

1. provider package 공통 interface 고정
2. capability / error model 고정
3. `provider-data-go-etf` 패키지 작성
4. `provider-kis` 패키지 작성
5. CLI bridge package 작성
