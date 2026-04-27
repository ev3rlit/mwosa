# Indicators Feature Task

## 목적

`packages/indicators` 를 순수 계산 라이브러리로 사용할 수 있게 만드는 구현 태스크를 정리한다.

아키텍처 결정은 `docs/architectures/packages/indicators/README.md` 를 기준으로 삼고, 이 문서는 실제 구현 작업의 작은 출발점을 다룬다.

## 첫 작업

처음에는 모든 보조지표를 직접 구현하지 않는다.

`cinar/indicator` v1 의 단순 계산 함수를 감싸서 `packages/indicators` public API 로 제공한다. 외부 라이브러리 타입은 public API 로 노출하지 않는다.

## 기준

- `packages/indicators` 는 provider, storage, Cobra, presentation 에 의존하지 않는다.
- service layer 는 `packages/indicators` 를 호출해 계산 use case 를 조립한다.
- MACD, 이동평균, RSI 처럼 v1 에서 바로 감쌀 수 있는 지표부터 검토한다.
- 일목균형표처럼 시간 배치 의미가 중요한 지표는 `SourceTime`, `PlotTime` 기준을 먼저 확인한다.
- 각 공개 계산 함수는 `testify` 기반 단위 테스트를 반드시 가진다.
- golden fixture 와 허용 오차를 테스트에 명시한다.

## 완료 기준

- `packages/indicators` 의 최소 public type 이 정리된다.
- `cinar/indicator` v1 wrapper 가 외부 타입을 숨긴다.
- 대표 지표 1개 이상이 정상 입력과 invalid input 테스트를 가진다.
- service/calc 가 나중에 호출할 수 있는 입력/출력 모양이 보인다.
