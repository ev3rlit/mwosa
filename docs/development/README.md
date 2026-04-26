# Development Collaboration Guide

## 목적

이 문서는 `mwosa` 를 함께 구현할 때의 개발 협업 기준을 정리한다.

아키텍처 문서는 레이어와 책임을 설명하고, 이 문서는 구현 중 어떤 결정을 지금 고정하고 어떤 결정을 나중으로 미룰지 정한다.

## 기본 원칙

- CLI core 는 provider 의 구현 방식을 모른다.
- service 는 provider role interface 에만 의존한다.
- provider adapter 는 external provider package 와 `mwosa` role interface 사이를 연결한다.
- provider 가 REST API, SDK, 파일, scraping 중 무엇을 쓰는지는 provider 구현 단계에서 정한다.
- 구현 라이브러리 선택은 필요가 보일 때 작게 결정하고, core contract 로 새지 않게 한다.

## HTTP Client 선택

provider 가 REST API 를 사용할 수도 있지만, 모든 provider 가 REST API 를 쓰는 것은 아니다. 따라서 HTTP client 라이브러리는 지금 공통 기술로 고정하지 않는다.

초기 기준은 다음과 같다.

- 기본값은 Go 표준 라이브러리 `net/http` 로 본다.
- retry/backoff 정책이 반복되면 `go-retryablehttp` 같은 얇은 wrapper 를 검토한다.
- REST request 작성 코드가 지나치게 장황해지면 `resty` 나 `req` 같은 고수준 client 를 검토한다.
- `fasthttp` 같은 성능 특화 client 는 측정된 병목이 있을 때만 검토한다.

지켜야 할 경계:

- provider role interface 에 특정 HTTP client type 을 노출하지 않는다.
- service request/result 에 HTTP 라이브러리 전용 옵션을 넣지 않는다.
- timeout, retry, rate limit, provenance 는 provider config 와 provider result 의 의미로 표현한다.
- REST provider 테스트는 가능하면 `httptest` 나 provider-local fake transport 로 작성한다.

## Provider 추가 흐름

새 provider 를 붙일 때는 아래 순서로 진행한다.

1. 필요한 role interface 를 먼저 확인한다.
2. provider 가 지원하는 capability 와 제한사항을 profile 로 적는다.
3. provider 구현체가 REST 인지 SDK 인지 파일 기반인지 확인한다.
4. 구현체에 맞는 transport/library 를 provider package 또는 provider 별 adapter 안에서만 선택한다.
5. external response 를 canonical data 로 변환한다.
6. 실패는 성공처럼 숨기지 않고 provider 이름, operation, symbol, market 같은 맥락을 붙여 반환한다.
7. service 에는 provider 별 adapter type 이 아니라 role interface 와 router 결과만 전달한다.

## 문서 갱신 기준

한 provider 에만 필요한 선택은 provider 문서에 적는다.

여러 provider 에 반복되는 선택은 이 문서에 올린다.

core contract 에 영향을 주는 선택은 `docs/architectures/layers/README.md` 또는 `docs/architectures/provider/README.md` 에 반영한다.

기술 스택으로 고정할 만큼 반복 사용이 확인된 선택은 `docs/architectures/tech-stack/README.md` 로 옮긴다.
