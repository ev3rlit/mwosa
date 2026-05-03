# ADR 0001: CLI release branching strategy

## 상태

Accepted

## 날짜

2026-05-03

## 맥락

`mwosa` 는 사용자가 설치해서 쓰는 Go CLI 앱이다. 따라서 개발 중인 브랜치와 설치 가능한 배포 기준을 분리해야 한다.

혼자 개발하는 환경에서는 Git Flow 같은 복잡한 장기 브랜치 모델이 부담스럽다. 대신 작은 작업 브랜치를 바텀업으로 쌓고, 검증된 변경만 `main` 과 `release/*` 로 통합하는 방식이 적합하다.

다만 작업 브랜치를 원격에 올릴 수 있어야 PR, CI, 리뷰, 작업 공유를 사용할 수 있다. 따라서 `feat/*`, `fix/*` 는 원격 브랜치로 허용한다.

반면 `codex/*`, `claude/*` 처럼 도구나 워크트리 출처를 드러내는 브랜치 이름은 리모트 정책으로 금지한다. 원격 브랜치 이름은 작업의 출처가 아니라 목적을 나타내야 한다.

## 결정

`mwosa` 는 다음 브랜치 전략을 사용한다.

- `main` 은 다음 릴리스 후보가 모이는 기본 브랜치로 둔다.
- `feat/*`, `fix/*` 는 원격에 push 할 수 있는 작업 브랜치로 둔다.
- 리모트 브랜치는 `main`, `release/*`, `feat/*`, `fix/*` 로 제한한다.
- `codex/*`, `claude/*`, `worktree/*` 같은 도구별 접두사는 리모트 브랜치 이름으로 금지한다.
- GitHub ruleset 으로 브랜치 이름 allowlist 를 강제한다.
- CLI 설치 기준은 `release/*` 브랜치가 아니라 `vX.Y.Z` SemVer 태그로 둔다.
- minor 릴리스 안정화가 필요할 때 `release/vX.Y` 브랜치를 만든다.
- 패치 릴리스는 같은 `release/vX.Y` 브랜치에서 준비하고 새 `vX.Y.Z` 태그를 만든다.

상세 운영 규칙은 `docs/development/git-branching/README.md` 를 따른다.

## 결과

이 결정으로 작업 흐름은 단순하게 유지된다. 작은 `feat/*`, `fix/*` 브랜치에서 변경을 나눠 진행하고, 검증된 결과만 `main` 으로 통합한다.

배포 기준은 명확해진다. 사용자는 움직이는 브랜치가 아니라 고정된 태그로 CLI 를 설치한다.

`release/*` 브랜치는 패치 릴리스 관리에는 유용하지만, 모든 개발이 거치는 장기 통합 브랜치가 아니다. 배포 안정화와 패치 수정에만 사용한다.

브랜치 이름 규칙은 GitHub ruleset 으로 강제된다. 따라서 에이전트나 로컬 워크트리가 `codex/*`, `claude/*` 같은 이름을 만들어도 리모트 push 단계에서 차단된다.

## 대안

### Git Flow

`develop`, `release`, `hotfix`, `main` 을 모두 운영하는 방식이다. 여러 명이 동시에 릴리스를 관리할 때는 도움이 되지만, 현재의 개인 CLI 개발 흐름에는 과하다.

### main only

모든 작업을 `main` 에 직접 커밋하고 태그만 찍는 방식이다. 가장 단순하지만, 실험과 배포 안정화가 섞이기 쉽다.

### stable branch

`stable` 브랜치를 최신 안정판으로 유지하는 방식이다. 사용자가 항상 최신 안정판만 설치해야 하는 배포 채널이 필요할 때 검토할 수 있다. 현재는 SemVer 태그와 GitHub Release 만으로 충분하다.

### local-only working branches

`feat/*`, `fix/*` 를 로컬 전용으로만 두는 방식이다. 원격 브랜치가 적어지는 장점은 있지만 PR, CI, ruleset 기반 merge gate 를 쓰기 어렵다. 현재는 주요 작업 브랜치를 원격에 허용하고, 이름 allowlist 로 불필요한 접두사를 막는 쪽을 선택한다.

## 관련 문서

- `docs/development/git-branching/README.md`
