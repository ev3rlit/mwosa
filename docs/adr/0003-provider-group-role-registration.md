# ADR 0003: Provider group role registration

## 상태

Accepted

## 날짜

2026-05-05

## 맥락

`mwosa` 는 여러 외부 금융 데이터 공급자를 하나의 CLI 경험으로 연결한다. 처음에는
provider adapter 가 `dailybar.Fetcher`, `instrument.Searcher` 같은 role field 를
직접 임베딩하고, registry 가 이 field 를 reflection 으로 읽어 role 후보를 등록했다.

이 방식은 provider 가 몇 개의 role 만 제공할 때 단순하고 잘 맞았다. 하지만
`datago` 처럼 하나의 provider 안에 여러 OpenAPI 활용신청 단위가 있는 경우에는
부족해진다. 공공데이터포털의 `securitiesProductPrice`, `stockPrice`,
`krxListedInstrument`, `corporateFinancial` 은 모두 `datago` 라는 provider 아래에
있지만 승인 상태, endpoint, operation, 제공 role 이 다르다.

또한 같은 provider 안에서 같은 role 을 여러 group 이 제공할 수 있다. 예를 들어
`securitiesProductPrice` 와 `stockPrice` 는 모두 daily bar 성격의 데이터를 제공할
수 있지만, 지원하는 security type 과 operation 은 다르다. 이런 차이를 하나의
provider-level `dailybar.Fetcher` 로 숨기면 provider adapter 내부가 다시 작은 router
처럼 커진다.

기업 재무 정보처럼 실행에 필요한 데이터가 따로 있는 경우도 있다.
`corporateFinancial` group 은 `crno` 같은 등록번호를 요구할 수 있다. 이 요구사항은
`krxListedInstrument` group 에 대한 provider 내부 의존성이 아니라, canonical
instrument store 에 저장된 `crno` 데이터에 대한 의존성으로 보는 편이 맞다.

더 정확히는 데이터 단위의 필드 의존성이다. `instrument` record 가 존재하더라도
`instrument.crno` field 가 비어 있으면 기업 재무 정보 role 은 실행할 수 없다.
반대로 `crno` field 만 확보되어 있으면 그 값이 `krxListedInstrument` 에서 왔든,
다른 provider 에서 왔든, 사용자가 직접 입력했든 기업 재무 정보 조회에 사용할 수
있다.

따라서 provider id 는 큰 공급자 이름으로 유지하고, provider group별로 role 후보를
명시적으로 등록할 수 있어야 한다.

## 결정

`mwosa` 는 provider group별 role registration 을 지원한다.

- provider id 는 계속 큰 공급자 이름으로 둔다. 예: `datago`, `kis`.
- provider group 은 같은 provider 안의 승인 범위, endpoint 묶음, operation 묶음이다.
- provider group adapter 는 자신이 제공하는 role registration 을 만든다.
- provider 는 group adapter 들을 모아 `RoleRegistrations()` 로 registry 에 넘긴다.
- registry 는 provider 가 `RoleRegistrations()` 를 제공하면 이 명시적 등록 목록을 우선 사용한다.
- registry 는 기존 embedded role field reflection 경로를 fallback 으로 유지한다.
- role profile 은 `provider`, `group`, `operation`, `market`, `security_type`, `freshness`, `auth scope`, `priority` 를 포함한다.
- router 는 등록된 role profile 을 기준으로 요청에 맞는 후보를 고른다.
- provider group 은 서로를 직접 import 하거나 호출하지 않는다.
- required data 는 provider group 의 hard dependency 가 아니라 service flow 가 input 또는 canonical store 에서 resolve 할 데이터 의존성으로 둔다.
- 데이터 의존성은 record 의존성과 field 의존성으로 구분한다.
- field 의존성은 `instrument.crno` 처럼 canonical record 의 특정 field 를 가리킨다.
- `crno` 가 없으면 `corporateFinancial` group 을 provider unavailable 로 처리하지 않고, `instrument.crno` field missing 으로 설명한다.

현재 코드 기준으로는 다음 interface 를 사용한다.

```go
type RoleRegistrationProvider interface {
	RoleRegistrations() []RoleRegistration
}

type GroupRoleProvider interface {
	RoleRegistrationProvider
	ProviderGroup() GroupID
}
```

`datago` 의 첫 적용은 `securitiesProductPrice` group 이다. 이 group 은 daily bar 와
instrument search role registration 을 제공한다. `datago.Provider` 는
`[]provider.GroupRoleProvider` 를 모아 registry 에 넘긴다.

```text
datago.Provider
  -> securitiesProductPrice group
    -> daily_bar role registration
    -> instrument role registration
  -> RoleRegistrations()
  -> provider.Registry
  -> provider.Router
```

## 결과

`datago` 는 사용자에게 하나의 provider 로 보인다. CLI 옵션과 provenance 에서도
provider id 는 `datago` 로 유지된다.

동시에 내부에서는 group별로 role 후보를 분리할 수 있다. 나중에 `stockPrice`,
`krxListedInstrument`, `corporateFinancial` group 을 추가해도 기존
`securitiesProductPrice` role 구현을 큰 switch 문으로 키울 필요가 없다.

`kis` 같은 다른 provider 에도 같은 구조를 적용할 수 있다. 예를 들어 시세, 일봉,
계좌, 주문 API 는 같은 `kis` provider 아래에 있더라도 권한 범위와 위험도가 다르므로
group 으로 분리할 수 있다.

기존 provider adapter 는 바로 바꾸지 않아도 된다. registry 는
`RoleRegistrations()` 가 없으면 embedded role field reflection 을 계속 사용한다.
따라서 provider 를 한 번에 모두 이 구조로 옮길 필요가 없다.

데이터 의존성과 provider 의존성도 분리된다. 기업 재무 정보 조회에 `crno` 가 필요하면
service flow 가 먼저 input 이나 canonical instrument store 에서 `crno` 를 찾는다.
`krxListedInstrument` 는 `crno` 를 저장소에 채우는 source 중 하나일 뿐,
`corporateFinancial` 의 직접 하위 모듈이 아니다.

이 구분 덕분에 KRX 상장 기업 목록은 `instrument` record 의 여러 field 를 채울 수
있고, 기업 재무 정보는 그중 `instrument.crno` field 만 요구한다고 표현할 수 있다.
나중에 다른 provider 나 수동 import 가 같은 field 를 채우더라도
`corporateFinancial` group 은 같은 방식으로 동작한다.

## 대안

### Embedded role field 만 유지

provider struct 에 `dailybar.Fetcher`, `instrument.Searcher` 같은 field 를 계속
임베딩하는 방식이다. 단순한 provider 에는 좋지만, 한 provider 안에서 같은 role 을
여러 group 이 제공하는 경우 표현력이 부족하다. group 선택 로직이 role 구현 내부로
들어가 adapter 가 커지기 쉽다.

### Provider 내부 role aggregator

provider 가 하나의 `dailybar.Fetcher` 를 제공하고, 그 내부에서 security type 이나
operation 에 따라 group 을 선택하는 방식이다. service layer 에는 단순하게 보이지만,
provider adapter 내부에 또 다른 router 가 생긴다. group disabled, missing auth,
unsupported operation 을 registry/router 단계에서 설명하기 어렵다.

### Provider id 를 group별로 쪼개기

`datago-securities-product-price`, `datago-stock-price` 처럼 group 을 provider id 에
붙이는 방식이다. 등록은 단순해지지만 사용자에게 보이는 provider 이름이 실제 공급자
단위와 어긋난다. provenance, config, auth, inspect UX 에서도 provider 와 group 의
경계가 흐려진다. 이 방식은 선택하지 않는다.

### Plugin-like group package

각 group 이 거의 독립 plugin 처럼 config spec, client build, role registration 을
모두 소유하는 방식이다. group 수가 크게 늘어나면 검토할 수 있지만, 현재 단계에서는
provider 안에 작은 plugin framework 를 만드는 셈이라 구조가 무겁다.

### Field 의존성을 provider group 의존성으로 취급

`corporateFinancial` 이 `crno` 를 요구한다는 이유로 `krxListedInstrument` group 을
항상 먼저 활성화해야 한다고 보는 방식이다. 구현은 단순해 보이지만, 실제 의존 대상이
`KRX상장종목정보` API 가 아니라 `instrument.crno` field 라는 점을 흐린다. 다른
source 가 `crno` 를 채우거나 사용자가 직접 `crno` 를 제공하는 경우까지 막게 되므로
선택하지 않는다.

## 관련 문서

- `docs/architectures/provider/README.md`
- `docs/architectures/provider/group-role-registration-patterns.md`
- `docs/providers/README.md`
- `docs/providers/datago/README.md`
