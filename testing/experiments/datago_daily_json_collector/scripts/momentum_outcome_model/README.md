# Momentum Outcome Model

수집된 Data.go.kr ETF 일봉 JSON으로 모멘텀 스크리너의 결과를 검증하는 실험입니다.

첫 버전은 머신러닝 모델을 바로 학습하지 않고, **feature/label 데이터셋**과 **질문별 통계 리포트**를 만듭니다. 목표는 현재 스크리너가 실제 다음 1~5거래일 결과와 어떤 관계가 있는지 확인하는 것입니다.

## 질문

이 실험은 다음 질문에 답합니다.

- 이 후보가 다음 3거래일 안에 `+3%`를 찍을 확률은?
- 다음 5거래일 안에 `-3%`를 먼저 맞을 위험은?
- 고가 대비 종가 이탈률이 큰 후보는 다음날 약한가?
- 거래대금 배율이 높을수록 다음날 지속성이 있는가?
- `balanced`와 `aggressive` 둘 다 잡힌 후보가 실제로 더 나은가?

## 실행

데이터셋을 만듭니다.

```bash
testing/experiments/datago_daily_json_collector/scripts/momentum_outcome_model/build_momentum_dataset.py
```

질문별 리포트를 만듭니다.

```bash
testing/experiments/datago_daily_json_collector/scripts/momentum_outcome_model/analyze_momentum_outcomes.py
```

기본 산출물은 다음 경로에 생성됩니다.

```text
tmp/testing/datago-daily-json-collector/analysis/
  momentum-outcome-dataset.csv
  momentum-outcome-report.json
  momentum-outcome-report.md
```

전체 데이터셋은 `ETF x 날짜` 단위라 행과 컬럼이 많습니다. 스프레드시트에서 직접 열려면 후보로 잡힌 행만, 핵심 컬럼만 저장하는 축약 파일을 만드는 편이 좋습니다.

```bash
testing/experiments/datago_daily_json_collector/scripts/momentum_outcome_model/build_momentum_dataset.py \
  --hits-only \
  --compact \
  --output tmp/testing/datago-daily-json-collector/analysis/momentum-outcome-hits-compact.csv
```

## Feature

주요 입력 feature는 현재 `next_day_momentum` 스크리너와 같은 계열입니다.

- `return1dPct`, `return3dPct`, `return5dPct`, `return20dPct`, `return60dPct`
- `valueRatio20`, `volumeRatio20`
- `closeLocationPct`, `closeFromHighPct`, `highClosePullbackPct`
- `high20PositionPct`, `gapTo20dHighPct`
- `closeVsMA5Pct`, `closeVsMA20Pct`, `ma5VsMA20Pct`
- `dailyVolatility20Pct`, `maxDrawdown20Pct`
- `balancedHit`, `aggressiveHit`, `overlapHit`
- `balancedScore`, `aggressiveScore`

## Label

현재 종가를 기준 진입가로 보고, 이후 거래일의 고가/저가/종가로 label을 만듭니다.

```text
next1dReturnPct = (next1Close / currentClose - 1) * 100
next3dHighReturnPct = (max(high over next 3 rows) / currentClose - 1) * 100
next5dLowReturnPct = (min(low over next 5 rows) / currentClose - 1) * 100
```

이벤트 label은 다음과 같습니다.

```text
next3dPlus3Hit = next3dHighReturnPct >= 3
next5dMinus3Hit = next5dLowReturnPct <= -3
```

`next5dFirstEvent3Pct`는 `+3%` 목표와 `-3%` 손절 중 무엇이 먼저 닿았는지 기록합니다. 일봉에서는 같은 날 목표와 손절을 모두 터치한 경우 선후관계를 알 수 없으므로, 보수적으로 `stop`을 먼저 발생한 것으로 처리합니다.

## 해석 주의

- 이 리포트는 실행 가능한 자동매매 백테스트가 아니라 리서치용 검증입니다.
- 일봉만 사용하므로 장중 체결 순서, 슬리피지, 호가 공백은 반영하지 않습니다.
- 최신 며칠은 미래 1~5거래일 데이터가 부족하므로 일부 label이 비어 있습니다.
- 이후 ML 모델을 붙일 때는 날짜 기준 train/test split을 사용해야 합니다.
