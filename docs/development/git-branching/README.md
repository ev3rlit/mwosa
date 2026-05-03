# Git Branching Strategy

## 목적

이 문서는 `mwosa` 의 브랜치 운영 기준을 정리한다.

`mwosa` 는 Go CLI 앱이고, 사용자는 최종적으로 특정 버전을 설치한다. 따라서 작업 브랜치와 배포 기준을 분리한다. 작업 브랜치는 목적별 접두사를 사용하고, 배포 기준은 릴리스 브랜치와 SemVer 태그로 고정한다.

## 기본 방향

- `main` 은 항상 다음 릴리스 후보가 모이는 기준선이다.
- `feat/*`, `fix/*` 는 원격에 push 할 수 있는 작업 브랜치다.
- 리모트에 만들 수 있는 브랜치는 `main`, `release/*`, `feat/*`, `fix/*` 로 제한한다.
- `codex/*`, `claude/*` 처럼 도구나 워크트리 출처를 드러내는 접두사는 리모트 브랜치 이름으로 쓰지 않는다.
- 사용자가 설치하는 기준은 브랜치가 아니라 `vX.Y.Z` 태그다.

## 브랜치 종류

| 브랜치 | 위치 | 용도 |
| --- | --- | --- |
| `main` | local, remote | 검증된 변경이 모이는 기본 브랜치 |
| `feat/<topic>` | local, remote | 작은 기능 구현 |
| `fix/<topic>` | local, remote | 버그 수정 또는 설정 수정 |
| `release/vX.Y` | local, remote | `vX.Y.Z` 패치 릴리스 안정화 |
| `vX.Y.Z` tag | remote | 사용자가 설치하는 고정 버전 |

`feat/*`, `fix/*` 는 원격에 push 할 수 있지만 배포 기준은 아니다. PR, CI, 리뷰, 작업 공유를 위한 브랜치로 사용하고, 검증된 변경만 `main` 또는 `release/*` 로 통합한다.

## 작업 흐름

일반 기능 작업은 작은 로컬 브랜치에서 시작한다.

```bash
git switch main
git pull --ff-only
git switch -c feat/provider-registry
git push -u origin feat/provider-registry
```

작업이 끝나면 로컬에서 검증하고 PR 또는 명시적인 merge 절차로 `main` 에 통합한다.

```bash
go test ./...
git switch main
git merge --ff-only feat/provider-registry
```

`main` 에 통합된 뒤에는 `main` 을 push 한다.

```bash
git push origin main
```

## 배포 흐름

CLI 배포는 `release/*` 브랜치와 SemVer 태그를 함께 사용한다.

```bash
git switch main
git pull --ff-only
git switch -c release/v0.1
git push -u origin release/v0.1
```

릴리스 브랜치에서는 배포 안정화에 필요한 최소 변경만 허용한다.

- 버전 문자열 갱신
- 릴리스 노트 정리
- 설치 경로 또는 completion 문서 수정
- 릴리스를 막는 버그 수정

배포 준비가 끝나면 태그를 만든다.

```bash
git tag v0.1.0
git push origin v0.1.0
```

사용자 설치 기준은 태그다.

```bash
go install github.com/<owner>/mwosa/cmd/mwosa@v0.1.0
```

`release/v0.1` 은 `v0.1.1`, `v0.1.2` 같은 패치 릴리스를 위해 유지한다. 다음 minor 버전은 `main` 에서 다시 `release/v0.2` 를 만든다.

## 금지 규칙

- `codex/*`, `claude/*`, `worktree/*` 같은 도구별 접두사를 `origin` 에 push 하지 않는다.
- 허용 목록 밖의 top-level 브랜치를 만들지 않는다.
- `dev`, `staging`, `prod` 같은 장기 환경 브랜치를 만들지 않는다.
- 배포 기준을 움직이는 브랜치 이름으로 안내하지 않는다.
- 검증하지 않은 실험 브랜치에서 태그를 만들지 않는다.
- 여러 주제의 변경을 한 브랜치와 한 커밋에 섞지 않는다.

환경 차이는 브랜치가 아니라 설정 파일, 빌드 플래그, 배포 스크립트, 문서로 관리한다.

## 브랜치 이름 규칙

브랜치 이름은 큰 단계보다 작은 결과물 중심으로 짓는다.

좋은 예:

- `feat/datago-client`
- `feat/sqlite-dailybar-store`
- `fix/provider-error-context`

피할 예:

- `codex/feat/datago-client`
- `claude/fix/provider-error-context`
- `phase1`
- `big-refactor`
- `all-docs`
- `release/latest`

`release/latest` 처럼 움직이는 배포 브랜치는 설치 기준으로 쓰지 않는다. 최신 안정판은 GitHub Release, 태그, 문서에서 안내한다.

## 병합 기준

가능하면 `--ff-only` 병합을 우선한다. 작은 로컬 브랜치를 `main` 에 순서대로 쌓는 흐름에서는 히스토리가 단순해진다.

`main` 이 먼저 움직여 fast-forward 가 불가능하면, 브랜치 그래프를 확인한 뒤 일반 merge commit 으로 통합한다. PR 을 사용하는 경우에는 ruleset 이 요구하는 CI 와 리뷰 조건을 통과한 뒤 merge 한다.

## GitHub ruleset

GitHub 에서는 브랜치 이름 allowlist 를 ruleset 으로 강제한다.

권장 ruleset:

- Ruleset name: `branch-name-policy`
- Enforcement status: 처음에는 `Evaluate`, 검증 후 `Active`
- Target branches: `Include by pattern` = `*`
- Rule: `Restrict branch names`
- Requirement: `Must match a given regex pattern`
- Regex:

```regex
^(main|release/v[0-9]+\.[0-9]+|(feat|fix)/[a-z0-9][a-z0-9._/-]*)\n?$
```

이 ruleset 은 `main`, `release/v0.1`, `feat/datago-client`, `fix/provider-error-context` 를 허용한다.

반대로 `codex/feat/datago-client`, `claude/fix/provider-error-context`, `worktree/test`, `phase1`, `dev`, `staging`, `prod` 는 허용하지 않는다.

`main` 과 `release/*` 는 별도 보호 ruleset 을 둔다. 이 ruleset 에서는 `Require a pull request before merging`, `Require status checks to pass`, `Block force pushes`, `Restrict deletions` 를 켠다.

## 정리 기준

`main` 또는 `release/*` 로 통합된 작업 브랜치는 로컬과 원격에서 삭제한다.

```bash
git branch -d feat/provider-registry
git push origin --delete feat/provider-registry
```

실험이 폐기된 브랜치는 변경 내용을 보관할 필요가 있을 때만 patch 나 문서로 남기고 삭제한다.

## 관련 결정

- `docs/adr/0001-cli-release-branching-strategy.md`
