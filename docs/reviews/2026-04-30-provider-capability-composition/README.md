# Provider Capability 등록 원칙

- 작성일: 2026-04-30
- 대상 브랜치: `codex/mwosa-cli-provider-core`
- 범위: `providers/core`, `providers/spec`, `providers/datago`, `service/daily`

## 목표

provider 가 무엇을 할 수 있는지 사람이 따로 설명하지 않아도, 실제 provider bridge 구조에서 자연스럽게 드러나야 한다.

핵심 원칙은 단순하다.

```text
provider bridge 에 임베딩된 role field 가 source of truth 다.
registry 는 그 embedded field 를 읽어 가능한 role/command 를 등록한다.
지원하지 않는 것은 직접 적지 않고, 임베딩된 role 과 전체 목록의 차이로 계산한다.
```

즉, "이 provider 는 quote 를 못 한다"라고 쓰지 않는다. `quote.Snapshotter` 가 임베딩되어 있지 않으면 registry/discovery 가 quote 미지원으로 해석하면 된다.

## 원하는 모델

provider client 는 provider-native API 를 가진다. provider bridge 는 그중 `mwosa` 에 노출할 role 만 anonymous field 로 임베딩한다.

```go
type Provider struct {
    provider.Identity

    dailybar.Fetcher
    instrument.Searcher
    // quote.Snapshotter 를 임베딩하지 않으면 quote 역할은 제공하지 않는다.
}
```

이 구조에서는 embedded role field 가 곧 capability 선언이다.

- `dailybar.Fetcher` 를 임베딩하면 daily-bar command group 을 제공한다.
- `instrument.Searcher` 를 임베딩하면 instrument command group 을 제공한다.
- `quote.Snapshotter` 를 임베딩하지 않으면 quote command group 은 제공하지 않는다.

Redis 의 command group 과 비슷하게 보면 된다.

```text
Redis:
  string commands
  hash commands
  list commands
  sorted set commands

mwosa provider:
  daily-bar commands
  quote commands
  instrument commands
  macro commands
```

각 command group 은 자기 profile 과 실행 메서드를 가진다. registry 는 provider struct 의 exported anonymous field 를 reflection 으로 훑고, `RoleProvider` 를 구현한 embedded field 만 등록한다.

## Registry 책임

registry 는 복잡한 별도 선언 파일을 읽지 않는다.

1. provider bridge struct 의 exported anonymous field 를 확인한다.
2. embedded field 가 `RoleProvider` 계열이면 `RoleRegistration()` 을 읽는다.
3. `RoleRegistration` 의 profile 과 impl 을 registry entry 로 저장한다.
4. 임베딩되지 않은 role 은 등록하지 않는다.

현재 코드의 reflection 기반 수집 방향은 유지하되, role field 는 named field 가 아니라 embedded field 를 기준으로 한다.

```go
type RoleProvider interface {
    RoleRegistration() RoleRegistration
}

type RoleRegistration struct {
    Profile RoleProfile
    Impl    any
}
```

세부 command 가 필요해지면, 먼저 별도 거대한 capability system 을 만들지 말고 embedded role field 가 이미 가진 profile/method 에서 읽을 수 있는지 확인한다. 필요할 때만 `RoleRegistration` 에 작은 metadata 를 추가한다.

## Unsupported 계산

unsupported 는 provider 가 직접 선언하는 값이 아니다.

```text
mwosa 가 아는 role/command 목록
- provider bridge 에서 발견한 embedded role/command
= discovery 에 표시할 미지원 항목
```

예를 들어 `datago` bridge 에 `dailybar.Fetcher`, `instrument.Searcher` 만 임베딩되어 있으면:

```text
supported:
  daily_bar
  instrument

derived unsupported:
  quote_snapshot
```

이 정보는 사람이 손으로 적은 문장이 아니라 구조에서 나온다. 그래서 provider 구현이 바뀌면 discovery 결과도 같이 바뀐다.

## Router 실패 진단

router 는 후보가 없을 때 `ErrNoProvider` 만 던지지 말고, 어떤 축에서 후보가 빠졌는지 최소한으로 남긴다.

처음에는 아래 정도면 충분하다.

```text
provider
role
market
security_type
group
operation
reason
```

예:

```text
datago 는 daily_bar role 은 있지만 security_type=stock 은 지원하지 않는다.
datago 는 quote.Snapshotter embedded field 가 없다.
```

이 정도만 있어도 AI 나 CLI 사용자가 "provider 가 없는지", "role 은 있지만 scope 가 안 맞는지", "인증/config 문제인지"를 구분할 수 있다.

## service layer 기준

service 는 provider 구현 타입을 모른다.

- service 는 필요한 role 을 router 에 요청한다.
- router 는 registry 에 등록된 embedded role field 중 맞는 후보를 고른다.
- provider 별 특수 분기는 adapter/role 구현 안에 둔다.
- 없는 기능은 service 가 추측하지 않고 router/discovery 진단으로 드러낸다.

`Backfill` 처럼 호출 모양이 여러 가지일 수 있는 경우에도, 처음부터 큰 capability 체계를 만들 필요는 없다. 우선 현재 role/profile 로 충분한지 보고, 부족해지는 순간에 command group 내부를 더 잘게 나눈다.

## 지금 하지 않을 것

- 지원하지 않는 capability 를 provider 코드에 직접 나열하지 않는다.
- 실제 구현보다 큰 capability matrix/status enum 을 먼저 만들지 않는다.
- router 메서드를 `RouteDateBatch`, `RouteDateRange` 처럼 미리 여러 개로 나누지 않는다.
- `Limitations` 를 unsupported 설명 용도로 쓰지 않는다.
- 문서가 구현보다 복잡해지지 않게 한다.

## 1차 변경 방향

1. 현재 `RegisterProvider` 의 reflection 기반 수집은 유지하되 embedded role field 만 등록한다.
2. discovery 또는 plan 출력에서 "등록된 embedded role" 과 "없는 role" 을 보여준다.
3. `Router.Plan` 은 후보 탈락 이유를 작게 남긴다.
4. 필요해질 때만 role 내부 command 를 더 세분화한다.

목표는 화려한 capability framework 가 아니다. provider bridge 를 자연스럽게 조립하면, 그 조립 결과가 곧 `mwosa` 가 이해하는 provider capability 가 되는 구조다.
