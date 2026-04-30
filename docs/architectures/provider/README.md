# Provider Architecture

## 목적

이 문서는 `mwosa` 의 provider 아키텍처를 설명하는 가이드다.

`mwosa` 는 KIS, 공공데이터포털, KRX 같은 여러 주식 API provider 를 하나의 CLI 명령 체계로 다룬다. 각 provider 는 지원하는 시장, 자산군, endpoint, 데이터 최신성이 다르다. provider 아키텍처는 이 차이를 CLI 바깥으로 새지 않게 막고, service layer 가 같은 역할 interface 와 canonical data 만 사용하게 만든다.

이 구조에서 provider 구현체는 작은 API client 단위로 본다. provider adapter 는 이 client 를 `mwosa` 안의 역할과 연결하고, provider router 는 요청에 맞는 role interface 를 골라 service 에 넘긴다.

## 용어

이 문서에서는 아래 용어를 사용한다.

| 용어 | 의미 |
| --- | --- |
| provider | `mwosa` 에 데이터를 공급할 수 있는 외부 데이터 소스 또는 그 통합 단위다. 예: `kis`, `datago`, `krx` |
| provider group | 같은 provider 안에서 승인 범위, 인증 조건, 도메인, endpoint 묶음이 달라지는 API 서비스 단위다. 예: `datago` 의 `securitiesProductPrice` |
| operation | provider group 안의 실제 호출 단위다. 예: `getETFPriceInfo`, `getETNPriceInfo` |
| credential scope | 인증 정보가 적용되는 범위다. provider 전체에 적용될 수도 있고, provider group 별로 달라질 수도 있다. |
| provider implementation | 외부 API 를 직접 호출하는 client 구현체다. 인증, pagination, 원본 응답 파싱을 담당하며, 코드에서는 provider client module 로 둔다. |
| provider client module | provider implementation 을 담는 독립 Go module 이다. `mwosa` workspace 안에 두되, 자체 `go.mod` 와 테스트를 가진다. |
| provider adapter | provider implementation 을 `mwosa` 내부 role interface 와 canonical data 로 연결하는 코드다. CLI config 변환, request/result 변환, error 변환, normalization 연결을 담당한다. |
| provider bridge | provider adapter 역할을 설명할 때만 쓰는 보조 표현이다. package, folder, type 이름의 접미사로는 쓰지 않는다. |
| Go workspace | `mwosa` repository root 에 둔 `go.work` 로 여러 Go module 을 함께 개발하는 구조다. CLI module 과 provider client module 을 같은 프로젝트 안에서 독립적으로 관리할 때 사용한다. |
| role interface | service layer 가 의존하는 provider 역할 계약이다. 예: `dailybar.Fetcher`, `quote.Snapshotter`, `instrument.Searcher` |
| role profile | provider 가 어떤 market, security type, freshness, auth 조건에서 해당 역할을 수행할 수 있는지 알려주는 정보다. |
| provider registry | provider 구현체 목록이 아니라 role 후보 목록과 profile 을 관리하는 registry 다. |
| provider router | service layer 의 요청에 맞는 provider role 을 찾고 fallback 순서를 정하는 라우터다. |
| service layer | `inspect`, `get`, `ensure`, `compare`, `screen` 같은 application flow 를 실행하는 레이어다. |
| domain layer | provider, CLI, storage 에 의존하지 않는 순수 도메인 규칙과 계산 레이어다. |

짧게 말하면, provider implementation 은 외부 API 를 호출하고, provider adapter 는 이를 `mwosa` 언어로 번역한다. service/domain layer 는 provider 의 세부 구현을 모르고 앱 동작과 도메인 계산에 집중한다.

provider 이름은 사용자가 고르는 큰 데이터 소스 이름이다. provider group 은 내부 라우팅과 인증 점검을 위한 하위 단위다. 예를 들어 `datago` 는 하나의 provider 이지만, 공공데이터포털에서는 주식시세정보와 증권상품시세정보가 별도 OpenAPI 로 제공되므로 `stockPrice`, `securitiesProductPrice` 같은 group 으로 나눠 관리한다.

## 큰 그림

provider 는 하나의 거대한 객체가 아니다. provider 는 독립적으로 만들고 테스트할 수 있는 작은 API client 를 `mwosa` 의 역할 interface 로 연결한 단위다.

service 는 provider 구현체를 직접 호출하지 않는다. service 는 provider router 에 요청 조건을 넘기고, router 는 registry 에 등록된 후보 중에서 맞는 역할을 고른다. 선택된 역할은 provider adapter 를 지나 provider client 로 이어진다.

```text
service
  -> provider router
  -> provider registry
  -> routed provider role
  -> provider adapter
  -> provider client
  -> provider group operation
  -> normalizer
  -> canonical data
```

service 는 `kis`, `datago`, `krx` 같은 provider 구현체를 직접 알지 않는다. service 는 `dailybar.Fetcher`, `quote.Snapshotter`, `instrument.Searcher` 같은 역할 interface 만 사용한다.

provider client 는 CLI 밖에서도 독립적으로 테스트할 수 있어야 한다. 다만 이 문서에서 말하는 독립 단위는 별도 서버나 마이크로서비스를 뜻하지 않는다. `mwosa` 프로젝트는 Go workspace 로 관리하고, 각 provider client 는 그 workspace 안의 독립 Go module 로 둔다.

## Mwosa Go Workspace

`mwosa` repository root 는 Go workspace 로 관리한다. root 의 `go.work` 는 CLI module 과 provider client module 들을 함께 묶는다.

각 provider client 는 workspace 안에 생성되는 독립 Go module 이다. 각 client module 은 자체 `go.mod` 를 가지고, provider API 호출, 인증, pagination, 원본 응답 파싱, provider-native error 처리를 자기 module 안에서 끝낸다.

이 구조에서는 provider client 를 `mwosa` 에 등록하기 전에 먼저 client module 단위로 테스트한다. 예를 들어 `providers/datago` adapter 를 붙이기 전에 `datago-etp` module 안에서 request builder, fake transport, 응답 파서, error mapping 테스트를 통과시킨다.

provider client 테스트는 CLI module, storage, provider router 에 의존하지 않는다. 외부 API 를 직접 치는 테스트가 필요하면 별도 integration test 로 분리하고, 기본 단위 테스트는 `httptest` 나 fake transport 로 빠르게 실행한다.

provider client 가 독립 테스트를 통과하면 adapter 에서 role profile, request/result 변환, canonical normalization 연결을 작성한다. 즉, 등록 순서는 client 검증, adapter 작성, registry 등록 순서다.

role profile 은 `providers/spec` 의 builder 로 선언한다. provider adapter 가 `RoleProfile` 구조체를 직접 채우면 필수 compatibility metadata 를 빠뜨리기 쉬우므로, builder 의 `Build` 단계에서 role, market, security type, operation, freshness, compatibility 를 검증한다.

## Provider Adapter 구성

provider adapter 는 provider identity 와 embedded role field 로 구성된다. adapter 는 provider client 를 받아 `mwosa` 가 이해하는 역할 interface 구현체로 감싼다.

```go
type Provider struct {
	provider.Identity

	dailybar.Fetcher
	instrument.Searcher
}
```

이 provider 는 `dailybar.Fetcher` 와 `instrument.Searcher` 역할을 제공한다. quote snapshot 을 제공하지 않는 provider 라면 `quote.Snapshotter` 를 임베딩하지 않는다.

quote snapshot 까지 제공하는 provider 는 다음처럼 역할을 더 가진다.

```go
type Provider struct {
	provider.Identity

	quote.Snapshotter
	dailybar.Fetcher
	instrument.Searcher
}
```

embedded role field 만 봐도 provider 가 어떤 역할을 제공하는지 알 수 있다. 지원하지 않는 기능은 unsupported method 로 남기지 않고 role 을 임베딩하지 않는다.

## Role Package

provider 역할 interface, registry, router 는 `providers/core` 아래에 둔다. provider 별 adapter 도 같은 `providers` 아래에 두고, 폴더 이름은 provider 이름을 그대로 쓴다.

```text
providers/
  core/
    identity.go
    registry.go
    selection.go
    errors.go

    dailybar/
      fetch.go
      profile.go
      selection.go

    quote/
      snapshot.go
      profile.go
      selection.go

    instrument/
      search.go
      profile.go
      selection.go

  kis/
    provider.go
    config.go

  datago/
    provider.go
    config.go

  spec/
    compatibility.go
    role.go
```

`providers/core` 는 위치 이름이고, 코드에서는 필요하면 import alias 로 `provider` 라고 읽을 수 있다.

```go
import provider "github.com/<org>/mwosa/providers/core"
```

즉, service 는 `provider.Identity`, `dailybar.Fetcher`, `quote.Snapshotter` 같은 core 계약을 보고, provider 별 구현체 폴더 이름은 `kis`, `datago` 처럼 실제 provider 이름으로 읽는다.

provider 별 폴더는 이미 adapter 역할을 하므로 `kisbridge`, `datagobridge` 처럼 `bridge` 접미사를 붙이지 않는다. `providers/kis`, `providers/datago` 처럼 실제 provider 이름을 그대로 쓰면 경로만 봐도 연결 지점을 알 수 있다.

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

service layer 는 위 interface 에만 의존한다. provider 별 구현 package 이름이나 외부 provider package 타입은 service layer 에 전달하지 않는다.

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

provider 별 구현체는 role interface 를 embedded role field 로 가진다.

```go
package datago

type Provider struct {
	provider.Identity

	dailybar.Fetcher
	instrument.Searcher
}
```

`datago.Provider` 는 embedded field 의 promoted method 로 `FetchDailyBars`, `SearchInstruments` 를 노출할 수 있다. service 는 provider router 를 통해 route 된 role implementation 을 받으므로 provider 구현 타입을 직접 알 필요는 없다.

## Profile

embedded role field 는 provider 가 코드상으로 어떤 역할을 제공하는지 보여준다. 실제 provider 선택에는 실행 시점의 조건이 필요하다. 이 조건은 각 role 의 `Profile` 이 제공한다.

```go
package dailybar

type Profile struct {
	Markets       []provider.Market
	SecurityTypes []provider.SecurityType
	Group         provider.GroupID
	Operations    []provider.OperationID
	AuthScope     provider.CredentialScope
	RangeQuery    RangeQuerySupport
	Freshness      provider.Freshness
	Compatibility  provider.Compatibility
	RequiresAuth  bool
	Priority      int
	Limitations   []string
}
```

예를 들어 두 provider 가 모두 `dailybar.Fetcher` 를 만족해도 지원 범위는 다를 수 있다.

```text
datago
  role: dailybar.Fetch
  group: securitiesProductPrice
  operations: getETFPriceInfo, getETNPriceInfo, getELWPriceInfo
  market: krx
  security_type: etf, etn
  range query: supported
  freshness: daily
  compatibility: previous_business_day, current_day_supported=false

kis
  role: dailybar.Fetch, quote.Snapshot
  market: krx
  security_type: stock, etf
  range query: limited
  freshness: realtime / daily
  compatibility: realtime or end_of_day, depending on role
```

registry 는 embedded role field 에서 role interface 와 profile 을 수집한다. provider router 는 registry 의 후보 목록을 사용해 요청에 맞는 provider role 로 라우팅한다. 이때 provider 이름만 비교하지 않고 group, operation, auth scope 까지 함께 본다.

`Compatibility` 는 provider 선택에서 필수 metadata 다. 같은 `daily` freshness 라도 현재 거래일 데이터를 지원하는지,
D-1 영업일 EOD 인지, historical-only 인지에 따라 사용 가능성이 달라진다. Provider 등록 시 이 값이 없으면
registry 가 거부한다.

## Registry

registry 는 provider instance 의 exported anonymous field 를 검사하면서 `provider.RoleProvider` 를 만족하는 role 을 수집한다.

```go
func (r *Registry) RegisterProvider(p provider.IdentityProvider) error {
	for _, field := range exportedAnonymousFields(p) {
		role, ok := field.Interface().(provider.RoleProvider)
		if !ok {
			continue
		}
		if err := r.Register(p, role.RoleRegistration()); err != nil {
			return err
		}
	}
	return nil
}
```

public role field 가 nil 이면 registry 는 이를 조용히 무시하지 않고 provider 구성 오류로 반환한다. 생성자에서 capability 주입이 빠진 상태를 빨리 발견하기 위해서다.

## Provider Router

provider router 는 service 요청 조건과 role profile 을 비교해 실제로 호출할 수 있는 provider role 후보를 만든다. 단일 provider 하나를 바로 고르는 선택기보다, fallback 순서까지 정하는 라우터로 본다.

router 의 책임:

- 요청한 역할에 맞는 후보를 찾는다. 예: `dailybar.Fetcher`, `quote.Snapshotter`, `instrument.Searcher`
- market, security type, group, operation, 데이터 최신성, 인증, 우선순위를 role profile 과 비교한다.
- 사용자 지정 `--provider` 는 강제 선택으로, `--prefer-provider` 는 우선순위 힌트로 반영한다.
- fallback 가능한 provider 후보 순서를 만든다.
- 후보가 없으면 `ErrNoProvider` 를 반환한다.
- provider 시도 결과와 fallback 사유를 출처 기록(provenance) 또는 explain 결과에 남길 수 있게 한다.

interface 는 두 층으로 둔다. service 가 하나의 role 만 받으면 되는 흐름에서는 `Route*` 메서드를 사용하고, fallback 실행을 직접 제어해야 하는 흐름에서는 `Plan*` 메서드로 후보 순서를 받는다.

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
	Group    provider.GroupID
	Fetcher  Fetcher
	Profile  Profile
	Reason   string
}
```

service 는 router 와 role interface 에만 의존한다. provider client module 타입은 service 에 전달하지 않는다.

## Provider Adapter

provider adapter 는 provider client 와 `mwosa` 내부 role interface 사이를 연결한다. adapter 는 provider 를 대표하는 작은 연결 코드이며, service 에 provider client 타입을 넘기지 않는다.

adapter 의 책임:

- provider client 초기화
- CLI config 를 provider client config 로 변환
- role profile 제공
- provider 원본 request/result 변환
- provider 원본 error 변환
- provider result 를 canonical data 로 normalize

provider 가 REST API 를 쓰는지, SDK 를 쓰는지, 파일을 읽는지는 provider client module 안에서만 다룬다. service 와 provider role interface 에는 특정 HTTP client library type 을 노출하지 않는다.

HTTP client 선택 기준은 `docs/development/README.md` 에 둔다.

adapter 가 하지 않는 일:

- Cobra flag parsing
- SQLite storage 직접 쓰기
- terminal output rendering
- indicator 계산

### datago 예시

```go
package datago

type Provider struct {
	provider.Identity

	dailybar.Fetcher
	instrument.Searcher
}

func New(client *datago.Client) *Provider {
	p := &Provider{
		Identity: provider.Identity{
			ID: provider.ProviderDataGo,
		},
	}

	p.Fetcher = spec.PreviousBusinessDayDailyBar(p.fetchDailyBars).
		Markets(provider.MarketKRX).
		SecurityTypes(provider.SecurityTypeETF, provider.SecurityTypeETN).
		Group(provider.GroupSecuritiesProductPrice).
		Operations(
			provider.OperationGetETFPriceInfo,
			provider.OperationGetETNPriceInfo,
			provider.OperationGetELWPriceInfo,
		).
		RequiresAuth(provider.CredentialScopeDataGo).
		RangeQuery(dailybar.RangeQuerySupported).
		Priority(50).
		MustBuild()

	p.Searcher = spec.PreviousBusinessDayInstrumentSearch(p.searchInstruments).
		Markets(provider.MarketKRX).
		SecurityTypes(provider.SecurityTypeETF, provider.SecurityTypeETN).
		Group(provider.GroupSecuritiesProductPrice).
		Operations(
			provider.OperationGetETFPriceInfo,
			provider.OperationGetETNPriceInfo,
			provider.OperationGetELWPriceInfo,
		).
		RequiresAuth(provider.CredentialScopeDataGo).
		Priority(50).
		MustBuild()

	return p
}
```

### KIS 예시

```go
package kis

type Provider struct {
	provider.Identity

	quote.Snapshotter
	dailybar.Fetcher
	instrument.Searcher
}
```

KIS provider 는 quote snapshot, daily bar, instrument search 역할을 제공한다. datago provider 는 quote snapshot 역할을 제공하지 않는다.

## Provider Group

provider group 은 provider router 가 실제 후보를 고를 때 보는 하위 단위다. group 은 adapter 가 아니라, 한 provider 안에서 API 서비스와 인증 범위를 나누는 이름이다.

사용자는 보통 `--provider datago` 처럼 provider 만 지정한다. 내부에서는 어떤 group 과 operation 이 그 요청을 처리했는지 함께 기록한다.

```text
daily_bar + market=krx + security_type=etf
  -> provider: datago
  -> group: securitiesProductPrice
  -> operation: getETFPriceInfo

daily_bar + market=krx + security_type=stock
  -> provider: datago
  -> group: stockPrice
  -> operation: getStockPriceInfo
```

group 을 두는 이유:

- 같은 provider 안에서도 API 활용신청이 따로 나뉠 수 있다.
- 인증키가 provider 전체에 적용되는지, 특정 API 서비스에만 적용되는지 provider 마다 다르다.
- rate limit, 사용 조건, freshness, pagination 정책이 group 별로 달라질 수 있다.
- 출처 기록(provenance)에 provider 이름만 남기면 어떤 원천 API 에서 온 데이터인지 부족하다.

config 는 provider 전체 설정을 기본값으로 두고, 필요한 경우 group 에서 오버라이드한다.

```json
{
  "providers": {
    "datago": {
      "enabled": true,
      "auth": {
        "service_key": "..."
      },
      "groups": {
        "securitiesProductPrice": {
          "enabled": true
        },
        "stockPrice": {
          "enabled": false
        }
      }
    }
  }
}
```

이 구조에서는 provider 이름에 `-` 나 `/` 를 붙여 하위 API 를 표현하지 않는다. `datago` 는 provider id 로 유지하고, 세부 API 범위는 `group` 필드로 표현한다.

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
  -> provider adapter
  -> provider client
  -> provider group operation
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

이 구조에서 provider 추가, 삭제, 교체는 provider adapter, provider registry, provider router 구성 범위에 머문다.

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

- provider struct 의 embedded role field 는 지원 역할을 보여준다.
- role interface 는 service 가 의존하는 계약이다.
- role profile 은 실행 시점의 provider routing 조건이다.
- provider group 은 같은 provider 안의 승인, 인증, endpoint 묶음이다.
- provider adapter 는 external API 를 canonical data 로 바꾸는 연결 지점이다.
- registry 는 provider 구현체가 아니라 role 후보 목록을 관리한다.
- provider router 는 role 후보 목록을 요청 조건에 맞는 실행 후보로 정렬하고 fallback 계획을 만든다.

## 관련 문서

- `docs/architectures/layers/README.md`
