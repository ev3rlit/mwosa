# Directory Architecture

## 목적

이 문서는 `mwosa` Go CLI 의 추천 디렉터리 구조를 정의한다.

`mwosa` 는 사용자에게는 verb-first CLI 로 보인다.

```text
mwosa <verb> [resource] [target] [flags]
```

하지만 구현은 수평 협업을 위해 도메인 단위로 나눈다. 즉, public CLI 는 `inspect`, `get`, `record`, `calc` 같은 동사를 먼저 보여주되, 코드 소유권은 `instrument`, `provider`, `portfolio`, `trade`, `strategy`, `data`, `calc` 같은 도메인 package 에 둔다.

## 설계 원칙

- `cmd/` 는 바이너리 진입점만 가진다.
- `internal/app` 은 애플리케이션 조립만 담당한다.
- `internal/cli` 는 root command, 공통 flag, registry, help/completion 을 담당한다.
- `internal/command/<domain>` 은 도메인별 Cobra command 등록과 argument validation 을 담당한다.
- `internal/service/<domain>` 은 실제 use case 를 실행한다.
- provider, storage, index, format 은 command package 에서 직접 다루지 않는다.
- provider 구현체는 외부 package 로 두고, 이 저장소에는 bridge adapter 만 둔다.

## 추천 디렉터리 구조

```text
cmd/
  mwosa/
    main.go

internal/
  app/
    run.go

  cli/
    root.go
    registry.go
    builder.go
    flags.go
    output.go
    errors.go

  command/
    instrument/
      routes.go
      inspect.go
      search.go
      get_quote.go
      get_candles.go

    provider/
      routes.go
      list.go
      inspect.go
      test.go

    portfolio/
      routes.go
      inspect.go
      create.go
      update.go
      rebalance.go

    trade/
      routes.go
      record.go
      close.go
      review.go

    strategy/
      routes.go
      create.go
      validate.go
      backtest.go

    data/
      routes.go
      ensure.go
      delete.go
      reindex.go
      export.go
      import.go

    calc/
      routes.go
      returns.go
      rsi.go
      position_size.go
      rr.go

  service/
    instrument/
    provider/
    portfolio/
    trade/
    strategy/
    data/
    calc/

  provider/
    registry.go
    capability.go
    bridge.go

  providers/
    kisbridge/
      bridge.go
      config.go
    datagobridge/
      bridge.go
      config.go

  canonical/
    model.go
    keys.go
    normalize.go

  storage/
    files/
      layout.go
      writer.go
      reader.go
      delete.go
      reindex.go

    index/
      surreal.go
      coverage.go
      manifest.go
      latestquote.go
      provenance.go

  indicator/
    sma.go
    ema.go
    rsi.go
    macd.go

  format/
    json.go
    ndjson.go
    csv.go
    table.go

  config/
    config.go
    env.go
    path.go

  util/
    dates.go
    numbers.go
    errors.go
```

## 패키지별 책임

### `cmd/mwosa`

- 실제 바이너리 진입점이다.
- `internal/app` 실행만 위임한다.
- config, provider, storage, command 세부 구현을 직접 알지 않는다.

### `internal/app`

- 애플리케이션 조립 지점이다.
- config 를 로딩한다.
- provider registry 를 만든다.
- storage/index 구현체를 연결한다.
- CLI root command 를 실행한다.

### `internal/cli`

- Cobra root command 를 만든다.
- 공통 flag 를 정의한다.
- command registry 를 관리한다.
- help, completion, 공통 output mode 를 관리한다.
- 개별 도메인 명령의 세부 동작은 소유하지 않는다.

### `internal/command`

- 도메인별 command package 를 둔다.
- 각 도메인은 자기 resource 와 관련된 command tree 를 등록한다.
- argument validation 과 typed request 생성을 담당한다.
- 실제 동작은 service 로 위임한다.

권장 등록 형태:

```go
func Register(registry *cli.Registry, deps Dependencies)
```

### `internal/service`

- 실제 use case 계층이다.
- provider registry, canonical storage, index, indicator 를 조합한다.
- command 에서 넘어온 typed request 를 실행하고 result 를 반환한다.
- 도메인 단위 package 로 나누어 병렬 작업하기 쉽게 유지한다.

### `internal/provider`

- CLI core 가 의존하는 provider 추상화를 정의한다.
- provider capability, priority, fallback 정책을 관리한다.
- 외부 provider package contract 자체가 아니라 CLI 내부 bridge contract 를 둔다.

### `internal/providers/*bridge`

- 외부 provider package 를 CLI 내부 contract 에 연결한다.
- config 변환, external result 변환, registry metadata 제공을 담당한다.
- canonical storage 를 직접 쓰지 않는다.
- CLI flag parsing 을 하지 않는다.

### `internal/canonical`

- provider-neutral record model 을 정의한다.
- canonical key 생성과 normalize helper 를 둔다.
- `docs/canonical-schema.md` 의 코드 표현이다.

### `internal/storage`

- `files` 는 canonical body 의 source of truth 를 관리한다.
- `index` 는 SurrealDB metadata/index 접근을 담당한다.
- file storage 와 index storage 를 하나의 정본처럼 섞지 않는다.

### `internal/format`

- table, json, ndjson, csv 출력을 담당한다.
- command 나 service 가 출력 포맷 문자열을 직접 만들지 않게 한다.

## Verb-first 와 도메인 소유권

사용자는 verb-first 로 명령을 실행한다.

```text
mwosa inspect portfolio core
mwosa list providers
mwosa get quote 005930
mwosa calc rsi 491820
mwosa record trade
```

구현은 도메인 package 가 소유한다.

예를 들어 `portfolio` package 는 아래 명령을 함께 관리한다.

```text
mwosa inspect portfolio <name>
mwosa list portfolios
mwosa create portfolio <name>
mwosa update portfolio <name>
mwosa compare portfolios <names...>
mwosa rebalance portfolio <name>
```

이렇게 나누면 public CLI 의 일관성을 유지하면서도, 작업자는 도메인별 디렉터리 안에서 독립적으로 수정할 수 있다.

## 의존 방향

권장 의존 방향:

```text
cmd -> app -> cli
cli -> command
command -> service
app -> config
app -> provider registry
app -> storage
service -> provider bridge
service -> canonical
service -> storage
service -> indicator
provider bridge -> external provider package
storage/index -> SurrealDB
storage/files -> local filesystem
format -> result DTO
```

피해야 할 의존:

- `cli -> external provider package`
- `command -> external provider package`
- `command -> storage/index`
- `command -> format internals`
- `indicator -> storage`
- `provider bridge -> cli`
- `storage -> cli`

## 파일 배치 규칙

- `routes.go` 는 command tree 등록만 담당한다.
- 개별 command 파일은 argument validation 과 request 생성까지만 담당한다.
- service package 에는 CLI flag 이름을 넘기지 않는다.
- service input 은 CLI 에 독립적인 typed request 로 정의한다.
- 출력 포맷은 `internal/format` 에서만 다룬다.
- 여러 도메인에서 쓰는 비즈니스 의미가 생기면 `util` 이 아니라 더 구체적인 package 로 옮긴다.

## 초기 구현 순서

1. `cmd/mwosa`
2. `internal/app`
3. `internal/cli`
4. `internal/command`
5. `internal/config`
6. `internal/provider`
7. `internal/canonical`
8. `internal/storage/files`
9. `internal/storage/index`
10. `internal/service`
11. `internal/providers/*bridge`
12. `internal/indicator`
13. `internal/format`

## 관련 문서

- `README.md`
- `docs/go-cli-package-layout.md`
- `docs/architectures/layers/README.md`
- `docs/architectures/interfaces/README.md`
- `docs/architectures/provider/README.md`
- `docs/architectures/tech-stack/README.md`
- `docs/architecture.md`
- `docs/canonical-schema.md`
- `docs/providers/provider-package-contract.md`
