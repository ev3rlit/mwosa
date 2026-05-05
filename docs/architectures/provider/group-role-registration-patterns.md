# Provider Group Role Registration Patterns

## 목적

이 문서는 하나의 provider 안에 여러 OpenAPI 활용신청 단위가 있는 경우
provider client 와 role registration 을 어떻게 구성할지 비교한다.

대표 사례는 `datago` 다. 공공데이터포털은 같은 공급자 안에서도
`주식시세정보`, `증권상품시세정보`, `KRX상장종목정보` 처럼 OpenAPI 서비스가
분리되고, 활용신청과 승인 상태도 따로 관리된다. 그래서 `mwosa` 는 provider
이름을 `datago` 하나로 유지하되, 실제 권한과 endpoint 묶음은 provider group
으로 나누어야 한다.

이 문서는 구현 결정을 확정하는 ADR 이 아니라, 다음 구현 전에 선택지를 검토하기
위한 설계 메모다.

현재 채택된 결정은 `docs/adr/0003-provider-group-role-registration.md` 에 기록한다.

## 판단 기준

비교할 때는 아래 기준을 우선한다.

| 기준 | 질문 |
| --- | --- |
| 활용신청 단위 표현 | OpenAPI별 승인 상태와 enabled 상태를 독립적으로 표현할 수 있는가? |
| registry 정확도 | router 가 `provider`, `group`, `operation`, `security_type` 을 보고 정확한 후보를 고를 수 있는가? |
| service layer 격리 | service layer 에 Datago 의 OpenAPI 서비스명이 새지 않는가? |
| client 독립성 | group별 client 를 CLI 와 분리해 테스트할 수 있는가? |
| config 확장성 | provider 기본값과 group별 override 를 함께 다룰 수 있는가? |
| 오류 설명력 | 권한 없음, group disabled, unsupported security type 을 명확히 설명할 수 있는가? |
| 데이터 의존성 | 특정 role 이 `crno` 같은 저장된 식별자나 reference data 를 필요로 할 때 이를 표현할 수 있는가? |
| 구현 복잡도 | 현재 reflection 기반 role registration 과 크게 충돌하지 않는가? |

## 전제

provider id 는 큰 데이터 소스 이름이다.

```text
provider: datago
```

provider group 은 같은 provider 안에서 승인 범위, 인증 조건, endpoint 묶음이
달라지는 API 서비스 단위다.

```text
provider: datago
group: securitiesProductPrice
operations: getETFPriceInfo, getETNPriceInfo, getELWPriceInfo

provider: datago
group: stockPrice
operations: getStockPriceInfo

provider: datago
group: krxListedInstrument
operations: ...

provider: datago
group: corporateFinancial
operations: ...
```

operation 은 provider group 안의 실제 endpoint 호출 단위다.

service layer 는 `datago`, `securitiesProductPrice` 같은 구현 이름을 직접
호출하지 않고 `dailybar.Fetcher`, `instrument.Searcher` 같은 role interface 만
사용한다.

일부 role 은 provider group 이 아니라 저장된 데이터에 의존할 수 있다. 예를 들어
기업 재무 정보 API 가 `crno` 를 요구한다면, 재무 정보 role 의 의존성은
`KRX상장종목정보` group 자체가 아니라 canonical instrument store 에 저장된 `crno`
다. `KRX상장종목정보` group 은 종목코드와 `crno` 를 연결하는 데이터를 채우는
source 중 하나로 본다.

이때 의존성은 record 전체보다 더 작은 field 단위일 수 있다. 기업 재무 정보는
`instrument` record 전체가 아니라 `instrument.crno` field 를 요구한다. 따라서
종목명, 종목코드, ISIN 이 저장되어 있어도 `crno` field 가 비어 있으면 재무 정보만
unavailable 로 설명해야 한다.

따라서 기업 재무 정보 group 을 `KRX상장종목정보` group 에 강하게 묶기보다,
`crno` 가 있으면 재무제표와 매출 정보를 조회하고 없으면 해당 데이터만
unavailable 로 표시하는 데이터 의존성으로 보는 편이 안전하다.

## 패턴 1: 단일 Provider Client

하나의 `datago.Client` 가 모든 group operation 을 가진다.

```text
clients/datago/
  client.go
  securities_product_price.go
  stock_price.go
  krx_listed_instrument.go
```

```go
client.GetETFPriceInfo(ctx, query)
client.GetStockPriceInfo(ctx, query)
client.GetKRXListedItems(ctx, query)
```

장점:

- Datago 공통 base URL, service key, HTTP transport, retry 정책을 공유하기 쉽다.
- module 수가 적고 초기 작성이 단순하다.
- 공통 envelope decode 를 한 곳에 둘 수 있다.

단점:

- 활용신청 단위와 코드 단위가 어긋난다.
- `serviceKey` 는 있어도 특정 OpenAPI 권한이 없을 수 있는 상태를 표현하기 어렵다.
- 시간이 지나면 `datago.Client` 가 여러 서비스 책임을 가진 큰 타입이 되기 쉽다.
- group별 e2e fixture 와 quota 정책이 한 module 안에서 섞일 수 있다.

판단:

초기 실험에는 단순하지만, `datago` 처럼 OpenAPI 서비스가 계속 늘어나는 provider
에는 장기 구조로 약하다.

## 패턴 2: Group 별 독립 Client Module

OpenAPI 활용신청 단위마다 독립 client module 을 둔다.

```text
clients/datago-etp/
clients/datago-stock-price/
clients/datago-krx-listed-instrument/

providers/datago/
```

각 module 은 자체 `go.mod`, OpenAPI spec, request builder, parser, fake
transport test, live e2e test 를 가진다.

장점:

- 활용신청 단위와 코드 단위가 일치한다.
- group별 승인 상태, quota, e2e fixture 를 분리하기 좋다.
- provider adapter 에 붙이기 전 client 단위로 독립 검증할 수 있다.
- 나중에 특정 client 를 별도 repository 로 분리하기 쉽다.

단점:

- module 수가 늘어난다.
- Datago 공통 envelope decode, retry, query encoding 코드가 반복될 수 있다.
- 공통 코드를 너무 빨리 분리하면 별도 framework 처럼 커질 수 있다.

판단:

현재 `clients/datago-etp` 가 이미 이 방향이다. Datago group 이 늘어나는 경우에도
가장 현실과 잘 맞는 client 경계다.

## 패턴 3: Group Client Interface 와 Composite Adapter

client module 은 group별로 두되, `providers/datago` adapter 는 concrete client
타입보다 group client interface 에 의존한다.

```go
type SecuritiesProductPriceClient interface {
	GetETFPriceInfo(context.Context, ETFPriceInfoQuery) (ETFPriceInfoResult, error)
	GetETNPriceInfo(context.Context, ETNPriceInfoQuery) (ETNPriceInfoResult, error)
}

type StockPriceClient interface {
	GetStockPriceInfo(context.Context, StockPriceInfoQuery) (StockPriceInfoResult, error)
}
```

`datago.Provider` 는 enabled group 의 client 를 받아 role 구현을 합성한다.

장점:

- adapter test 에서 fake client 를 넣기 쉽다.
- provider adapter 와 client module 의 결합도가 낮다.
- group별 client 가 아직 구현되지 않았어도 adapter 설계를 먼저 검증할 수 있다.

단점:

- 작은 group 에도 interface 가 추가되어 코드가 늘어난다.
- interface 위치와 naming 기준을 정하지 않으면 adapter package 가 산만해질 수 있다.
- 현재 group 이 하나뿐일 때는 과한 구조처럼 보일 수 있다.

판단:

패턴 2와 함께 쓰기 좋다. 다만 interface 는 provider adapter 가 실제로 테스트
경계를 필요로 하는 곳에만 둔다.

## 패턴 4: Role 별 Aggregator

`datago` adapter 안에서 role 구현이 group 을 선택한다.

```text
datago dailybar role
  security_type=etf   -> securitiesProductPrice
  security_type=etn   -> securitiesProductPrice
  security_type=stock -> stockPrice

datago instrument role
  security_type=etf   -> securitiesProductPrice
  security_type=stock -> stockPrice or krxListedInstrument
```

장점:

- service layer 에는 하나의 `dailybar.Fetcher`, 하나의 `instrument.Searcher` 로 보인다.
- provider 내부에서 group 선택 규칙을 감출 수 있다.
- 기존 embedded role field 구조와 비교적 잘 맞는다.

단점:

- role 구현 내부가 작은 router 처럼 변한다.
- group별 권한 없음과 unsupported 를 정교하게 드러내지 않으면 디버깅이 어렵다.
- provider router 의 책임과 provider 내부 routing 책임이 섞일 수 있다.

판단:

group 수가 적고 role 당 선택 규칙이 단순하면 쓸 수 있다. 하지만 group 이 늘어나면
router 가 이미 해야 할 일을 adapter 내부에서 다시 구현하게 될 위험이 있다.

## 패턴 5: Group 별 Role Registration

provider id 는 `datago` 하나로 유지하되, registry entry 는 group별 role 후보로
등록한다.

```text
provider=datago group=securitiesProductPrice role=daily_bar security_type=etf
provider=datago group=securitiesProductPrice role=daily_bar security_type=etn
provider=datago group=stockPrice role=daily_bar security_type=stock
provider=datago group=krxListedInstrument role=instrument security_type=stock
provider=datago group=corporateFinancial role=fundamentals security_type=stock
```

이 방식에서 router 는 provider 이름만 보는 것이 아니라 role profile 의 group,
operation, market, security type, freshness, auth scope 를 함께 본다.

예상 구조:

```text
clients/
  datago-etp/
  datago-stock-price/
  datago-krx-listed-instrument/

providers/datago/
  builder.go
  provider.go
  config.go
  groups.go
  securities_product_price.go
  stock_price.go
  krx_listed_instrument.go
```

`providers/datago` 는 enabled group 만 조립하고, 각 group adapter 는 자신이 제공할
role registration 을 만든다.

```go
type GroupRoleProvider interface {
	ProviderGroup() provider.GroupID
	RoleRegistrations() []provider.RoleRegistration
}
```

예를 들어 `securitiesProductPrice` group 은 ETF/ETN/ELW daily bar 와 instrument
role 을 등록하고, `stockPrice` group 은 stock daily bar 와 instrument role 을
등록한다.

`crno` 요구사항은 registration 자체를 막는 provider dependency 로 먼저 보지 않는다.
대신 role profile 과 route plan 에 필요한 데이터 조건을 드러낸다. 기업 재무 정보
group 이 `crno` 를 요구하는 경우, 이 group 은 `fundamentals` role 후보로 등록될 수
있지만 실행 단계에서는 아래 데이터 중 하나가 필요하다.

- 사용자가 `crno` 를 직접 넘긴다.
- canonical instrument store 에 해당 종목의 `crno` 가 이미 있다.
- `KRX상장종목정보` 같은 source 를 먼저 동기화해 canonical instrument store 에 `crno` 를 저장했다.

이렇게 하면 `corporateFinancial` group 은 독립적으로 활용신청, 설정, 테스트할 수
있고, `KRX상장종목정보` group 은 기업 재무 정보의 필수 하위 모듈이 아니라
canonical instrument data 를 채우는 source 로 동작한다.

장점:

- 공공데이터포털의 활용신청 단위와 registry 후보 단위가 일치한다.
- group disabled, missing auth, unsupported operation 을 router/inspect 단계에서 설명하기 좋다.
- `provider=datago` 는 유지하면서 provenance 에 `group` 과 `operation` 을 정확히 남길 수 있다.
- 같은 role 을 여러 group 이 제공해도 priority 와 compatibility 로 비교할 수 있다.
- `inspect provider datago` 에서 group별 상태와 제공 role 을 자연스럽게 보여줄 수 있다.
- `crno` 같은 식별자 요구사항을 provider dependency 대신 데이터 의존성으로 표현할 수 있다.

단점:

- 현재 embedded role field reflection 구조와 그대로 맞지는 않는다.
- 한 provider 안에서 같은 role 을 여러 번 등록할 수 있는 명시적 registration API 가 필요하다.
- `Provider struct` 만 봐서는 group별 role 후보가 모두 드러나지 않을 수 있다.
- registry, inspect, config spec 이 group 상태를 더 깊게 이해해야 한다.
- route plan 이 단순 후보 선택을 넘어 missing identifier 와 데이터 확보 방법을 설명해야 한다.

판단:

Datago 요구에는 가장 정확하다. 다만 현재의 “embedded role field 가 source of
truth” 라는 단순 모델을 일부 확장해야 한다. 구현한다면 기존 reflection 등록을
버리지 말고, 명시적 group role registration 을 추가 경로로 열어두는 편이 안전하다.

## 데이터 의존성 처리

의존성은 provider runtime dependency 와 데이터 의존성으로 나눈다. 데이터 의존성은
다시 record 의존성과 field 의존성으로 나눠서 표현한다.

| 종류 | 의미 | 예시 | 처리 방식 |
| --- | --- | --- | --- |
| provider runtime dependency | 대상 구성 요소가 없으면 client 자체를 만들 수 없다. | 공통 OAuth token issuer 가 반드시 필요한 API | build 단계에서 실패하거나 group 을 등록하지 않는다. |
| record 의존성 | role 실행 전에 특정 canonical record 가 필요하다. | 특정 symbol 의 canonical `instrument` record | role 은 등록하되 실행 전에 repository 에서 record 를 찾는다. |
| field 의존성 | role 실행 전에 canonical record 의 특정 field 가 필요하다. | 기업 재무 정보 조회에 필요한 `instrument.crno` | field 가 없으면 해당 데이터만 unavailable 로 설명한다. |

Datago 의 기업 재무 정보는 provider dependency 가 아니라 데이터 의존성으로 본다.
기업 재무 정보 OpenAPI 는 `crno` 를 요구할 수 있지만, `crno` 는 여러 경로에서 올 수
있다. `KRX상장종목정보` 는 그 데이터를 채우는 source 중 하나일 뿐이다.

```text
mwosa sync instruments --market krx --provider datago --group krxListedInstrument
  -> KRX 상장 기업 목록 수집
  -> canonical instrument store 에 symbol, name, crno 저장

mwosa get financials 005930 --provider datago
  -> canonical instrument store 에서 symbol=005930 의 crno 조회
  -> crno 가 있으면 datago group=corporateFinancial 로 재무제표 조회
  -> crno 가 없으면 financials 데이터만 unavailable 로 설명
```

이 흐름에서 `corporateFinancial` 은 `krxListedInstrument` 를 직접 호출하지 않는다.
service flow 가 먼저 repository 에서 필요한 `crno` 를 찾고, 확보된 `crno` 를 기업
재무 정보 role 에 넘긴다. `crno` 를 확보하지 못하면 빈 성공을 반환하지 않고,
다음처럼 설명 가능한 unavailable 상태를 만든다.

```text
provider=datago
group=corporateFinancial
role=fundamentals
status=unavailable
required_field=instrument.crno
reason=required field is missing
suggestion=sync KRX listed instruments or provide crno directly
```

이 방식의 장점은 강한 provider 내부 결합을 만들지 않는다는 점이다. 사용자가
`corporateFinancial` 만 활용신청했고 `crno` 를 직접 알고 있다면 기업 재무 정보는
동작할 수 있다. 반대로 `KRX상장종목정보` 만 활성화되어 있으면 재무 정보는
비활성으로 보이지만 KRX 상장 기업 목록과 instrument metadata 는 저장할 수 있다.

따라서 group role registration 에는 다음 metadata 를 추가로 검토한다.

- role 이 요구하는 입력 데이터: 예: `symbol`, `isin`, `crno`
- role 이 직접 받을 수 있는 identifier
- role 실행 전에 조회할 repository 데이터
- role 실행 전에 필요한 canonical field: 예: `instrument.crno`
- 데이터가 없을 때의 unavailable reason
- `inspect provider` 에 표시할 required 데이터 관계

중요한 원칙은 provider group 이 서로를 직접 import 하거나 호출하지 않게 하는
것이다. group 은 자신이 제공하는 role 과 필요한 입력 데이터를 선언하고, 조합은
repository 를 포함한 service flow 에서 처리한다.

## 패턴 6: Plugin-like Group Package

각 group package 가 자기 config spec, client build, role registration 을 거의
플러그인처럼 제공한다.

```text
providers/datago/groups/securitiesproductprice/
providers/datago/groups/stockprice/
providers/datago/groups/krxlistedinstrument/
```

```go
securitiesproductprice.Register(registry, config)
stockprice.Register(registry, config)
```

장점:

- group 추가가 독립적이다.
- package 경계만 보면 어떤 OpenAPI 서비스가 붙어 있는지 알기 쉽다.
- group 수가 많아져도 파일 충돌이 적다.

단점:

- 현재 provider builder 보다 추상화가 크다.
- provider 안에 작은 plugin framework 를 만드는 모양이 될 수 있다.
- 공통 config, 공통 HTTP 정책, inspect 출력이 흩어질 수 있다.

판단:

group 이 많이 늘어난 뒤에는 검토할 수 있다. 지금 단계에서는 구조가 무겁다.

## 비교 요약

| 패턴 | 활용신청 단위 | registry 정확도 | 구현 복잡도 | 현재 적합도 |
| --- | --- | --- | --- | --- |
| 단일 provider client | 약함 | 보통 | 낮음 | 낮음 |
| group별 client module | 강함 | 보통 | 보통 | 높음 |
| group client interface + composite adapter | 강함 | 보통 | 보통 | 높음 |
| role 별 aggregator | 보통 | 보통 | 보통 | 중간 |
| group별 role registration | 강함 | 강함 | 높음 | 가장 높음 |
| plugin-like group package | 강함 | 강함 | 높음 | 보류 |

## 현재 후보 결정

현재 유력 후보는 패턴 5다.

다만 client 경계는 패턴 2를 따른다. 즉, OpenAPI 활용신청 단위마다 독립 client
module 을 두고, provider adapter 에서는 group별 role registration 으로 registry
후보를 만든다.

```text
provider id: datago
client unit: OpenAPI service / provider group
registration unit: provider + group + role + operation profile
service dependency: role interface only
data dependency: required identifiers are resolved from input or canonical store
```

이 방향을 택하면 `datago` 는 사용자에게 하나의 provider 로 보이지만, 내부에서는
`securitiesProductPrice`, `stockPrice`, `krxListedInstrument` 를 각각 독립적으로
활성화하고 진단할 수 있다.

현재 구현 기준은 이렇다.

- registry 는 provider 가 `RoleRegistrations()` 를 제공하면 그 명시적 목록을 우선 등록한다.
- 기존 embedded role field reflection 경로는 그대로 유지한다.
- `datago` 는 `securitiesProductPrice` group 이 daily bar 와 instrument role registration 을 제공한다.
- 아직 추가하지 않은 group 은 client module 과 group adapter 가 생길 때 같은 방식으로 붙인다.

## 구현 시 확인할 질문

- `ProviderBuilder.Build` 는 provider 하나를 반환해야 하는가, 아니면 group별
  role registration bundle 을 반환할 수 있어야 하는가?
- `ConfigSpec` 는 group별 required auth 를 표현할 수 있어야 하는가?
- role profile 은 `crno` 같은 required 데이터를 표현해야 하는가?
- `crno` 조회는 provider role 로 둘 것인가, canonical store lookup 을 먼저 보는
  service dependency 로 둘 것인가?
- group별 `enabled=false` 는 provider build 단계에서 제외할 것인가, registry 단계에서
  disabled decision 으로 남길 것인가?
- `inspect provider datago` 는 등록된 role 만 보여줄 것인가, disabled/missing group 도
  함께 보여줄 것인가?
- 같은 security type 을 여러 group 이 제공할 때 priority 는 group adapter 가 정할 것인가,
  router policy 가 정할 것인가?
- required 데이터가 없어서 일부 데이터가 비활성일 때 CLI JSON 은 어떤 상태 값을
  반환할 것인가?

## 권장 다음 단계

1. `providers/core` 에 group별 explicit role registration API 를 작게 추가한다.
2. `providers/datago` builder 가 `groups.<group>.enabled` 와 group auth override 를 읽게 한다.
3. `securitiesProductPrice` 를 먼저 새 registration 방식으로 옮긴다.
4. 이후 `stockPrice`, `krxListedInstrument` 는 각각 독립 client module 로 추가한다.
5. 기업 재무 정보 group 은 `crno` 를 required 데이터로 선언하고, KRX 상장 기업 목록은 canonical instrument store 에 저장한다.
6. `inspect provider datago` 에 group별 configured/registered/required-data 상태를 보여주는 출력을 설계한다.
