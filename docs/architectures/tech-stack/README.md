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
- provider adapter 와 service interface 를 명확하게 나누기 좋다.

### CLI framework

- `spf13/cobra`

선정 이유:

- subcommand 기반 CLI 구조에 적합하다.
- help, completion, persistent flag, local flag 를 기본 지원한다.
- `mwosa inspect portfolio`, `mwosa get quote`, `mwosa calc rsi` 같은 다층 command tree 를 표현하기 좋다.

적용 범위:

- `cmd/mwosa`
- `cli`
- `command/*`

### Error handling

- `github.com/samber/oops`

선정 이유:

- operation, domain, key/value context, cause 를 함께 보존하기 좋다.
- provider, storage, service, CLI 경계에서 실패 맥락을 명시적으로 붙이기 좋다.
- fallback 판단, 사용자 메시지, 로그 진단에 필요한 정보를 error chain 에 남기기 좋다.

결정:

- 하위 레이어 error 는 호출 경계에서 `oops.In(...).With(...).Wrap(err)` 형태로 명시적으로 감싼다.
- error code, fallback 판단, 사용자 메시지 분리는 각 아키텍처 결정 문서에서 해당 경계에 맞게 관리한다.
- invalid input, partial data, provider failure 를 성공처럼 숨기지 않는다.

적용 범위:

- `cli`
- `service`
- `providers/*`
- `storage`

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
- 이 저장소에는 `providers/core` 와 provider 별 adapter 만 둔다.

예:

- external package: `github.com/<org>/marketdata-provider-kis`
- external package: `github.com/<org>/marketdata-provider-datago`
- in-repo adapter: `providers/kis`
- in-repo adapter: `providers/datago`

provider 이름은 큰 데이터 소스 단위로 둔다. 공공데이터포털처럼 개별 API 서비스가 따로 승인되는 경우에도 provider id 는 `datago` 로 유지하고, 세부 API 범위는 provider group 으로 표현한다.

### HTTP client

- `Not fixed yet`

결정:

- provider 가 REST API 를 사용할 때의 HTTP client 는 실제 provider 구현 단계에서 정한다.
- 기본 후보는 Go 표준 라이브러리 `net/http` 로 둔다.
- retry/backoff, request builder, tracing 같은 필요가 반복될 때만 wrapper 나 별도 client 를 검토한다.
- provider role interface 와 service layer 에는 특정 HTTP client library type 을 노출하지 않는다.

세부 협업 기준은 `docs/development/README.md` 에 둔다.

### Configuration

- `Go standard library + explicit config package`

결정:

- 초기에는 별도 설정 framework 를 도입하지 않는다.
- 환경변수, 설정 파일, 기본 경로 처리는 `config` 에서 직접 다룬다.

## 아직 정하지 않음

다음 항목은 구현 과정에서 필요가 분명해질 때 결정한다.

- logging library
- test assertion library
- table rendering library
- config file format
- migration/versioning tool
- NDJSON 이후 추가 저장 포맷
- provider package repository strategy

## 관련 문서

- `docs/architectures/layers/README.md`
- `docs/architectures/provider/README.md`
- `docs/architectures/indicator/README.md`
- `docs/architectures/completion/README.md`
- `docs/development/README.md`
- `docs/canonical-schema.md`
