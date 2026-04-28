# ETF JSON Screening

## 목적

ETF 데이터를 먼저 외부 도구에서 다루기 쉬운 JSON 계층으로 제공하고, 그 위에 스크리닝 프리셋과 나중의 커스텀 DSL 을 얹는다.

초기 목표는 똑똑한 쿼리 언어를 바로 만드는 것이 아니라, `jq`, DuckDB, SQLite, Python, AI agent 가 같은 데이터를 쉽게 읽을 수 있게 하는 것이다.

## 배경

Data.go.kr ETF/ETN 일별 원천 데이터를 날짜 단위 JSON 파일로 수집해보니, 1년치 전체 ETF 데이터도 개인 CLI 에서 다룰 수 있는 크기였다. 이 데이터만으로도 다음과 같은 후보 추출이 가능했다.

- 1년 수익률 양수
- 최근 3개월 수익률 양수
- 주간 변동성 낮음
- 최대 낙폭 작음
- 평균 거래대금과 순자산총액 기준 충족
- 레버리지/인버스 제외
- 최근 1개월 급등 종목 감점

따라서 `mwosa` 의 ETF 기능은 원천 데이터 확보와 JSON 출력 안정화를 먼저 처리하고, 자주 반복되는 분석 패턴을 CLI 명령으로 승격시키는 방향이 적합하다.

## 방향

### 1. JSON 출력 우선

모든 데이터 조회와 스크리닝 결과는 기계가 읽기 좋은 구조화 출력이 가능해야 한다.

```bash
mwosa get daily --asset etf --from 2025-04-28 --to 2026-04-27 --output json
mwosa get daily --asset etf --from 2025-04-28 --to 2026-04-27 --output ndjson
mwosa screen etf low-vol-uptrend --output json
mwosa screen etf low-vol-uptrend --output csv
```

기본 터미널 출력은 사람이 읽기 좋은 table 일 수 있지만, `json`, `ndjson`, `csv` 는 안정적인 계약으로 관리한다.

### 2. Raw 와 Normalized 분리

원천 API 응답은 검증과 재처리를 위해 보존할 수 있다. 하지만 CLI 의 기본 조회 대상은 provider 중립적인 normalized record 로 둔다.

- raw JSON: provider 원본 응답 보존, 재처리, 디버깅 용도
- normalized JSON/NDJSON: `jq`, DuckDB, SQLite import, agent tool 입력 용도
- derived result JSON: 수익률, 변동성, MDD, 점수 같은 계산 결과 포함

### 3. jq 친화성

`mwosa` 가 jq 문법을 직접 재구현하지 않는다. 먼저 파이프와 파일 입출력만으로 jq 와 잘 붙는 형태를 제공한다.

```bash
mwosa get daily --asset etf --output json \
  | jq 'map(select(.itmsNm | contains("미국채")))'
```

나중에 필요하면 `--jq` 같은 편의 옵션을 검토할 수 있지만, 핵심은 자체 jq 방언을 만드는 것이 아니라 안정적인 JSON 데이터를 내보내는 것이다.

### 4. 도메인 프리셋

반복해서 쓰는 분석은 사용자가 매번 jq 를 작성하지 않아도 되도록 `screen` 명령으로 제공한다.

```bash
mwosa screen etf low-vol-uptrend --from 2025-04-28 --to 2026-04-27
mwosa screen etf weekly-return --days 5 --limit 100
mwosa screen etf low-drawdown --max-mdd -10
```

프리셋은 내부적으로 명확한 계산식과 필터 조건을 가진다. 사용자는 `--explain` 으로 점수와 탈락 사유를 확인할 수 있어야 한다.

### 5. 커스텀 DSL 은 나중에

커스텀 DSL 은 처음부터 만들지 않는다. JSON 출력과 프리셋 명령을 사용하면서 반복되는 패턴이 보이면 그때 작게 도입한다.

가능한 방향은 MongoDB aggregation 과 비슷한 pipeline 형식이다.

```json
[
  { "match": { "assetType": "etf" } },
  { "metric": { "return1yPct": { "return": ["clpr", "1y"] } } },
  { "match": { "return1yPct": { "gt": 0 } } },
  { "sort": { "score": "desc" } },
  { "limit": 100 }
]
```

이 DSL 은 범용 데이터베이스를 흉내 내기보다, ETF 리서치에서 반복되는 필터, 지표 계산, 정렬, 설명 출력을 안정적으로 표현하는 정도로 제한한다.

## 초기 범위

- ETF/ETN 일별 가격 데이터 조회
- `json`, `ndjson`, `csv`, `table` 출력
- 날짜 범위, 자산 유형, provider 선택
- 저변동 우상향 ETF 후보 스크리닝
- 스크리닝 결과에 점수, 주요 지표, 필터 통과 이유 포함

## 제외 범위

- 자동매매
- 종목 매수 추천
- jq 문법 직접 구현
- 범용 MongoDB aggregation 엔진 구현
- 운용보수, 분배금, holdings, 섹터/국가 비중의 완전 자동 수집

## 열어둘 질문

- normalized daily record 의 최소 필드는 무엇으로 고정할 것인가?
- raw JSON 은 장기 보관할 것인가, SQLite 반영 후 검증용으로만 둘 것인가?
- `screen` 결과의 점수 계산식은 전략별로 버전 관리할 것인가?
- watchlist 종목은 필터 탈락 여부와 함께 별도 섹션으로 보여줄 것인가?
- DSL 을 도입한다면 JSON pipeline 파일만 받을 것인가, 짧은 inline expression 도 지원할 것인가?
