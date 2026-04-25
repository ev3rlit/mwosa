# Layer And Interface Architecture

## 목적

이 문서는 `mwosa` 가 여러 market data provider 를 지원하면서도 사용자에게 일관된 데이터 구조를 보여주기 위한 layer 와 interface 설계를 정의한다.

핵심 방향은 다음과 같다.

- provider 마다 다른 API shape 는 bridge/normalizer 계층에서 흡수한다.
- service 계층부터는 provider-neutral canonical model 만 사용한다.
- 큰 `Provider` interface 하나에 모든 기능을 넣지 않는다.
- 기능별 capability interface 를 작게 나누어 provider 가 지원하는 역할만 구현하게 한다.
- interface 는 되도록 사용하는 쪽 package 에 둔다.

## 전체 흐름

```text
CLI command
  -> typed service request
  -> service
  -> provider registry
  -> capability-specific provider bridge
  -> external provider package
  -> provider-native result
  -> normalizer
  -> canonical record
  -> storage/index
  -> service result
  -> formatter
```

이 구조에서 provider 고유 형식은 `external provider package` 와 `providers/*bridge` 안에만 머문다. `service`, `storage`, `indicator`, `format` 은 provider 고유 응답 타입을 직접 알지 않는다.

## Layer 책임

### `internal/command`

- Cobra command 에서 받은 argv 와 flag 를 검증한다.
- CLI 입력을 service request 로 변환한다.
- provider, storage, index 를 직접 호출하지 않는다.
- 출력 포맷을 직접 만들지 않는다.

예:

```go
type InspectInstrumentRequest struct {
	Symbols []string
	Market  string
	AsOf    civil.Date
}
```

### `internal/service`

- 실제 use case 를 실행한다.
- provider registry, storage, index, indicator 를 조합한다.
- 입력과 출력은 CLI 에 독립적인 request/result type 으로 둔다.
- service 부터는 canonical type 을 기준으로 사고한다.

예:

```go
type InstrumentInspector interface {
	InspectInstruments(ctx context.Context, input InspectInstrumentInput) (InspectInstrumentResult, error)
}
```

### `internal/provider`

- provider 를 고르는 registry 와 capability model 을 가진다.
- provider bridge 가 구현해야 하는 내부 interface 를 정의한다.
- fallback, priority, freshness, auth 필요 여부 같은 선택 정책을 관리한다.

### `internal/providers/*bridge`

- 외부 provider package 를 내부 provider interface 에 맞춘다.
- CLI config 를 provider package config 로 바꾼다.
- provider-native result 를 canonical record 로 변환한다.
- provider-native error 를 내부 error model 로 변환한다.
- canonical storage 를 직접 쓰지 않는다.

### external provider package

- 외부 API 호출, 인증, pagination, raw response parsing 을 담당한다.
- canonical storage, SurrealDB, CLI flag, formatter 를 알지 않는다.
- provider-native but stable result 를 반환한다.

## Interface 설계 원칙

### 1. 큰 provider interface 를 피한다

다음처럼 모든 기능을 하나에 넣으면 provider 가 지원하지 않는 method 를 억지로 구현해야 한다.

```go
type MarketDataProvider interface {
	FetchQuote(...)
	FetchDailyBars(...)
	SearchInstruments(...)
	FetchFinancials(...)
	FetchNews(...)
}
```

이 방식은 시간이 지나면 `unsupported` method 와 빈 구현이 늘어난다. `mwosa` 에서는 endpoint/capability 단위로 interface 를 쪼갠다.

### 2. 공통 identity 와 capability 는 작게 둔다

모든 provider bridge 는 최소한 자기 이름과 capability 를 알려야 한다.

```go
type ProviderIdentity interface {
	ProviderName() ProviderName
}

type CapabilityReporter interface {
	Capabilities() CapabilitySet
}

type ProviderBase interface {
	ProviderIdentity
	CapabilityReporter
}
```

### 3. 데이터 endpoint 는 capability interface 로 분리한다

```go
type InstrumentSearcher interface {
	ProviderBase
	SearchInstruments(ctx context.Context, input InstrumentSearchInput) (InstrumentSearchResult, error)
}

type InstrumentFetcher interface {
	ProviderBase
	FetchInstrument(ctx context.Context, input InstrumentFetchInput) (InstrumentFetchResult, error)
}

type QuoteSnapshotFetcher interface {
	ProviderBase
	FetchQuoteSnapshot(ctx context.Context, input QuoteSnapshotFetchInput) (QuoteSnapshotFetchResult, error)
}

type DailyBarFetcher interface {
	ProviderBase
	FetchDailyBars(ctx context.Context, input DailyBarFetchInput) (DailyBarFetchResult, error)
}
```

provider 는 자신이 지원하는 interface 만 구현한다.

예:

- `kisbridge`: `QuoteSnapshotFetcher`, `DailyBarFetcher`, `InstrumentSearcher`
- `datagobridge`: `DailyBarFetcher`, `InstrumentSearcher`
- `newsbridge`: `NewsSearcher`, `NewsFetcher`

### 4. service 는 필요한 capability 만 요구한다

service 는 구체 provider 를 알지 않고 registry 에 capability 를 요청한다.

```go
type DailyBarProviderSelector interface {
	SelectDailyBarFetcher(ctx context.Context, input DailyBarFetchInput) (DailyBarFetcher, error)
}

type QuoteProviderSelector interface {
	SelectQuoteSnapshotFetcher(ctx context.Context, input QuoteSnapshotFetchInput) (QuoteSnapshotFetcher, error)
}
```

이렇게 하면 `calc rsi` service 는 daily bar capability 만 알면 되고, quote/news/provider 구현을 몰라도 된다.

### 5. interface 는 소비자 쪽에 둔다

Go 에서는 구현체 쪽보다 사용하는 쪽에 작은 interface 를 두는 편이 변경에 강하다.

권장:

- `service/data` 가 필요한 storage interface 는 `service/data` 쪽에 둔다.
- `service/instrument` 가 필요한 provider selector interface 는 `service/instrument` 쪽에 둔다.
- provider bridge package 는 별도 선언 없이 필요한 method 를 구현한다.

단, 여러 service 에서 반복되는 핵심 provider capability 는 `internal/provider` 에 둔다.

## Provider capability model

Capability 는 단순 bool 목록보다 endpoint, market, asset type, freshness 를 함께 표현할 수 있어야 한다.

```go
type Endpoint string

const (
	EndpointInstrumentSearch Endpoint = "instrument.search"
	EndpointInstrumentFetch  Endpoint = "instrument.fetch"
	EndpointQuoteSnapshot    Endpoint = "quote.snapshot"
	EndpointDailyBar         Endpoint = "daily_bar"
)

type Capability struct {
	Endpoint      Endpoint
	Markets       []Market
	SecurityTypes []SecurityType
	NeedsAuth     bool
	Freshness     FreshnessClass
	Priority      int
}

type CapabilitySet struct {
	Items []Capability
}
```

선택 정책은 registry 가 담당한다.

```go
type Registry interface {
	Register(provider ProviderBase) error
	Find(input ProviderSelectionInput) ([]ProviderBase, error)
}
```

service 는 `Find` 의 세부 구현을 몰라도 되고, 더 구체적인 selector interface 만 사용한다.

## Canonical boundary

`mwosa` 에는 세 종류의 데이터 모델이 존재한다.

### Provider-native model

- 외부 provider package 가 반환한다.
- provider API 의 현실을 보존한다.
- pagination, raw payload, provider-specific field 를 포함할 수 있다.
- CLI core 밖에서도 재사용될 수 있다.

### Canonical record

- `internal/canonical` 이 정의한다.
- storage 와 service 가 기준으로 삼는 provider-neutral model 이다.
- `instrument`, `daily_bar`, `quote_snapshot` 같은 record type 으로 나눈다.
- provider 정보는 본문이 아니라 provenance 로 분리한다.

### Service result

- command 와 formatter 에 반환하는 use case 결과다.
- canonical record 를 그대로 반환할 수도 있고, inspect/compare/screen 에 맞춘 view model 로 가공할 수도 있다.
- provider-specific payload 를 기본 출력에 노출하지 않는다.

## Normalizer interface

bridge 는 provider-native result 를 canonical record 로 바꾼다.

```go
type DailyBarNormalizer[Input any] interface {
	NormalizeDailyBars(input Input, meta NormalizeMeta) ([]canonical.DailyBar, []canonical.Provenance, error)
}

type QuoteSnapshotNormalizer[Input any] interface {
	NormalizeQuoteSnapshot(input Input, meta NormalizeMeta) (canonical.QuoteSnapshot, canonical.Provenance, error)
}
```

초기 구현에서 generics 가 과하게 느껴지면 provider bridge 내부 함수로 시작해도 된다.

```go
func normalizeDailyBars(result datago.DailyPriceResult, meta NormalizeMeta) ([]canonical.DailyBar, []canonical.Provenance, error)
```

중요한 점은 normalizer 가 다음을 책임진다는 것이다.

- provider field 이름을 canonical field 이름으로 바꾼다.
- 날짜, 통화, market, security type 을 canonical vocabulary 로 정규화한다.
- canonical key 를 만든다.
- provenance 를 만든다.
- 누락/해석 불가능한 필드는 조용히 버리지 않고 명시적 error 로 반환한다.

## Storage interface

storage 는 record type 과 index 역할을 나눠서 작게 둔다.

```go
type DailyBarStore interface {
	AppendDailyBars(ctx context.Context, records []canonical.DailyBar) error
	ReadDailyBars(ctx context.Context, selector DailyBarSelector) ([]canonical.DailyBar, error)
	DeleteDailyBars(ctx context.Context, selector DailyBarSelector) (DeleteResult, error)
}

type QuoteSnapshotStore interface {
	AppendQuoteSnapshots(ctx context.Context, records []canonical.QuoteSnapshot) error
	ReadQuoteSnapshots(ctx context.Context, selector QuoteSnapshotSelector) ([]canonical.QuoteSnapshot, error)
}

type InstrumentStore interface {
	UpsertInstruments(ctx context.Context, records []canonical.Instrument) error
	ReadInstruments(ctx context.Context, selector InstrumentSelector) ([]canonical.Instrument, error)
}
```

index 는 별도 interface 로 둔다.

```go
type CoverageIndex interface {
	GetCoverage(ctx context.Context, selector CoverageSelector) (CoverageResult, error)
	ReplaceCoverage(ctx context.Context, update CoverageUpdate) error
}

type ManifestIndex interface {
	AddFileObject(ctx context.Context, object FileObject) error
	FindFileObjects(ctx context.Context, selector FileObjectSelector) ([]FileObject, error)
}

type ProvenanceIndex interface {
	RecordProvenance(ctx context.Context, records []canonical.Provenance) error
	FindProvenance(ctx context.Context, selector ProvenanceSelector) ([]canonical.Provenance, error)
}

type LatestQuoteIndex interface {
	GetLatestQuote(ctx context.Context, security canonical.SecurityKey) (canonical.QuoteSnapshot, error)
	UpdateLatestQuote(ctx context.Context, quote canonical.QuoteSnapshot) error
}
```

파일 정본과 SurrealDB index 를 하나의 거대한 repository 로 합치지 않는다. service 가 use case 에 필요한 작은 interface 묶음만 받게 한다.

## Service dependency 예시

`get daily` service 는 다음 역할만 필요하다.

```go
type GetDailyService struct {
	Providers DailyBarProviderSelector
	Bars      DailyBarStore
	Coverage  CoverageIndex
	Manifest  ManifestIndex
	Provenance ProvenanceIndex
}
```

`calc rsi` service 는 provider 를 직접 알 필요가 없다. daily series 확보 use case 만 의존하면 된다.

```go
type DailySeriesEnsurer interface {
	EnsureDailyBars(ctx context.Context, input EnsureDailyBarsInput) ([]canonical.DailyBar, error)
}

type RSICalculator struct {
	Series DailySeriesEnsurer
}
```

이렇게 하면 계산 로직은 provider/storage 변경에 흔들리지 않는다.

## Error model

error 는 fallback 판단과 사용자 메시지 생성을 위해 분류돼야 한다.

```go
type ErrorCode string

const (
	ErrUnsupported       ErrorCode = "unsupported"
	ErrUnauthorized      ErrorCode = "unauthorized"
	ErrRateLimited       ErrorCode = "rate_limited"
	ErrRemoteUnavailable ErrorCode = "remote_unavailable"
	ErrInvalidRequest    ErrorCode = "invalid_request"
	ErrDecodeFailure     ErrorCode = "decode_failure"
	ErrDataInvariant     ErrorCode = "data_invariant"
)

type ProviderError struct {
	Code      ErrorCode
	Provider  ProviderName
	Endpoint  Endpoint
	Retryable bool
	Cause     error
}
```

규칙:

- unsupported 는 capability 와 일치해야 한다.
- fallback 은 registry/service 에서 명시적으로 판단한다.
- invalid input 을 조용히 무시하지 않는다.
- partial data 를 허용해야 한다면 result 에 completeness 를 명시한다.

## 출력 interface

formatter 는 service result 를 출력 형식으로 바꾼다.

```go
type Renderer[T any] interface {
	Render(ctx context.Context, output io.Writer, value T) error
}
```

초기에는 generic interface 없이 format 별 함수로 시작해도 된다.

```go
func WriteJSON(w io.Writer, value any) error
func WriteNDJSON[T any](w io.Writer, values []T) error
func WriteTable(w io.Writer, table Table) error
```

중요한 규칙:

- JSON/NDJSON/CSV 출력은 machine-readable 해야 한다.
- provider 진단 정보는 기본 stdout 에 섞지 않는다.
- `--explain` 이 필요한 경우 result model 에 설명 필드를 명시한다.

## 추천 패키지 배치

```text
internal/
  provider/
    endpoint.go
    capability.go
    registry.go
    errors.go
    interfaces.go

  providers/
    kisbridge/
      bridge.go
      normalize_quote.go
      normalize_daily.go
    datagobridge/
      bridge.go
      normalize_daily.go
      normalize_instrument.go

  service/
    data/
      get_daily.go
      ensure_daily.go
      dependencies.go
    instrument/
      inspect.go
      search.go
      dependencies.go
    calc/
      rsi.go
      dependencies.go

  storage/
    files/
      daily_bar_store.go
      quote_snapshot_store.go
      instrument_store.go
    index/
      coverage_index.go
      manifest_index.go
      provenance_index.go
      latest_quote_index.go

  canonical/
    instrument.go
    daily_bar.go
    quote_snapshot.go
    provenance.go
    keys.go
```

## 초기 구현 순서

1. canonical Go type 을 먼저 만든다.
2. `internal/provider` 에 endpoint, capability, error model 을 만든다.
3. `DailyBarFetcher`, `QuoteSnapshotFetcher`, `InstrumentSearcher` 를 먼저 정의한다.
4. `Registry` 와 selector interface 를 만든다.
5. storage file/index interface 를 작은 역할 단위로 만든다.
6. `data-go` bridge 로 `DailyBarFetcher` 를 먼저 구현한다.
7. `get daily` 또는 `ensure daily` service 를 연결한다.
8. formatter 를 붙여 table/json 출력 차이를 검증한다.

## 관련 문서

- `docs/architectures/directory/README.md`
- `docs/architectures/layers/README.md`
- `docs/architectures/provider/README.md`
- `docs/providers/provider-package-contract.md`
- `docs/canonical-schema.md`
- `docs/go-cli-package-layout.md`
