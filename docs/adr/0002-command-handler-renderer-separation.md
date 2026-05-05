# ADR 0002: Command, handler, renderer separation

## 상태

Accepted

## 날짜

2026-05-05

## 맥락

`mwosa` CLI command 는 Cobra command tree, flag parsing, app runtime 생성, service 호출, output rendering 을 한 함수 안에서 함께 처리하고 있었다.

초기에는 이 방식이 단순했지만 command 가 늘어나면서 같은 패턴이 반복되기 시작했다. 예를 들어 command 는 service result 를 받은 뒤 `table`, `json`, `ndjson`, `csv` output mode 별로 writer 함수를 직접 선택했다. 이 구조에서는 새 command 나 result type 을 추가할 때마다 command layer 에 business flow 와 presentation branch 가 함께 늘어난다.

또한 CLI 가 직접 service 를 호출하면 동일한 use case 를 CLI 밖에서 재사용하기 어렵다. 나중에 TUI, local automation, HTTP-like adapter 같은 다른 entrypoint 가 필요해질 때 Cobra command 에 묶인 실행 흐름을 다시 분리해야 한다.

따라서 command parsing, application handler, terminal renderer 를 분리한다.

## 결정

`mwosa` 는 command layer, app handler layer, renderer layer 를 분리한다.

- CLI command 는 Cobra command tree, args/flags parsing, config loading, app runtime 생성만 담당한다.
- CLI command 는 생성된 `app.Runtime` 의 `Handlers` 필드에 있는 명시적 handler method 를 호출한다.
- Handler 는 `app.Runtime` 을 소유하지 않는다. 반대로 `app.Runtime` 이 handler 들을 조립해서 필드로 가진다.
- Handler 는 Cobra, writer, output mode 를 알지 않는다.
- Handler 는 필요한 service 만 의존하고, request struct 를 service request 로 변환한 뒤 result 를 반환한다.
- Handler method 는 `context.Context` 와 handler-local request struct 를 받고, 결과 구조체와 error 를 반환한다.
- CLI command 는 handler 결과를 받은 뒤 공통 renderer 에 넘긴다.
- Renderer 는 `OutputMode` 에 따라 `json`, `ndjson`, `csv`, `table` 을 처리한다.
- Result type 은 필요할 때 output projection method 를 구현한다.
  - `JSONValue() any`
  - `NDJSONRows() any`
  - `CSVRows() any`
  - `TableRows() (header []string, rows [][]string)`
- Renderer 는 위 method 를 덕타이핑으로 감지한다. 따라서 result type 이 renderer package 를 import 할 필요는 없다.
- CSV field formatting 은 `csvutil.Marshaler` 를 계속 사용한다.
- Table rendering 은 `tablewriter` 를 계속 사용한다.

`app.Runtime` 의 handler 필드는 `HandlerRuntime` 같은 접미사를 쓰지 않고 `Handlers` 로 둔다.

예상 구조는 다음과 같다.

```text
cli command
  -> app.Runtime 생성
  -> runtime.Handlers.<domain>.<usecase>(ctx, request)
  -> cli renderer
  -> stdout
```

```text
app.Runtime
  -> Storage
  -> Providers
  -> Services
  -> Handlers
```

초기 handler 연결은 명시적 method 호출로 둔다. HTTP router 처럼 문자열 route 나 URL route 로 dispatch 하는 자동 라우팅은 만들지 않는다.

## 결과

Command layer 는 얇아진다. command 는 flag 와 arg 를 읽어 handler request 를 만들고, handler 결과를 renderer 에 넘기는 adapter 로 남는다.

Handler layer 는 CLI 밖에서도 호출 가능한 application-facing API 가 된다. 이 레이어는 `app/handler` 아래에 두고, service 를 조합하거나 service request 로 변환하는 책임을 가진다.

Renderer layer 는 command 별 output switch 를 중앙화한다. JSON 과 NDJSON 은 기본적으로 Go value 를 직렬화하고, CSV 와 table 은 라이브러리 기본 동작을 우선 사용한다. 사람이 보기 좋은 projection 이 필요한 result 는 output projection method 를 구현한다.

Service layer 는 여전히 Cobra, stdout, output format 을 모른다. Provider, storage, domain 경계도 유지된다.

이 결정은 CLI output 의 stdout 계약과도 연결된다. command result 는 stdout 으로 쓰고, diagnostics, progress, log 는 stderr 로 남긴다.

## 대안

### Command 에서 service 와 renderer 를 직접 호출

가장 단순한 방식이다. 하지만 command 가 늘어날수록 `RunE` 안에 service call 과 output switch 가 반복된다. CLI 밖 entrypoint 에서 use case 를 재사용하기도 어렵다.

### Handler 가 app runtime 을 소유

Handler constructor 가 `*app.Runtime` 을 받아 내부에서 필요한 service 를 꺼내 쓰는 방식이다. 호출부는 단순하지만 handler 가 runtime 전체를 알게 되고, `app/handler` 가 `app` 을 import 하면서 순환 의존 위험이 생긴다. 이 방식은 선택하지 않는다.

### Router 기반 자동 dispatch

`strategy.create` 나 `/strategy/create` 같은 route key 로 handler 를 자동 dispatch 하는 방식이다. handler 수가 많아지면 검토할 수 있지만, 현재는 request type, validation, route naming, error mapping 정책이 먼저 커진다. 초기에는 명시적 method 연결을 선택한다.

### Result type 별 writer 함수 유지

`writeScreenRunHistory(w, output, runs)` 같은 함수에 output switch 를 계속 두는 방식이다. 기존 테스트를 유지하기 쉽지만 renderer 정책이 command/result type 별로 흩어진다. 공통 renderer 와 projection method 로 대체한다.

## 관련 문서

- `docs/architectures/layers/README.md`
