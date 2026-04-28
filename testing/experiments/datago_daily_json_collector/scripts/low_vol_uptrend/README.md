# Datago JSON analysis scripts

수집된 Datago 일별 JSON 스냅샷을 `jq`로 분석하는 스크립트 모음입니다. 현재는 저변동 우상향 ETF 후보 추출 스크립트를 제공합니다.

## 저변동 우상향 ETF 후보 추출

`screen_low_vol_uptrend.sh`는 원본 JSON 파일을 읽어 ETF별 시계열을 만들고, 변동성은 낮고 완만하게 우상향한 후보를 JSON 또는 CSV로 출력합니다.

JSON으로 저장하려면:

```bash
testing/experiments/datago_daily_json_collector/scripts/screen_low_vol_uptrend.sh \
  --format json
```

CSV로 저장하려면:

```bash
testing/experiments/datago_daily_json_collector/scripts/screen_low_vol_uptrend.sh \
  --format csv
```

기본 출력 경로는 다음과 같습니다.

```text
tmp/testing/datago-daily-json-collector/analysis/
  low-vol-uptrend-etf-candidates.json
  low-vol-uptrend-etf-candidates.csv
```

필터를 조정할 수도 있습니다.

```bash
testing/experiments/datago_daily_json_collector/scripts/screen_low_vol_uptrend.sh \
  --format json \
  --top 100 \
  --min-avg-tr-prc 500000000 \
  --min-avg-nppt-tot-amt 50000000000 \
  --max-weekly-volatility 3 \
  --max-drawdown-pct -15 \
  --min-positive-week-ratio 0.6 \
  --recent-surge-threshold-pct 10 \
  --recent-surge-penalty-weight 1.5
```

## 기본 필터

기본 필터는 다음 조건을 모두 만족한 ETF만 후보로 남깁니다.

- `observationDays >= 120`
- `return1yPct > 0`
- `return3mPct > 0`
- `weeklyVolatilityPct <= 3`
- `maxDrawdownPct >= -15`
- `positiveWeekRatio >= 0.55`
- `avgTrPrc >= 100000000`
- `avgNPptTotAmt >= 10000000000`
- `itmsNm + bssIdxIdxNm`이 `레버리지|인버스|곱버스|2X|2x|3X|3x`와 매칭되지 않음

## 수치 계산 수식

모든 수익률 단위는 `%`입니다. 가격은 `clpr`, 거래대금은 `trPrc`, 순자산총액은 `nPptTotAmt`를 사용합니다.

`return1yPct`는 관측 첫 거래일 종가 대비 최신 종가 수익률입니다.

```text
return1yPct = (latestClpr / firstClpr - 1) * 100
```

`return3mPct`는 최근 60거래일 전 종가 대비 최신 종가 수익률입니다.

```text
return3mPct = (latestClpr / clpr_60_trading_days_ago - 1) * 100
```

`return1mPct`는 최근 20거래일 전 종가 대비 최신 종가 수익률입니다.

```text
return1mPct = (latestClpr / clpr_20_trading_days_ago - 1) * 100
```

`weeklyReturnAvgPct`와 `weeklyVolatilityPct`는 5거래일 간격 수익률 배열로 계산합니다.

```text
weeklyReturn[i] = (clpr[i] / clpr[i - 5] - 1) * 100
weeklyReturnAvgPct = average(weeklyReturn)
weeklyVolatilityPct = sample_stddev(weeklyReturn)
```

`positiveWeekRatio`는 5거래일 수익률 중 양수인 비율입니다.

```text
positiveWeekRatio = count(weeklyReturn > 0) / count(weeklyReturn)
```

`maxDrawdownPct`는 관측 기간 중 이전 고점 대비 가장 큰 하락률입니다.

```text
runningPeak[i] = max(clpr[0..i])
drawdown[i] = (clpr[i] / runningPeak[i] - 1) * 100
maxDrawdownPct = min(drawdown)
```

`avgTrPrc`와 `avgNPptTotAmt`는 관측 기간 평균입니다.

```text
avgTrPrc = average(trPrc)
avgNPptTotAmt = average(nPptTotAmt)
```

최근 1개월 급등 페널티는 최근 20거래일 수익률이 기준값을 넘을 때만 적용합니다. 기본 기준값은 `10`, 기본 가중치는 `1.5`입니다.

```text
recentSurgePenalty =
  max(return1mPct - recentSurgeThresholdPct, 0)
  * recentSurgePenaltyWeight
```

최종 `score`는 후보 정렬용 점수입니다. 투자 판단용 절대 점수가 아니라, “저변동 우상향처럼 보이는 후보”를 먼저 보기 위한 랭킹 값입니다.

```text
liquidityBonus =
  if avgTrPrc >= minAvgTrPrc * 5 then 5
  else if avgTrPrc >= minAvgTrPrc * 2 then 2
  else 0

assetBonus =
  if avgNPptTotAmt >= minAvgNPptTotAmt * 5 then 5
  else if avgNPptTotAmt >= minAvgNPptTotAmt * 2 then 2
  else 0

score =
  return1yPct * 0.25
  + return3mPct * 0.35
  + positiveWeekRatio * 25
  - weeklyVolatilityPct * 2.5
  + maxDrawdownPct * 0.8
  + liquidityBonus
  + assetBonus
  - recentSurgePenalty
```

`maxDrawdownPct`는 음수이므로 `+ maxDrawdownPct * 0.8`은 낙폭이 클수록 점수를 깎는 효과를 냅니다.

## 주요 옵션

- `--format json|csv`: 출력 형식입니다. 기본값은 `json`입니다.
- `--top N`: 출력 후보 수입니다. 기본값은 `50`입니다.
- `--min-days N`: 최소 관측 일수입니다. 기본값은 `120`입니다.
- `--min-avg-tr-prc N`: 최소 평균 거래대금입니다. 기본값은 `100000000`입니다.
- `--min-avg-nppt-tot-amt N`: 최소 평균 순자산총액입니다. 기본값은 `10000000000`입니다.
- `--max-weekly-volatility N`: 최대 주간 변동성입니다. 기본값은 `3`입니다.
- `--max-drawdown-pct N`: 허용 최대 낙폭입니다. 기본값은 `-15`입니다.
- `--min-positive-week-ratio N`: 최소 상승 주간 비율입니다. 기본값은 `0.55`입니다.
- `--one-month-window N`: 1개월 급등 페널티 계산용 거래일 수입니다. 기본값은 `20`입니다.
- `--recent-surge-threshold-pct N`: 급등 페널티 시작 기준입니다. 기본값은 `10`입니다.
- `--recent-surge-penalty-weight N`: 급등 페널티 가중치입니다. 기본값은 `1.5`입니다.
- `--weekly-window N`: 주간 수익률 계산용 거래일 수입니다. 기본값은 `5`입니다.
- `--three-month-window N`: 3개월 수익률 계산용 거래일 수입니다. 기본값은 `60`입니다.
- `--exclude-regex REGEX`: 제외할 ETF명/기초지수명 정규식입니다.
