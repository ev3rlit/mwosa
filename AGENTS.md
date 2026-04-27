# AGENTS.md

이 저장소는 instruction 문서를 레이어로 나누어 관리합니다.

## 반드시 읽을 문서

- 코드를 변경하기 전에 `RULE.md`를 읽습니다.
- 경계를 변경할 때는 관련 아키텍처 문서를 읽습니다:
  `docs/architectures/provider/README.md`,
  `docs/architectures/layers/README.md`,
  `docs/architectures/tech-stack/README.md`.
- provider 동작을 변경할 때는 provider 문서를 읽습니다:
  `docs/providers/README.md`와 provider별 README.

## 작업 규칙

- `RULE.md`를 기본 프로그래밍 규칙으로 따릅니다.
- 커밋은 좁고 주제별로 나눕니다.
- Go 코드를 수정한 뒤에는 `gofmt`를 실행합니다.
- 코드 변경 뒤에는 저장소 루트에서 `go test ./...`를 실행합니다.
- provider client 모듈을 변경했다면 해당 모듈 안에서도 `go test ./...`와
  `go mod verify`를 실행합니다.

## 생성 코드

- `storage/ent`는 직접 수정하지 않습니다.
- Ent schema는 `storage/schema` 아래에서 변경한 뒤 생성 코드를 다시 만듭니다.
