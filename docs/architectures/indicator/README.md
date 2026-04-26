# Indicator Architecture

## 목적

이 문서는 `mwosa` 의 지표와 추세 계산 레이어를 설명하는 가이드다.

`mwosa` 의 첫 실사용 흐름은 한국 주식/ETF 스윙 트레이딩이다. 이 흐름에서는 종목을 추천하거나 매매를 자동화하는 것보다, AI 에이전트와 사람이 같은 데이터를 보고 추세, 거래량, 변동성, 모멘텀, 가격 위치를 해석할 수 있게 만드는 것이 중요하다.

지표 레이어는 provider 에서 가져온 원천 데이터를 그대로 보여주는 레이어가 아니다. provider router 로 확보한 가격, 거래량, 종목, 시장 context 를 `mwosa` 내부 canonical data 로 정규화한 뒤, 그 위에서 계산값과 판단 context 를 만든다.

## 범위

초기 범위:

- daily candle 기반 추세 지표
- 거래량과 상대 거래량
- 기간 수익률과 모멘텀
- 변동성과 drawdown
- 가격 위치와 돌파/이탈 여부
- 스윙 후보 스크리닝에 필요한 계산 context

나중에 확장할 범위:

- 거시 지표와 시장 regime context
- 포트폴리오 단위 risk, beta, correlation
- 공시, 뉴스, 실적 이벤트와 가격 반응 연결
- 주간/월간 리서치용 요약 지표

범위 밖:

- 종목 추천
- 자동매매
- 주문 실행
- 지표 결과를 근거로 한 매수/매도 지시
- 고급 백테스트 엔진

## 원칙

지표 계산은 provider 차이를 숨기되, 원천과 계산 과정을 숨기지 않는다.

- 원천 데이터는 provider router 를 통해 가져온다.
- 지표 계산은 canonical data 위에서 수행한다.
- 계산 결과는 `table`, `json`, `ndjson`, `csv` 출력으로 다시 사용할 수 있어야 한다.
- `--explain` 에서는 어떤 데이터 범위와 provider 를 사용했는지 확인할 수 있어야 한다.
- 데이터가 부족하면 빈 성공처럼 보이지 않고, 어떤 입력이 부족한지 명확히 알려야 한다.

## 요청 흐름

```text
CLI command
  -> service
  -> provider router
  -> canonical data
  -> indicator calculator
  -> screen / inspect / compare result
  -> presentation
```

`calc` 는 단일 지표 계산에 가깝고, `screen` 과 `inspect` 는 여러 지표를 묶어 사용자 흐름에 맞게 보여준다.

예:

```text
mwosa calc indicator 005930 rsi --window 14
mwosa calc relative-volume 005930 --window 20
mwosa screen swing --universe krx-etf --as-of 2026-04-24
mwosa inspect 491820 --explain
```

## 스윙용 기본 지표 묶음

초기 스윙 흐름에서는 지표를 개별 값으로만 보지 않고, 후보 판단에 필요한 묶음으로 본다.

| 묶음 | 예시 지표 | 의미 |
| --- | --- | --- |
| 추세 | SMA, EMA, 고점/저점 구조, 이동평균 기울기 | 가격이 어느 방향으로 움직이는지 본다. |
| 모멘텀 | 기간 수익률, 상대 강도, 신고가/저점 대비 위치 | 최근 힘이 붙는지 본다. |
| 거래량 | 거래량 증가율, 상대 거래량, 거래대금 | 움직임에 참여가 있는지 본다. |
| 변동성 | ATR, 표준편차, drawdown | 손절 폭과 포지션 크기 판단에 쓴다. |
| 가격 위치 | 52주 고점/저점 대비 위치, 박스권 돌파/이탈 | 진입 위치가 너무 늦었는지 본다. |
| 리스크 | reward/risk, 손절 기준 하락률, beta | 감당 가능한 거래인지 본다. |

이 묶음은 스윙에 먼저 맞추지만, 같은 계산기는 ETF 비교, 지수 분석, 포트폴리오 점검에도 재사용한다.

## Provider 와의 관계

지표 계산기는 특정 provider 를 직접 호출하지 않는다. 필요한 데이터 종류만 service 에 요청하고, service 는 provider router 를 통해 호환 provider 를 찾는다.

예를 들어 스윙 후보를 계산하려면 보통 아래 capability 가 필요하다.

```text
instrument -> 종목 이름, 시장, 자산 유형
candles    -> OHLCV 시계열
quote      -> 최신 가격 또는 최신 snapshot
index      -> 기준 지수 비교
macro      -> 시장 context 확장
news       -> 이벤트 context 확장
```

provider router 는 capability, market, security type, freshness, auth 상태, `--provider`, `--prefer-provider` 를 보고 후보를 고른다. 계산 결과에는 사용한 provider, 기간, 누락 데이터, fallback 여부를 남겨야 한다.

## 저장과 캐시

초기에는 지표 계산 결과를 정본으로 저장하지 않는다.

- 원천 canonical data 를 저장한다.
- 지표 결과는 요청 시 계산한다.
- 계산 결과 캐시는 필요가 분명해질 때 도입한다.
- 지표 결과를 저장하더라도 원천 데이터와 계산 파라미터를 함께 남겨야 한다.

이 기준을 두는 이유는 provider 데이터가 갱신되거나 계산식이 바뀌었을 때, 오래된 계산 결과가 정본처럼 남는 문제를 피하기 위해서다.

## 출력 기준

사람이 보는 출력은 요약과 이유를 함께 보여준다. AI 에이전트가 다시 처리하는 출력은 필드를 안정적으로 유지한다.

예시 JSON shape:

```json
{
  "symbol": "491820",
  "as_of": "2026-04-24",
  "market": "krx",
  "indicators": {
    "trend": {
      "sma_20": 12345.67,
      "sma_60": 12001.23,
      "price_vs_sma_20_pct": 3.2
    },
    "volume": {
      "relative_volume_20d": 1.8
    },
    "risk": {
      "atr_14": 210.5,
      "drawdown_20d_pct": -4.7
    }
  },
  "provenance": {
    "providers": ["datago"],
    "range": {
      "from": "2025-10-24",
      "to": "2026-04-24"
    }
  }
}
```

필드 이름은 계산식보다 오래 살아야 한다. 계산식이 바뀌는 경우에는 schema version 또는 calculation version 을 함께 둔다.

## 관련 문서

- `README.md`
- `docs/architectures/layers/README.md`
- `docs/architectures/provider/README.md`
- `docs/providers/README.md`
