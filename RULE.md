# RULE.md

이 문서는 mwosa의 짧은 프로그래밍 규칙입니다. 아키텍처 문서보다
짧게 유지합니다. 설명이 길어지는 내용은 이 문서에 모두 풀어쓰기보다
관련 문서로 연결합니다.

## 핵심 원칙

- 레이어 경계를 지킵니다. CLI는 의존성을 조립하고, service는 role
  interface와 repository interface만 사용하며, provider 구현 세부사항은
  service로 새지 않게 합니다.
- provider client는 독립적으로 관리합니다. `clients` 아래의
  provider client는 자체 `go.mod`와 테스트를 가진 별도 Go 모듈입니다.
- 변경 범위는 좁게 유지합니다. 현재 경계 안에서 해결할 수 있는 요청에
  불필요한 대형 리팩터링을 붙이지 않습니다.
- 실패를 빈 성공처럼 숨기지 않습니다. unsupported capability, route
  없음, remote error, storage error, invalid input은 명시적인 error로
  반환하고, 가능한 경우 provider, group, operation, market,
  security type, symbol, date 맥락을 포함합니다.

## 에러 처리

- 직접 작성하는 Go 코드는 error 생성, wrapping, joining에
  `github.com/samber/oops`를 사용합니다.
- 새 error를 만들 때 `fmt.Errorf`, `errors.New`, `errors.Join`을
  사용하지 않습니다. error 판별을 위한 `errors.Is`, `errors.As`는
  사용할 수 있습니다.
- `storage/ent` 같은 생성 코드는 이 규칙에서 제외합니다. 생성 코드는
  다시 생성될 수 있으므로 직접 수정하지 않습니다.
- 새 error는 `oops.New` 또는 `oops.Errorf`를 사용합니다. 하위 레이어
  error를 원인으로 보존해야 할 때는 `Wrap` 또는 `Wrapf`를 사용합니다.
  cleanup 과정에서 여러 error를 보존해야 할 때는 `oops.Join`을
  사용합니다.
- 같은 함수 안에서 domain과 context가 반복되면 재사용 가능한 builder를
  먼저 만듭니다. builder는 `.New`, `.Errorf`, `.Wrap`, `.Wrapf`,
  `.Join`, `.Recover`, `.Recoverf` 같은 종료 메서드로 끝냅니다.

```go
errb := oops.
	In("dailybar_repository").
	With(
		"market", query.Market,
		"security_type", query.SecurityType,
		"symbol", query.Symbol,
		"from", query.From,
		"to", query.To,
	)

client, err := r.database.Client(ctx)
if err != nil {
	return nil, errb.Wrap(err)
}
```

- context는 CLI 경계에서 한 번에 몰아서 붙이지 않습니다. 호출자는 자신이
  알고 있는 요청 맥락을 붙이고, 호출받는 쪽은 자신이 알고 있는 domain
  맥락을 붙입니다.
- `With(...)`는 구조화된 context이며, 항상 사람이 읽는 메시지를 대체하지는
  않습니다. 테스트나 CLI 사용자가 `provider=datago`,
  `group=securitiesProductPrice`, `operation=getETFPriceInfo`,
  `status=502` 같은 필드를 error 문자열에서 직접 확인해야 한다면 해당
  필드를 메시지에도 남깁니다.

## Storage

- 로컬 canonical storage 방향은 SQLite입니다.
- Ent schema는 `storage/schema` 아래에 둡니다. 생성된 Ent 코드는
  `storage/ent` 아래에 있으며 직접 수정하지 않습니다.
- database runtime 접근은 lazy하게 처리합니다. storage handle 생성만으로
  SQLite를 열지 않고, 실제 첫 사용 시점에 열 수 있습니다. cleanup은
  command 단위에서 닫습니다.
- repository 구현체는 export하지 않습니다. 생성자만 export하고,
  service layer repository interface를 반환합니다.
- repository 생성 시점에 결정되는 invariant는 생성자에서 검증합니다.
  모든 repository 메서드에서 반복 방어하지 않습니다.

## Providers

- provider id와 provider group은 분리된 개념입니다. datago의 provider id는
  `datago`이고, 첫 group은 `securitiesProductPrice`입니다.
- provider group을 provider id에 `-` 또는 `/`로 붙이지 않습니다.
- provider adapter는 provider-native 응답을 canonical model 방향으로
  변환합니다.
- provider client는 endpoint path, service key, pagination,
  provider-native parsing, remote error context를 소유합니다.
- 외부 API 테스트는 fake HTTP transport 또는 `httptest`를 사용합니다.
  단위 테스트는 실제 public API 호출에 의존하지 않습니다.

## CLI

- CLI는 verb-first 구조를 유지하고 `README.md`와 일관되게 둡니다.
- machine-readable output은 예측 가능해야 합니다. JSON output은 `jq`로
  다루기 쉬운 구조를 우선하고, human table output은 간결해도 됩니다.
- stdout은 command 결과에 사용합니다. stderr는 diagnostics, progress,
  log에 사용합니다.

## Documentation

- 아키텍처 계약은 `docs/architectures` 아래에 둡니다.
- provider 목록과 provider별 구현 계획은 `docs/providers` 아래에 둡니다.
- `docs/providers/provider-package-contract.md`는 다시 만들지 않습니다. 이미
  삭제된 문서입니다.
