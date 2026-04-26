# Provider Architecture

## 목적

이 문서는 `mwosa` 의 provider 아키텍처를 설명하는 가이드다.

`mwosa` 는 KIS, 공공데이터포털, KRX 같은 여러 주식 API provider 를 같은 CLI 명령 체계로 조회한다. 각 provider 는 지원하는 시장, 자산군, endpoint, freshness 가 다르다. provider 아키텍처는 이 차이를 provider bridge 에서 처리하고, service layer 는 일관된 역할 interface 와 canonical data 만 사용하게 한다.

## 용어

이 문서에서는 아래 용어를 기준으로 통일한다.

| 용어 | 의미 |
| --- | --- |
| provider | `mwosa` 에 데이터를 공급할 수 있는 외부 데이터 소스 또는 그 통합 단위다. 예: `kis`, `data-go-etf`, `krx` |
| provider implementation | 실제 외부 API 호출, 인증, pagination, provider-native response parsing 을 담당하는 구현체다. 코드에서는 external provider package 로 둔다. |
| provider adapter | provider implementation 을 `mwosa` 내부 role interface 와 canonical data 로 연결하는 adapter 다. 코드와 문서에서는 provider bridge 라고 부른다. |
| provider bridge | provider adapter 의 공식 명칭이다. CLI config 변환, request/result 변환, error 변환, normalization 연결을 담당한다. |
| role interface | service layer 가 의존하는 provider 역할 계약이다. 예: `dailybar.Fetcher`, `quote.Snapshotter`, `instrument.Searcher` |
| role profile | provider 가 어떤 market, security type, freshness, auth 조건에서 role 을 수행할 수 있는지 나타내는 선택 정보다. |
| provider registry | provider 구현체 목록이 아니라 role 후보 목록과 profile 을 관리하는 registry 다. |
| provider router | service layer 의 요청을 capability-compatible provider role 로 라우팅하고 fallback 후보 순서를 결정하는 컴포넌트다. 단일 후보 선택만 다룰 때는 selector 라고 부를 수 있지만, 문서와 설계의 기본 용어는 router 다. |
| service layer | `inspect`, `get`, `ensure`, `compare`, `screen` 같은 application flow 를 실행하는 레이어다. |
| domain layer | provider, CLI, storage 에 의존하지 않는 순수 도메인 규칙과 계산 레이어다. |

짧게 말하면, provider implementation 은 외부 API 를 호출하고, provider bridge 는 이를 `mwosa` 언어로 번역하며, service/domain layer 는 실제 앱 동작과 도메인 계산을 수행한다.

## 큰 그림

provider 는 하나의 거대한 객체가 아니라 여러 역할을 가진 객체다. 아래 흐름에서 `provider bridge` 는 provider adapter 이고, `external provider package` 는 provider implementation 이다.

```text
service
  -> provider registry
  -> provider router
  -> routed provider role
  -> provider bridge
  -> external provider package
  -> normalizer
  -> canonical data
```

service 는 `kis`, `data-go-etf`, `krx` 같은 provider 구현체를 직접 알지 않는다. service 는 `dailybar.Fetcher`, `quote.Snapshotter`, `instrument.Searcher` 같은 역할 interface 만 사용한다.

## Provider 구성

provider bridge 는 provider identity 와 embedded role 로 구성된다.

```go
type Provider struct {
	provider.Identity

	dailybar.Fetch
	instrument.Search
}
```

이 provider 는 `dailybar.Fetch` 와 `instrument.Search` 역할을 제공한다. quote snapshot 을 제공하지 않는 provider 라면 `quote.Snapshot` field 를 가지지 않는다.

quote snapshot 까지 제공하는 provider 는 다음처럼 역할을 더 가진다.

```go
type Provider struct {
	provider.Identity

	quote.Snapshot
	dailybar.Fetch
	instrument.Search
}
```

embedded field 만 봐도 provider 가 어떤 역할 interface 를 만족하는지 알 수 있다. 지원하지 않는 기능은 unsupported method 로 남기지 않고 field 자체를 두지 않는다.

## Role Package

provider 역할은 endpoint 성격에 맞춰 package 로 나눈다.

```text
provider/
  identity.go
  registry.go
  selection.go
  errors.go

provider/dailybar/
  fetch.go
  profile.go
  selection.go

provider/quote/
  snapshot.go
  profile.go
  selection.go

provider/instrument/
  search.go
  profile.go
  selection.go
```

Go package 이름이 문맥을 제공하므로 type 이름은 짧게 둔다.

- `dailybar.Fetch`
- `dailybar.Fetcher`
- `dailybar.Profile`
- `quote.Snapshot`
- `quote.Snapshotter`
- `quote.Profile`
- `instrument.Search`
- `instrument.Searcher`
- `instrument.Profile`

코드 타입명에는 `Capability` 접미사를 사용하지 않는다. 문서에서는 설명상 “지원 역할”이라고 부를 수 있지만, Go 코드에서는 package 와 type 이름으로 역할을 표현한다.

## Role Interface

각 role package 는 service 가 사용하는 interface 를 제공한다.

```go
package dailybar

type Fetcher interface {
	FetchDailyBars(ctx context.Context, input FetchInput) (FetchResult, error)
	DailyBarProfile() Profile
}
```

```go
package quote

type Snapshotter interface {
	FetchQuoteSnapshot(ctx context.Context, input SnapshotInput) (SnapshotResult, error)
	QuoteProfile() Profile
}
```

```go
package instrument

type Searcher interface {
	SearchInstruments(ctx context.Context, input SearchInput) (SearchResult, error)
	InstrumentSearchProfile() Profile
}
```

service layer 는 위 interface 에만 의존한다. provider bridge package 이름이나 외부 provider package 타입은 service layer 에 전달하지 않는다.

## Embedded Role Struct

role 은 concrete struct 로 구현한다.

```go
package dailybar

type Fetch struct {
	profile Profile
	fetch   FetchFunc
}

type FetchFunc func(context.Context, FetchInput) (FetchResult, error)

func NewFetch(profile Profile, fetch FetchFunc) Fetch {
	return Fetch{
		profile: profile,
		fetch:   fetch,
	}
}

func (f Fetch) FetchDailyBars(ctx context.Context, input FetchInput) (FetchResult, error) {
	return f.fetch(ctx, input)
}

func (f Fetch) DailyBarProfile() Profile {
	return f.profile
}
```

provider bridge 는 이 role struct 를 embed 한다.

```go
package datagobridge

type Provider struct {
	provider.Identity

	dailybar.Fetch
	instrument.Search
}
```

Go 의 method promotion 때문에 `datagobridge.Provider` 는 자연스럽게 `dailybar.Fetcher` 와 `instrument.Searcher` 를 만족한다.

## Profile

embedded role 은 provider 가 코드상으로 어떤 역할을 제공하는지 보여준다. 실제 provider 선택에는 실행 시점의 조건이 필요하다. 이 조건은 각 role 의 `Profile` 이 제공한다.

```go
package dailybar

type Profile struct {
	Markets       []provider.Market
	SecurityTypes []provider.SecurityType
	RangeQuery    RangeQuerySupport
	Freshness      provider.Freshness
	RequiresAuth  bool
	Priority      int
	Limitations   []string
}
```

예를 들어 두 provider 가 모두 `dailybar.Fetcher` 를 만족해도 지원 범위는 다를 수 있다.

```text
data-go-etf
  role: dailybar.Fetch
  market: krx
  security_type: etf, etn
  range query: supported
  freshness: daily

kis
  role: dailybar.Fetch, quote.Snapshot
  market: krx
  security_type: stock, etf
  range query: limited
  freshness: realtime / daily
```

registry 는 role interface 와 profile 을 수집한다. provider router 는 registry 의 후보 목록을 사용해 요청에 맞는 provider role 로 라우팅한다.

## Registry

registry 는 provider instance 를 등록하면서 어떤 role interface 를 만족하는지 수집한다.

```go
func (r *Registry) Register(p provider.IdentityProvider, impl any) error {
	if fetcher, ok := impl.(dailybar.Fetcher); ok {
		r.dailyBars = append(r.dailyBars, dailybar.Entry{
			Provider: p.Identity(),
			Fetcher:  fetcher,
			Profile:  fetcher.DailyBarProfile(),
		})
	}

	if snapshotter, ok := impl.(quote.Snapshotter); ok {
		r.quotes = append(r.quotes, quote.Entry{
			Provider:    p.Identity(),
			Snapshotter: snapshotter,
			Profile:    snapshotter.QuoteProfile(),
		})
	}

	return nil
}
```

## Provider Router

provider router 는 service 요청 조건과 role profile 을 비교해 capability-compatible provider role 후보를 만든다. 단일 provider 를 바로 반환하는 selector 보다, fallback 가능한 후보 순서까지 결정하는 router 로 보는 것이 기본이다.

router 의 책임:

- 요청 capability 에 맞는 role 후보를 찾는다. 예: `dailybar.Fetcher`, `quote.Snapshotter`, `instrument.Searcher`
- market, security type, freshness, auth, priority 를 role profile 과 비교한다.
- 사용자 지정 `--provider` 는 강제 선택으로, `--prefer-provider` 는 우선순위 힌트로 반영한다.
- fallback 가능한 provider 후보 순서를 만든다.
- 후보가 없으면 `ErrNoProvider` 를 반환한다.
- provider 시도 결과와 fallback 사유를 provenance 또는 explain 결과에 남길 수 있게 한다.

interface 는 두 층으로 둘 수 있다. service 가 단순한 use case 라면 단일 role 을 받는 `Route*` 메서드를 사용하고, fallback 실행을 직접 제어해야 하는 use case 라면 `Plan*` 메서드로 후보 순서를 받는다.

```go
type Router interface {
	RouteDailyBars(ctx context.Context, input dailybar.RouteInput) (dailybar.Fetcher, error)
	RouteQuoteSnapshot(ctx context.Context, input quote.RouteInput) (quote.Snapshotter, error)
	RouteInstrumentSearch(ctx context.Context, input instrument.RouteInput) (instrument.Searcher, error)

	PlanDailyBars(ctx context.Context, input dailybar.RouteInput) (dailybar.RoutePlan, error)
	PlanQuoteSnapshot(ctx context.Context, input quote.RouteInput) (quote.RoutePlan, error)
	PlanInstrumentSearch(ctx context.Context, input instrument.RouteInput) (instrument.RoutePlan, error)
}
```

route plan 은 fallback 순서를 명시한다.

```go
package dailybar

type RoutePlan struct {
	Candidates []RouteCandidate
}

type RouteCandidate struct {
	Provider provider.Identity
	Fetcher  Fetcher
	Profile  Profile
	Reason   string
}
```

service 는 router 와 role interface 에만 의존한다. provider bridge package 이름이나 external provider package 타입은 service 에 전달하지 않는다.

## Provider Bridge

provider bridge 는 external provider package 와 `mwosa` 내부 role interface 사이의 adapter 다.

bridge 의 책임:

- external provider client 초기화
- CLI config 를 external package config 로 변환
- role profile 제공
- provider 원본 request/result 변환
- provider 원본 error 변환
- provider result 를 canonical data 로 normalize

bridge 가 하지 않는 일:

- Cobra flag parsing
- local file storage 직접 쓰기
- SurrealDB index 직접 갱신
- terminal output rendering
- indicator 계산

### data-go ETF 예시

```go
package datagobridge

type Provider struct {
	provider.Identity

	dailybar.Fetch
	instrument.Search
}

func New(client *datago.Client) *Provider {
	return &Provider{
		Identity: provider.Identity{
			Name: "data-go-etf",
		},
		Fetch: dailybar.NewFetch(dailybar.Profile{
			Markets:       []provider.Market{"krx"},
			SecurityTypes: []provider.SecurityType{"etf", "etn"},
			RangeQuery:    dailybar.RangeQuerySupported,
			Freshness:      provider.FreshnessDaily,
			RequiresAuth:  true,
			Priority:      50,
		}, func(ctx context.Context, input dailybar.FetchInput) (dailybar.FetchResult, error) {
			result, err := client.FetchDailyPrices(ctx, toDataGoRequest(input))
			if err != nil {
				return dailybar.FetchResult{}, mapDataGoError(err)
			}
			return normalizeDailyBars(result)
		}),
		Search: instrument.NewSearch(...),
	}
}
```

### KIS 예시

```go
package kisbridge

type Provider struct {
	provider.Identity

	quote.Snapshot
	dailybar.Fetch
	instrument.Search
}
```

KIS provider 는 quote snapshot, daily bar, instrument search 역할을 제공한다. data-go ETF provider 는 quote snapshot 역할을 제공하지 않는다.

## Unsupported 처리

지원하지 않는 기능은 provider method 가 아니라 provider routing 결과로 표현한다.

```text
no provider supports quote.Snapshot for market=krx security_type=etf
```

규칙:

- 지원하지 않는 역할은 provider struct 에 field 로 존재하지 않는다.
- provider router 는 후보가 없을 때 `ErrNoProvider` 를 반환한다.
- provider 내부 API 가 특정 조건을 거절하면 role profile 과 error 가 일치해야 한다.
- 빈 result 를 성공처럼 반환하지 않는다.

## Service 에서 보는 구조

`get daily` service 는 provider 구현체를 직접 모른다.

```go
type GetDailyService struct {
	DailyBars dailybar.Router
	Store     DailyBarStore
	Coverage  CoverageIndex
}
```

실행 흐름:

```text
GetDailyService
  -> dailybar.Router
  -> routed dailybar.Fetcher or dailybar.RoutePlan
  -> provider bridge
  -> external provider package
  -> canonical daily bars
  -> store/index
```

`calc rsi` service 는 provider router 도 직접 알 필요가 없다. daily series 확보 use case 에만 의존한다.

```go
type DailySeriesEnsurer interface {
	EnsureDailyBars(ctx context.Context, input EnsureDailyBarsInput) ([]canonical.DailyBar, error)
}

type RSICalculator struct {
	Series DailySeriesEnsurer
}
```

이 구조에서 provider 추가, 삭제, 교체는 provider bridge, provider registry, provider router 구성 범위에 머문다.

## Naming

provider role naming 은 package 이름과 type 이름이 함께 읽히도록 구성한다.

```text
dailybar.Fetch
dailybar.Fetcher
dailybar.Profile

quote.Snapshot
quote.Snapshotter
quote.Profile

instrument.Search
instrument.Searcher
instrument.Profile
```

`Capability`, `Feature`, `Module` 같은 접미사는 코드 타입명에 사용하지 않는다. 역할 자체가 package 경계로 드러나기 때문이다.

## 읽을 때의 기준

이 문서를 읽을 때는 다음 경계를 기준으로 보면 된다.

- provider struct 의 embedded field 는 지원 역할을 보여준다.
- role interface 는 service 가 의존하는 계약이다.
- role profile 은 실행 시점의 provider routing 조건이다.
- provider bridge 는 external API 를 canonical data 로 바꾸는 adapter 다.
- registry 는 provider 구현체가 아니라 role 후보 목록을 관리한다.
- provider router 는 role 후보 목록을 capability-compatible 실행 후보로 정렬하고 fallback 계획을 만든다.

## 관련 문서

- `docs/architectures/layers/README.md`
- `docs/providers/provider-package-contract.md`
