# jq Screening Strategies

## 목적

`mwosa` 는 커스텀 DSL 을 새로 만들지 않고, 투자 데이터 스크리닝 쿼리 언어로
`jq` 를 우선 지원한다.

핵심 목표는 여러 provider 에 흩어진 투자 데이터를 `jq` 로 다루기 쉬운
JSON/NDJSON 으로 제공하고, 반복해서 쓰는 `jq` 쿼리를 스크리닝 전략으로
저장, 실행, 기록할 수 있게 만드는 것이다.

`mwosa` 의 역할은 새 언어를 정의하는 것이 아니라 다음을 안정적으로 제공하는
것이다.

- provider 별 원천 데이터를 정규화한 JSON shape
- AI 에이전트와 사람이 참고할 수 있는 schema 와 샘플 데이터
- inline `jq`, `jq` 파일, Unix pipeline, 저장된 전략 실행 흐름
- 반복 실행 결과와 사용한 입력 데이터의 재현 정보

## 배경

ETF 원천 JSON 파일을 여러 날짜로 모아두고 `jq` 로 처리해보면, 간단한
스크리닝은 별도 의존성 없이도 충분히 가능하다.

```bash
jq -s -f strategies/etf-low-vol-uptrend.jq data/etf-daily/*.json
```

이 방식은 단순하고 강하다. JSON 원본을 눈으로 확인할 수 있고, 쿼리는 텍스트
파일로 버전 관리할 수 있으며, shell pipeline 과도 잘 맞는다.

따라서 `mwosa` 는 MongoDB aggregation 과 비슷한 별도 DSL 을 만들기보다,
이미 레퍼런스가 많고 AI 에이전트도 잘 다루는 `jq` 를 공식 실행 경로로
지원한다.

## 사용 흐름

### 1. 직접 jq 입력

빠른 실험은 inline `jq` 로 실행한다.

```bash
mwosa screen etf \
  --jq 'map(select(.return_1y_pct > 0)) | sort_by(.volatility_12w_pct) | .[:30]'
```

이 방식은 조건을 즉석에서 바꿔보는 탐색 작업에 쓴다. 긴 쿼리나 반복 실행이
필요해지면 `jq` 파일이나 저장된 전략으로 옮긴다.

### 2. jq 파일 실행

조금 길어진 스크리닝은 `jq` 파일로 분리한다.

```bash
mwosa screen etf --jq-file strategies/etf-low-vol-uptrend.jq
```

또는 `mwosa` 의 JSON 출력과 표준 `jq` 를 직접 조합할 수 있다.

```bash
mwosa get etf-metrics --from 2025-01-01 --to 2026-01-01 --output json \
  | jq -f strategies/etf-low-vol-uptrend.jq
```

이 pipeline 경로는 `mwosa` 가 감싸지 않는 표준 Unix 조합으로 남겨둔다.

### 3. Unix pipeline

`mwosa` 는 표준 Unix pipeline 을 막지 않는다. 데이터 생성, 필터링, 후속 조회를
작은 명령으로 조합할 수 있어야 한다.

```bash
mwosa get etf-metrics --from 2025-01-01 --to 2026-01-01 --output json \
  | jq -f strategies/etf-low-vol-uptrend.jq \
  | mwosa inspect instrument --stdin --output table
```

NDJSON 을 쓰면 record 단위 필터링과 다른 도구와의 조합이 더 쉬워진다.

```bash
mwosa get etf-metrics --output ndjson \
  | jq -c 'select(.return_3m_pct > 0 and .avg_trading_value_20d >= 1000000000)' \
  | mwosa compare instruments --stdin --output json
```

pipeline 모드에서 `mwosa` 는 stdout 과 stderr 경계를 지킨다.

- stdout: 다음 명령이 읽을 수 있는 결과 데이터
- stderr: progress, warning, provider 호출 진단, 실행 요약
- stdin: symbol 목록, JSON array, NDJSON record 를 명령별로 명시해 수용

이 경로는 저장된 strategy 보다 느슨하다. 실행 기록을 자동으로 남기기보다,
사용자가 shell history, 파일 redirect, Git-tracked `jq` 파일로 실험을 관리할
수 있게 둔다.

### 4. 저장된 스크리닝 전략

반복해서 쓰는 `jq` 쿼리는 전략 리소스로 등록한다.

```bash
mwosa create strategy etf-lowvol \
  --engine jq \
  --input etf_daily_metrics \
  --jq-file strategies/etf-low-vol-uptrend.jq

mwosa screen strategy etf-lowvol --output table
mwosa screen strategy etf-lowvol --argjson limit 50 --output json
```

저장된 전략은 `jq` 쿼리만이 아니라 입력 데이터 종류, 기본 파라미터, 설명,
실행 기록 기준을 함께 가진다.

## 전략 모델

`jq` 파일은 전략을 만드는 입력 방식일 뿐이다. `mwosa create strategy` 가
성공하면 파일 경로가 아니라 `jq` 원문과 해시가 database 에 저장된다.
따라서 원본 파일이 이동되거나 삭제되어도 저장된 전략은 계속 실행할 수 있어야
한다.

inline query 와 file query 는 같은 저장 모델을 사용한다.

```bash
mwosa create strategy etf-lowvol \
  --engine jq \
  --input etf_daily_metrics \
  --jq 'map(select(.return_1y_pct > 0)) | sort_by(.volatility_12w_pct) | .[:30]'

mwosa create strategy etf-lowvol \
  --engine jq \
  --input etf_daily_metrics \
  --jq-file strategies/etf-low-vol-uptrend.jq
```

두 명령 모두 최종적으로는 `query_text` 와 `query_hash` 를 저장한다. `--jq-file`
은 파일 내용을 읽어오는 편의 옵션일 뿐, 저장 모델에는 파일 경로를 남기지
않는다.

초기 전략 리소스는 다음 정보를 가진다.

```json
{
  "id": "etf-lowvol",
  "engine": "jq",
  "input_dataset": "etf_daily_metrics",
  "input_schema_version": 1,
  "query_text": "map(select(.return_1y_pct > 0)) | sort_by(.volatility_12w_pct) | .[:30]",
  "query_hash": "sha256:...",
  "params": {
    "limit": 30,
    "min_trading_value": 1000000000,
    "max_mdd_pct": -15
  }
}
```

`input_dataset` 은 별도 projection 이 아니다. `jq` 에 넣을 전체 normalized
record 묶음의 이름이다. 예를 들어 `etf_daily_metrics` 는 ETF 일별 metric
record 전체를 뜻한다. 전략은 전체 record 를 입력으로 받고, `jq` 가 필요한
필드만 선택한다.

`input_schema_version` 은 해당 dataset 의 JSON field 이름, 단위, 의미가 바뀌는
것을 추적하기 위한 값이다. schema version 이 바뀌면 기존 전략이 여전히
호환되는지 `validate strategy` 또는 `screen strategy` 에서 경고할 수 있다.

`jq` 에서는 `--argjson` 으로 전달되는 파라미터를 사용할 수 있다.

```jq
map(select(.return_1y_pct > 0))
| map(select(.return_3m_pct > 0))
| map(select(.avg_trading_value_20d >= $min_trading_value))
| map(select(.mdd_1y_pct >= $max_mdd_pct))
| sort_by(.volatility_12w_pct)
| .[:$limit]
```

## 저장 스키마

전략 저장은 application level 에서 관리한다. 문서에서는 SQL DDL 대신 Go type 으로
테이블 모양을 정의한다. 실제 storage 구현은 이 model 을 SQLite schema 와
repository 로 옮긴다.

```go
package strategy

import "time"

type Engine string

const EngineJQ Engine = "jq"

// Strategy 는 사용자가 관리하는 스크리닝 전략의 현재 상태다.
// 실제 jq 원문은 StrategyVersion 에 저장하고, Strategy 는 활성 버전만 가리킨다.
type Strategy struct {
	ID              string     // 내부 식별자
	Name            string     // 사용자가 명령에서 부르는 이름
	Engine          Engine     // 현재는 jq 만 지원한다
	ActiveVersionID string     // screen strategy 가 기본으로 사용할 버전
	CreatedAt       time.Time  // 처음 생성된 시각
	UpdatedAt       time.Time  // 이름, 활성 버전, archive 상태가 바뀐 시각
	ArchivedAt      *time.Time // soft delete 시각. nil 이면 활성 전략이다
}

// StrategyVersion 은 실행 가능한 jq 원문과 그 원문이 기대하는 입력 계약이다.
// update strategy 는 기존 버전을 덮어쓰지 않고 새 StrategyVersion 을 만든다.
type StrategyVersion struct {
	ID                 string    // 내부 식별자
	StrategyID         string    // 소속 Strategy
	Version            int       // 사용자에게 보여줄 증가 버전
	QueryText          string    // 저장된 jq 원문
	QueryHash          string    // query_text 기준 해시. 중복 확인과 실행 복원에 사용한다
	InputDataset       string    // jq 에 넣을 전체 normalized dataset 이름
	InputSchemaVersion int       // dataset JSON shape 의 버전
	ParamsJSON         []byte    // 기본 --argjson 값
	CreatedAt          time.Time // 이 버전이 생성된 시각
	Note               string    // 변경 이유나 전략 설명
}

type ScreenRunStatus string

const (
	ScreenRunSucceeded ScreenRunStatus = "succeeded"
	ScreenRunFailed    ScreenRunStatus = "failed"
)

// ScreenRun 은 저장된 전략으로 후보군을 뽑은 스크리닝 이력이다.
// 큰 결과 payload 는 ScreenRunItem 으로 분리하고, ScreenRun 은 조회용 요약만 가진다.
type ScreenRun struct {
	ID                 string          // 필수. 내부 식별자
	Alias              string          // 선택. 사용자가 붙인 실행 기록 별칭. CLI 에서 screen-id 대신 참조할 수 있다
	StrategyID         string          // 필수. 스크리닝에 사용한 Strategy
	StrategyVersionID  string          // 필수. 스크리닝 당시 StrategyVersion
	QueryHash          string          // 필수. 스크리닝 당시 jq 해시
	InputDataset       string          // 필수. 스크리닝에 사용한 dataset
	InputSchemaVersion int             // 필수. 스크리닝 당시 dataset schema version
	ParamsJSON         []byte          // 선택. 스크리닝에 실제 적용한 --argjson 값
	DataFrom           string          // 선택. 조회 시작일
	DataTo             string          // 선택. 조회 종료일
	DataAsOf           string          // 선택. 스냅샷 기준일
	StartedAt          time.Time       // 필수. 스크리닝 시작 시각
	FinishedAt         *time.Time      // 선택. 스크리닝 종료 시각. 실행 중이면 nil 이다
	Status             ScreenRunStatus // 필수. 성공 또는 실패
	ResultCount        int             // 필수. 결과 row 수
	ResultHash         string          // 필수. 전체 결과 payload 기준 해시
	ResultSizeBytes    int64           // 필수. 전체 결과 payload byte 크기
	SummaryJSON        []byte          // 선택. history screen 에 필요한 작은 요약. SQLite 에서는 JSONB 저장을 검토한다
	ErrorMessage       string          // 선택. 실패 시 사용자에게 보여줄 에러
}

// ScreenRunItem 은 스크리닝 결과 row 하나를 보관한다.
// 점수, bucket, 통과 사유 같은 전략별 의미는 고정 필드로 빼지 않고 PayloadJSON 에 둔다.
type ScreenRunItem struct {
	ID          string // 필수. 내부 식별자
	ScreenRunID string // 필수. 소속 ScreenRun
	Ordinal     int    // 필수. jq 결과 배열에서 나온 순서
	Symbol      string // 선택. 검색용 symbol. 결과 row 에 없으면 비워둘 수 있다
	PayloadJSON []byte // 필수. 결과 row 전체 JSON. SQLite 에서는 JSONB 저장을 검토한다
}
```

`Strategy` 는 사용자가 보는 리소스다. `StrategyVersion` 은 실제 실행 가능한
`jq` 원문을 보관한다. `ScreenRun` 은 저장된 전략으로 후보군을 뽑은 스크리닝
기록이고, `ScreenRunItem` 은 그 결과 row 를 나누어 보관한다.

`update strategy` 는 기존 version 을 덮어쓰지 않고 새 version 을 만든다.
과거 스크리닝 기록은 항상 실행 당시의 `strategy_version_id` 와 `query_hash` 를
참조한다.

SQLite 구현에서는 `SummaryJSON` 과 `PayloadJSON` 을 TEXT JSON 으로 저장할 수도
있고, SQLite 3.45.0 이후 지원하는 JSONB BLOB 로 저장할 수도 있다. JSONB 는
SQLite 내부 binary JSON 표현이므로 앱 외부로 내보내는 포맷으로 쓰지 않는다.
CLI 출력, export, pipeline 에서는 항상 표준 JSON/NDJSON text 로 변환한다.

## AI 에이전트 지원

AI 에이전트가 `jq` 쿼리를 안정적으로 만들려면 전체 데이터베이스를 설명할
필요가 없다. 대신 `mwosa` 는 dataset 별 schema 와 샘플을 제공한다.

```bash
mwosa inspect schema etf_daily_metrics --output json
mwosa sample input etf_daily_metrics --limit 5 --output json
mwosa list metrics --input etf_daily_metrics --output table
```

에이전트는 이 정보를 보고 `jq` 쿼리를 만든다. 사용자는 생성된 쿼리를
검토하고, 괜찮으면 전략으로 저장한다.

```bash
mwosa create strategy etf-lowvol \
  --engine jq \
  --input etf_daily_metrics \
  --jq-file strategies/etf-low-vol-uptrend.jq
```

이 흐름에서는 `mwosa` 가 에이전트 전용 DSL 을 만들 필요가 없다. 에이전트는
표준 `jq` 를 만들고, `mwosa` 는 그 쿼리가 적용될 안정적인 데이터 계약을
제공한다.

## 명령 표면 초안

```bash
mwosa screen etf --jq '<jq expression>'
mwosa screen etf --jq-file <path>

mwosa get <input> --output json | jq -f <path>
mwosa get <input> --output ndjson | jq -c '<jq expression>' | mwosa <verb> --stdin

mwosa create strategy <name> --engine jq --input <dataset> --jq '<jq expression>'
mwosa create strategy <name> --engine jq --input <dataset> --jq-file <path>
mwosa list strategies
mwosa inspect strategy <name>
mwosa update strategy <name> --jq-file <path>
mwosa delete strategy <name>
mwosa screen strategy <name>
mwosa screen strategy <name> --alias <screen-alias>
mwosa history screen
mwosa inspect screen <screen-id>
mwosa inspect screen <screen-alias>

mwosa inspect schema <input>
mwosa sample input <input>
mwosa list metrics --input <input>
```

`screen` 은 일회성 실행에 가깝고, `strategy` 는 저장 가능한 반복 실행
리소스다.

## 실행 기록

저장된 전략으로 스크리닝하면 결과만 남기지 않는다. 나중에 같은 판단을 복원할
수 있도록 `ScreenRun` 에 다음 정보를 함께 기록한다.

- strategy id 와 version
- screen id 와 사용자가 지정한 alias
- engine 과 jq query hash
- input dataset 과 schema version
- 데이터 기준일과 조회 기간
- 사용한 provider 와 canonical data version
- 실행 파라미터
- 결과 row 수, 결과 hash, 결과 byte 크기
- history 화면에 필요한 작은 summary

후보군 전체 결과는 `ScreenRun.ResultJSON` 같은 단일 큰 컬럼에 넣지 않는다.
대신 jq 결과 배열을 row 단위로 나누어 `ScreenRunItem.PayloadJSON` 에 저장한다.
`ScreenRunItem` 은 결과 순서와 검색용 symbol 정도만 별도 필드로 가진다. score,
rank, bucket, 통과 사유 같은 전략별 의미는 고정 schema 로 강제하지 않고
`PayloadJSON` 안에 그대로 둔다.

사용자는 `history screen` 으로 최근 스크리닝 목록을 보고, `inspect screen
<screen-id>` 로 특정 스크리닝 결과와 당시 사용한 전략 버전, 개별 결과 row 를
확인한다.

`ScreenRun.Alias` 는 Kubernetes pod 이름처럼 사용자가 기억하고 참조하기 쉬운
고유 이름이다. 내부 `ID` 는 항상 유지하되, 사용자가 `--alias` 를 지정하면
`inspect screen <alias>` 로도 같은 ScreenRun 을 조회할 수 있다. alias 는 비워둘
수 있지만, 값이 있으면 전체 ScreenRun 안에서 unique 해야 한다.

## 초기 범위

- `jq` inline 실행
- `jq` 파일 실행
- Unix pipeline 에서 사용할 수 있는 JSON/NDJSON 출력과 stdin 입력
- `jq` 기반 strategy CRUD
- strategy 실행 결과 JSON/table 출력
- `history screen` 으로 최근 스크리닝 기록 조회
- input schema 조회
- input sample 조회
- metric catalog 조회

## 제외 범위

- `jq` 문법 재구현
- `mwosa` 전용 커스텀 DSL 설계
- MongoDB aggregation 호환 엔진 구현
- 백테스트 엔진
- 자동매매
- 종목 매수 추천

## 완료 기준

- 사용자가 `mwosa` 가 제공한 JSON schema 와 샘플만 보고 `jq` 쿼리를 작성할
  수 있다.
- inline `jq`, `jq` 파일, 저장된 strategy 실행 흐름이 같은 input shape 를
  사용한다.
- JSON/NDJSON 출력은 `jq` 와 다른 CLI 도구로 안전하게 pipe 할 수 있고,
  diagnostics 는 stdout 에 섞이지 않는다.
- 저장된 strategy 는 `list`, `inspect`, `update`, `delete` 로 관리하고,
  `screen strategy` 로 실행할 수 있다.
- 최근 스크리닝 결과는 `history screen` 으로 조회하고, 단일 결과는
  `inspect screen` 으로 확인할 수 있다.
- 실행 결과는 사람이 읽는 table 과 AI 에이전트가 읽는 JSON 으로 모두
  출력할 수 있다.
- 별도 DSL 없이도 ETF 스크리닝 루틴을 반복 실행할 수 있다.
