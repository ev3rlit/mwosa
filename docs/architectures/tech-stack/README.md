# Technology Stack

## 목적

이 문서는 `mwosa` Go CLI 의 기술 스택 결정을 정리한다.

지금은 구현을 시작하기 위한 기본 선택만 적는다. 세부 라이브러리와 운영 도구는 실제 구현 대화를 진행하면서 이 문서에 추가한다.

## 현재 확정

### Language

- `Go`

선정 이유:

- 단일 바이너리 CLI 배포가 쉽다.
- 파일 I/O, HTTP client, context cancellation, 병렬 처리에 적합하다.
- provider bridge 와 service interface 를 명확하게 나누기 좋다.

### CLI framework

- `spf13/cobra`

선정 이유:

- subcommand 기반 CLI 구조에 적합하다.
- help, completion, persistent flag, local flag 를 기본 지원한다.
- `mwosa inspect portfolio`, `mwosa get quote`, `mwosa calc rsi` 같은 다층 command tree 를 표현하기 좋다.

적용 범위:

- `cmd/mwosa`
- `internal/cli`
- `internal/command/*`

### Canonical source of truth

- `Local files`

결정:

- provider-neutral canonical record 를 로컬 파일에 저장한다.
- 초기 저장 포맷은 `NDJSON` 를 기준으로 한다.
- SurrealDB index 는 파일 정본을 대체하지 않는다.

### Metadata / index

- `SurrealDB`

결정:

- coverage, file manifest, provenance, latest quote, provider metadata 를 저장한다.
- index 는 손상되어도 로컬 파일 기준으로 재구축할 수 있어야 한다.

### Provider implementation

- `External Go packages`

결정:

- provider 실제 구현체는 CLI 저장소 밖의 Go package 로 분리한다.
- 이 저장소에는 provider bridge adapter 와 registry 연결만 둔다.

예:

- external package: `github.com/<org>/marketdata-provider-kis`
- external package: `github.com/<org>/marketdata-provider-data-go-etf`
- in-repo bridge: `internal/providers/kisbridge`
- in-repo bridge: `internal/providers/datagobridge`

### Configuration

- `Go standard library + explicit config package`

결정:

- 초기에는 별도 설정 framework 를 도입하지 않는다.
- 환경변수, 설정 파일, 기본 경로 처리는 `internal/config` 에서 직접 다룬다.

## 아직 정하지 않음

다음 항목은 구현 과정에서 필요가 분명해질 때 결정한다.

- logging library
- test assertion library
- HTTP client wrapper
- table rendering library
- config file format
- migration/versioning tool
- NDJSON 이후 추가 저장 포맷
- provider package repository strategy

## 관련 문서

- `docs/architectures/directory/README.md`
- `docs/architectures/layers/README.md`
- `docs/architectures/interfaces/README.md`
- `docs/architectures/provider/README.md`
- `docs/architectures/completion/README.md`
- `docs/canonical-schema.md`
- `docs/providers/provider-package-contract.md`
