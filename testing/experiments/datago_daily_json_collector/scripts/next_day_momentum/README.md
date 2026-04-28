# Next-Day Momentum ETF Screen

Data.go.kr ETF 일별 JSON 스냅샷에서 다음 거래일 상승 가능 모멘텀이 있는 후보군을 찾는 실험 스크립트입니다.

이 스크리너는 매수 추천이나 예측 모델이 아니라, 스윙/모멘텀 트레이딩 관찰 후보를 빠르게 좁히기 위한 랭킹 도구입니다.

## 실행

JSON 결과:

```bash
testing/experiments/datago_daily_json_collector/scripts/next_day_momentum/screen_next_day_momentum.sh \
  --preset balanced \
  --format json
```

CSV 결과:

```bash
testing/experiments/datago_daily_json_collector/scripts/next_day_momentum/screen_next_day_momentum.sh \
  --preset balanced \
  --format csv
```

공격적 프리셋:

```bash
testing/experiments/datago_daily_json_collector/scripts/next_day_momentum/screen_next_day_momentum.sh \
  --preset aggressive \
  --format json
```

기본 출력 경로:

```text
tmp/testing/datago-daily-json-collector/analysis/
  next-day-momentum-balanced-etf-candidates.json
  next-day-momentum-balanced-etf-candidates.csv
  next-day-momentum-aggressive-etf-candidates.json
  next-day-momentum-aggressive-etf-candidates.csv
```

## 기본 아이디어

다음 거래일 후보군은 다음 네 가지가 동시에 보이는 ETF를 우선합니다.

- 최근 1/3/5/20거래일 수익률이 양수
- 최신 거래대금이 20거래일 평균보다 증가
- 최신 종가가 당일 고가권과 20거래일 고가권에 가까움
- 종가가 5일/20일 이동평균 위에 있음

기본 `balanced` 프리셋은 레버리지, 인버스, 곱버스 ETF를 제외합니다. `aggressive` 프리셋은 레버리지 ETF를 포함하되 인버스와 곱버스는 제외합니다.

## 프리셋

| 항목 | balanced | aggressive |
| --- | ---: | ---: |
| `minDays` | 60 | 40 |
| `minLatestTrPrc` | 500000000 | 300000000 |
| `minAvg20TrPrc` | 200000000 | 100000000 |
| `minAvgNPptTotAmt` | 10000000000 | 5000000000 |
| `minReturn1dPct` | 0 | -1 |
| `minReturn3dPct` | 0.5 | 0 |
| `minReturn5dPct` | 1.5 | 1 |
| `minReturn20dPct` | 2 | 0 |
| `maxReturn1dPct` | 12 | 20 |
| `minValueRatio20` | 1.3 | 1.1 |
| `minCloseLocationPct` | 55 | 45 |
| `minHigh20PositionPct` | 70 | 60 |
| `maxDailyVolatility20` | 8 | 15 |
| `maxDrawdown20Pct` | -18 | -30 |
| `highClosePullbackFreePct` | 2 | 4 |
| `highClosePullbackPenaltyWeight` | 1.2 | 0.7 |

## balanced 주요 필터

- `observationDays >= 60`
- `latestTrPrc >= 500000000`
- `avg20TrPrc >= 200000000`
- `avgNPptTotAmt >= 10000000000`
- `return1dPct >= 0`
- `return3dPct >= 0.5`
- `return5dPct >= 1.5`
- `return20dPct >= 2`
- `return1dPct <= 12`
- `valueRatio20 >= 1.3`
- `closeLocationPct >= 55`
- `high20PositionPct >= 70`
- `closeVsMA5Pct >= -0.5`
- `closeVsMA20Pct >= 0`
- `dailyVolatility20Pct <= 8`
- `maxDrawdown20Pct >= -18`

## 수치 계산

모든 수익률 단위는 `%`입니다. 가격은 `clpr`, 시가/고가/저가는 `mkp`, `hipr`, `lopr`, 거래대금은 `trPrc`, 순자산총액은 `nPptTotAmt`를 사용합니다.

최근 N거래일 수익률:

```text
returnNdPct = (latestClpr / clpr_N_trading_days_ago - 1) * 100
```

20거래일 거래대금 배율:

```text
avg20TrPrc = average(trPrc over latest 20 rows)
valueRatio20 = latestTrPrc / avg20TrPrc
```

당일 종가 위치:

```text
closeLocationPct = (latestClpr - latestLopr) / (latestHipr - latestLopr) * 100
```

고가 대비 종가 이탈률:

```text
closeFromHighPct = (latestClpr / latestHipr - 1) * 100
highClosePullbackPct = (1 - latestClpr / latestHipr) * 100
```

`closeFromHighPct`는 고가 대비 얼마나 아래에서 끝났는지를 음수로 보여주고, `highClosePullbackPct`는 점수 감점 계산을 위해 양수로 표현합니다.

20거래일 고가권 위치:

```text
high20PositionPct = (latestClpr - low20) / (high20 - low20) * 100
gapTo20dHighPct = (latestClpr / high20 - 1) * 100
```

이동평균 이격:

```text
ma5 = average(clpr over latest 5 rows)
ma20 = average(clpr over latest 20 rows)
closeVsMA5Pct = (latestClpr / ma5 - 1) * 100
closeVsMA20Pct = (latestClpr / ma20 - 1) * 100
ma5VsMA20Pct = (ma5 / ma20 - 1) * 100
```

20거래일 변동성:

```text
dailyReturn[i] = (clpr[i] / clpr[i - 1] - 1) * 100
dailyVolatility20Pct = sample_stddev(dailyReturn over latest 20 rows)
```

20거래일 최대 낙폭:

```text
runningPeak[i] = max(clpr[0..i])
drawdown[i] = (clpr[i] / runningPeak[i] - 1) * 100
maxDrawdown20Pct = min(drawdown)
```

## 점수

최종 `score`는 후보 정렬용 점수입니다. 값이 높을수록 최근 모멘텀과 거래대금 유입이 강한 후보로 먼저 봅니다.

```text
valuePulse = min(max(valueRatio20, 0), 5)

oneDayOverheatPenalty =
  max(return1dPct - overheat1dPct, 0) * overheatPenaltyWeight

fiveDayOverheatPenalty =
  max(return5dPct - overheat5dPct, 0) * overheatPenaltyWeight

highClosePullbackPenalty =
  max(highClosePullbackPct - highClosePullbackFreePct, 0)
  * highClosePullbackPenaltyWeight

score =
  return1dPct * 0.8
  + return3dPct * 1.2
  + return5dPct * 1.4
  + return20dPct * 0.45
  + closeVsMA5Pct * 0.8
  + closeVsMA20Pct * 0.45
  + ma5VsMA20Pct * 0.7
  + valuePulse * 4
  + high20PositionPct * 0.12
  + closeLocationPct * 0.06
  + liquidityBonus
  - dailyVolatility20Pct * 0.6
  + maxDrawdown20Pct * 0.3
  - oneDayOverheatPenalty
  - fiveDayOverheatPenalty
  - highClosePullbackPenalty
```

`maxDrawdown20Pct`는 음수이므로 점수에 더하면 최근 낙폭이 큰 후보의 점수가 낮아집니다.

## 옵션 예시

레버리지까지 포함한 공격적 프리셋으로 보려면:

```bash
testing/experiments/datago_daily_json_collector/scripts/next_day_momentum/screen_next_day_momentum.sh \
  --format json \
  --preset aggressive
```

거래대금 조건을 더 강하게 보려면:

```bash
testing/experiments/datago_daily_json_collector/scripts/next_day_momentum/screen_next_day_momentum.sh \
  --format csv \
  --min-latest-tr-prc 3000000000 \
  --min-value-ratio20 1.8
```
