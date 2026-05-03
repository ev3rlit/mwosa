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

## Git branch strategy

`mwosa` 의 브랜치 운영 기준은 `docs/development/git-branching/README.md` 를 따릅니다. 이 결정의 ADR 은 `docs/adr/0001-cli-release-branching-strategy.md` 에 기록되어 있습니다.

핵심 규칙:

- `codex/*`, `claude/*`, `worktree/*` 같은 도구별 접두사는 리모트에 push 하지 않습니다.
- 리모트에 push 할 수 있는 브랜치는 `main`, `release/*`, `feat/*`, `fix/*` 입니다.
- 사용자가 설치하는 CLI 기준은 `vX.Y.Z` SemVer 태그입니다.
- 배포 안정화는 `release/vX.Y` 브랜치에서 진행합니다.
- 일반 기능과 수정 작업은 작은 `feat/*`, `fix/*` 브랜치에서 시작합니다.
- GitHub ruleset 으로 허용된 브랜치 이름만 원격에 생성합니다.

작업 전에 현재 브랜치와 원격 추적 상태를 확인합니다.

```bash
git status --short --branch
```

작업 브랜치에서 작업한 변경은 검증 후 PR 또는 명시적인 merge 절차로 `main` 또는 필요한 `release/*` 브랜치에 통합합니다.
