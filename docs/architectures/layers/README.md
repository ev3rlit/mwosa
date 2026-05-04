# Layer Architecture

## 목적

이 문서는 `mwosa` CLI 의 레이어 아키텍처 뼈대를 정의한다. 디렉터리 구조는 별도 기준으로 고정하지 않고, 이 문서 안에서 레이어를 구현하는 예시로만 다룬다.

기본 관점은 웹서버의 클린 아키텍처를 CLI에 적용한 것이다. HTTP router, controller, web response 가 있던 자리에 command parser, CLI handler, terminal presentation 이 들어온다.

```text
web server:
  route -> handler/controller -> middleware -> service/use case -> domain -> persistence -> presenter/response

mwosa CLI:
  command -> cli handler -> middleware -> service/use case -> domain -> persistence -> presentation
```

## 레이어 목록

초기 레이어는 다음으로 나눈다.

- `command layer`
- `cli handler layer`
- `middleware layer`
- `service layer`
- `domain layer`
- `persistence layer`
- `presentation layer`

## 디렉터리 구조 예시

레이어 책임과 의존 방향이 기준이다. 아래 구조는 구현을 시작할 때 참고하는 예시이며, 실제 package 이름과 파일 배치는 구현하면서 바꿀 수 있다.

```text
cmd/mwosa/

app/
cli/
command/<domain>/
service/<domain>/
providers/
  core/
  kis/
  datago/
packages/
  indicators/
canonical/
storage/
format/
config/
```

초기에는 Go의 `internal/` 디렉터리를 쓰지 않는다. 아직 package 경계가 자주 바뀔 수 있으므로 접근 제한보다 단순한 이동과 의존 관계 실험을 우선한다.

외부에서 import 하면 안 되는 구현 세부사항이 분명해지면 그때 해당 package 를 `internal/` 아래로 옮긴다. 반대로 provider role interface 처럼 provider client module 과 공유해야 하는 계약이 생기면 `internal/` 이 아니라 공개 package 또는 별도 module 로 분리한다.

필요가 분명해질 때만 아래 package 를 분리한다.

```text
handler/cli/
middleware/
domain/
presentation/
```

## 용어

이 문서에서는 아래 용어를 기준으로 통일한다.

| 용어                    | 의미                                                                                                                                                                                                                                  |
| ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| command layer           | Cobra command tree, verb-first 명령 표면, flag 선언을 담당하는 레이어다.                                                                                                                                                              |
| cli handler layer       | argv, flags, stdin, env 값을 use case request 로 정규화하는 레이어다.                                                                                                                                                                 |
| service layer           | use case 를 실행하고 provider registry, persistence, domain service 를 조합하는 application layer 다.                                                                                                                                 |
| domain layer            | 투자 리서치 도메인의 순수 규칙, 계산, value object 를 담는 레이어다. CLI, provider, storage 에 의존하지 않는다.                                                                                                                       |
| persistence layer       | 로컬 SQLite 정본 database 접근을 담당하는 저장소 레이어다.                                                                                                                                                                           |
| presentation layer      | service result 를 `table`, `json`, `ndjson`, `csv` 출력으로 변환하는 레이어다.                                                                                                                                                        |
| provider implementation | 실제 외부 API 호출, 인증, pagination, provider-native response parsing 을 담당하는 구현체다. provider architecture 문서에서는 provider client module 에 둔다.                                                                          |
| provider adapter        | provider implementation 을 service layer 가 사용하는 provider role interface 로 연결하는 adapter 다.                                                                                                                                  |
| provider router         | service layer 의 요청을 실제로 실행할 수 있는 provider role 로 라우팅하고 fallback 후보 순서를 결정하는 application component 다. 구체적인 interface 는 [Provider Architecture](../provider/README.md#provider-router) 에서 정의한다. |
| provider role interface | service layer 가 의존하는 provider 계약이다. 예: `dailybar.Fetcher`, `quote.Snapshotter`, `instrument.Searcher`                                                                                                                       |

이 용어 기준에서는 provider adapter 가 CLI 안 연결 지점이고, provider implementation 이 실제 외부 API client 다. service/domain layer 는 provider 세부사항을 알지 않고 앱의 use case 와 도메인 규칙을 수행한다.

## 요청 흐름

```text
argv/stdin/env
  -> command layer
  -> cli handler layer
  -> middleware layer
  -> service layer
  -> domain layer
  -> persistence layer
  -> service result
  -> presentation layer
  -> stdout/stderr/files
```

레이어 방향은 바깥에서 안쪽으로 흐른다. 안쪽 레이어는 바깥 레이어를 알지 않는다.

## 보조지표 계산 흐름

보조지표 계산은 service layer 가 use case 를 조립하고, `packages/indicators` 가 순수 계산만 맡는 구조로 둔다.

| 위치 | 역할 |
| --- | --- |
| provider | 외부 provider 에서 원천 데이터를 가져온다. |
| storage | 확보한 데이터를 보관하고 다시 읽는다. |
| service/calc | 필요한 데이터 확보와 계산 호출을 조율한다. |
| packages/indicators | 정렬된 입력 시계열을 받아 보조지표를 계산한다. |
| command/calc | CLI 입력을 service 요청으로 바꾼다. |
| presentation | 계산 결과를 `table`, `json`, `ndjson`, `csv` 로 보여준다. |

provider 는 보조지표 공식을 알지 않는다. `packages/indicators` 는 provider, storage, Cobra, 출력 형식을 알지 않는다.

## Command Layer

역할:

- Cobra command tree 를 정의한다.
- verb-first 명령 표면을 구성한다.
- argv, flag, stdin 사용 여부를 읽는다.
- 입력을 CLI handler 가 이해할 수 있는 request 로 넘긴다.

책임:

- command 이름과 alias 관리
- command help text 관리
- flag 선언
- 기본 argument shape 검증

하지 않는 일:

- provider 호출
- storage 접근
- domain 계산
- table/json 렌더링

예시 위치:

```text
command/<domain>/
```

## CLI Handler Layer

역할:

- command layer 에서 넘어온 CLI 입력을 use case request 로 변환한다.
- CLI 특유의 입력 방식을 application request 로 정규화한다.
- stdin, args, flags, env-derived option 을 하나의 request 로 합친다.

책임:

- flag value parsing
- stdin symbol list parsing
- output mode 선택값 정규화
- service input DTO 생성
- command-specific validation

하지 않는 일:

- business rule 판단
- provider fallback 판단
- canonical record 직접 생성
- stdout 직접 렌더링

예시 위치:

```text
handler/cli/
```

초기 구현에서는 `command/<domain>` 안에 handler 함수를 함께 둘 수 있다. 다만 handler 가 커지면 `handler/cli/<domain>` 으로 분리한다.

## Middleware Layer

역할:

- command 실행 전후의 공통 처리를 담당한다.
- 웹서버 middleware 처럼 cross-cutting concern 을 command/use case 사이에서 처리한다.

초기 middleware 후보:

- context cancellation
- timeout
- logging
- tracing
- error classification
- output mode propagation
- TTY detection
- color enable/disable
- interactive confirmation
- quiet/verbose mode
- stdin/stdout/stderr wiring

TTY 관련 책임:

- stdout 이 terminal 인지 pipe 인지 판별한다.
- table/color/progress 출력 가능 여부를 결정한다.
- machine-readable 출력에서는 progress 나 진단 정보를 stdout 에 섞지 않게 한다.
- interactive prompt 사용 가능 여부를 판단한다.

예시 위치:

```text
middleware/
```

## Service Layer

역할:

- use case 를 실행한다.
- provider registry, persistence, domain service 를 조합한다.
- read-through acquisition, ensure, inspect, compare 같은 application flow 를 담당한다.

책임:

- provider capability 선택 요청
- local coverage 확인
- 부족한 데이터 확보
- persistence 호출 순서 조정
- domain 계산 호출
- presentation 에 넘길 result model 생성

하지 않는 일:

- Cobra flag parsing
- terminal 출력
- provider-native response parsing
- 파일 경로 세부 계산

예시 위치:

```text
service/<domain>/
```

### Provider Routing

service layer 는 여러 provider 가 동시에 활성화된 상태를 전제로 한다. 따라서 service 는 특정 provider 구현체를 직접 호출하지 않고, 필요한 capability 를 provider router 에 요청한다. provider router 의 구체적인 interface 는 [Provider Architecture](../provider/README.md#provider-router) 의 `Router` 계약을 따른다.

provider router 의 역할:

- 요청에 필요한 capability 를 확인한다. 예: `quote`, `candles`, `instrument`, `filings`, `macro`
- provider registry 에 등록된 role profile 을 보고 호환 가능한 provider 후보를 찾는다.
- market, security type, freshness, auth 상태, priority, 사용자 지정 `--provider`, `--prefer-provider` 를 기준으로 후보 순서를 정한다.
- 선택된 provider role 을 service 에 반환하거나, 실행 helper 를 통해 fallback 순서대로 호출한다.
- 실패한 provider 와 fallback 이유를 service result 의 provenance 또는 explain 정보로 남길 수 있게 한다.

예:

```text
InspectService
  needs: instrument + quote + recent daily candles

provider router:
  instrument -> krx, kis, datago
  quote      -> kis, kiwoom
  candles    -> kis, datago, krx
```

fallback 규칙:

- 호환 provider 가 없으면 `ErrNoProvider` 를 반환한다.
- provider 가 rate limit, 일시 장애, 지원하지 않는 symbol 처럼 대체 가능한 오류를 반환하면 다음 후보를 시도할 수 있다.
- 인증 실패, 잘못된 사용자 입력, canonical validation 실패처럼 대체로 해결되지 않는 오류는 조용히 fallback 하지 않고 명확히 surface 한다.
- fallback 이 발생해도 성공 결과만 반환하지 않고, 어떤 provider 를 시도했고 왜 넘어갔는지 `--explain` 또는 verbose 결과에서 확인할 수 있어야 한다.

## Domain Layer

역할:

- 투자 리서치 도메인의 핵심 규칙과 계산을 담는다.
- CLI, provider, storage, terminal 에 의존하지 않는다.

domain 후보:

- instrument identity
- canonical key
- price series
- return calculation
- indicator result vocabulary
- RSI, MACD, SMA 같은 보조지표 결과 해석
- 스윙 흐름에 필요한 추세, 거래량, 변동성, 모멘텀 규칙
- position sizing
- reward/risk calculation
- portfolio weight calculation
- trade journal rule evaluation

책임:

- 순수 계산
- domain invariant 검증
- canonical vocabulary 정의
- value object 정의

하지 않는 일:

- 외부 API 호출
- SQLite storage 접근
- stdout 렌더링
- Cobra command 접근

예시 위치:

```text
domain/
canonical/
packages/indicators/
```

투자 리서치에 필요한 순수 보조지표 계산은 CLI domain 폴더 안에 넣기보다 `packages/indicators` 의 독립 계산 패키지로 둔다. MACD, 일목균형표, 이동평균은 그 검토 대상 중 일부다. domain/service 는 이 패키지를 호출하고, 계산 결과를 리서치 흐름에서 어떻게 사용할지 조합한다.

`canonical`, `packages/indicators`, `domain` 은 별도 책임으로 본다. 실제 package 배치는 책임 경계가 충분히 분명해졌을 때 조정한다.

지표와 추세 계산의 세부 기준은 [Indicator Architecture](../indicator/README.md) 에 둔다. provider 에서 가져온 원천 데이터는 canonical data 로 정규화하고, indicator 계산은 그 위에서 수행한다.

## Persistence Layer

역할:

- 로컬 SQLite 정본 database 접근을 담당한다.
- service 가 필요한 저장소 interface 를 구현한다.

구성:

- SQLite canonical database
- Ent schema types
- generated Ent client
- query/index helpers
- provenance columns
- latest quote view or table

책임:

- canonical record append/read/delete
- SQLite schema 와 index 를 Bun model 기준으로 관리
- service 가 의존하는 `ReadRepository` / `WriteRepository` 구현
- reindex 지원
- storage error 표준화

하지 않는 일:

- provider API 호출
- CLI flag 해석
- table/json 출력
- indicator 계산

예시 위치:

```text
storage/
storage/<resource>/
```

## Presentation Layer

역할:

- service result 를 terminal 또는 machine-readable output 으로 변환한다.
- 출력 형식별 책임을 분리한다.

초기 format:

- `table`
- `json`
- `ndjson`
- `csv`

책임:

- table rendering
- JSON encoding
- NDJSON streaming
- CSV encoding
- error output formatting
- explain output formatting

규칙:

- machine-readable 출력은 stdout 에 결과만 쓴다.
- progress, debug, verbose 정보는 stderr 로 보낸다.
- TTY 가 아니면 color/progress 출력은 기본 비활성화한다.
- presentation 은 service 를 호출하지 않는다.

예시 위치:

```text
presentation/
format/
```

초기에는 `format` 으로 시작하고, terminal 상호작용이 커지면 `presentation` 으로 확장한다.

## 레이어 의존 방향

```text
command
  -> cli handler
  -> middleware
  -> service
  -> domain

service
  -> persistence interface
  -> provider interface

persistence implementation
  -> SQLite database

presentation
  <- service result
```

금지 방향:

- `domain -> service`
- `domain -> persistence`
- `domain -> command`
- `service -> command`
- `persistence -> command`
- `persistence -> presentation`
- `provider adapter -> command`

## 웹서버 클린 아키텍처와의 대응

| Web server                     | mwosa CLI                        |
| ------------------------------ | -------------------------------- |
| route                          | command                          |
| controller / handler           | cli handler                      |
| HTTP middleware                | CLI middleware                   |
| request context                | command context                  |
| use case / application service | service                          |
| entity / value object          | domain                           |
| repository / gateway           | persistence / provider interface |
| database adapter               | storage                          |
| response presenter             | presentation                     |
| JSON HTTP response             | table / json / ndjson / csv      |

## 열어둘 결정

이 문서는 아직 뼈대만 둔다. 아래 항목은 이후 구현 대화에서 채운다.

- `handler/cli` 를 처음부터 분리할지 여부
- `presentation` 을 `format` 과 별도로 둘지 여부
- middleware chain 의 구체적인 함수 signature
- TTY abstraction interface
- error rendering 정책
- progress renderer 정책
- interactive confirmation 정책

## 관련 문서

- `docs/architectures/provider/README.md`
- `docs/architectures/tech-stack/README.md`
- `docs/architectures/packages/README.md`
- `docs/go-cli-package-layout.md`
