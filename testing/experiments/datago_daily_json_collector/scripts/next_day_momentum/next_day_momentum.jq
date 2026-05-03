def num:
  if . == null or . == "" then
    null
  else
    (tostring | gsub(","; "") | tonumber?)
  end;

def pow10($n):
  reduce range(0; $n) as $_ (1; . * 10);

def roundn($n):
  if . == null then
    null
  else
    pow10($n) as $p | ((. * $p) | round) / $p
  end;

def avg:
  if length == 0 then
    null
  else
    add / length
  end;

def stddev:
  if length <= 1 then
    null
  else
    avg as $mean
    | (map((. - $mean) * (. - $mean)) | add / (length - 1) | sqrt)
  end;

def maxn:
  if length == 0 then
    null
  else
    max
  end;

def minn:
  if length == 0 then
    null
  else
    min
  end;

def tail($n):
  if length <= $n then
    .
  else
    .[(length - $n):]
  end;

def pct($start; $end):
  if $start == null or $start == 0 or $end == null then
    null
  else
    (($end / $start) - 1) * 100
  end;

def clamp($min; $max):
  if . == null then
    null
  elif . < $min then
    $min
  elif . > $max then
    $max
  else
    .
  end;

def daily_returns:
  . as $rows
  | [
      range(1; ($rows | length)) as $index
      | pct($rows[$index - 1].clpr; $rows[$index].clpr)
      | select(. != null)
    ];

def max_drawdown:
  reduce .[] as $row (
    {peak: null, maxDrawdownPct: 0};
    if $row.clpr == null then
      .
    elif .peak == null or $row.clpr > .peak then
      .peak = $row.clpr
    else
      pct(.peak; $row.clpr) as $drawdown
      | if $drawdown < .maxDrawdownPct then
          .maxDrawdownPct = $drawdown
        else
          .
        end
    end
  )
  | .maxDrawdownPct;

def raw_etf_rows:
  [
    .[]
    | .basDt as $snapshotBasDt
    | .products[]?
    | select(.product == "etf")
    | .items[]?
    | {
        basDt: ((.basDt // $snapshotBasDt) | tostring),
        srtnCd: (.srtnCd | tostring),
        isinCd: (.isinCd // "" | tostring),
        itmsNm: (.itmsNm // "" | tostring),
        bssIdxIdxNm: (.bssIdxIdxNm // "" | tostring),
        clpr: (.clpr | num),
        mkp: (.mkp | num),
        hipr: (.hipr | num),
        lopr: (.lopr | num),
        trPrc: (.trPrc | num),
        trqu: (.trqu | num),
        nPptTotAmt: (.nPptTotAmt | num),
        mrktTotAmt: (.mrktTotAmt | num),
        nav: (.nav | num),
        fltRt: (.fltRt | num)
      }
    | select(.basDt != "" and .srtnCd != "" and .clpr != null)
  ];

def not_excluded($regex):
  if $regex == "" then
    true
  else
    ((.itmsNm + " " + .bssIdxIdxNm) | test($regex; "i") | not)
  end;

def ret_at($rows; $window):
  ($rows | length) as $rowCount
  | if $rowCount > $window then
      pct($rows[$rowCount - 1 - $window].clpr; $rows[-1].clpr)
    else
      null
    end;

def summarize_symbol:
  sort_by(.basDt) as $rows
  | ($rows | length) as $rowCount
  | ($rows[-1]) as $latest
  | ($rows | tail(5)) as $rows5
  | ($rows | tail(20)) as $rows20
  | ([ $rows20[].trPrc | select(. != null) ] | avg) as $avg20TrPrc
  | ([ $rows20[].trqu | select(. != null) ] | avg) as $avg20Trqu
  | ([ $rows20[] | (.hipr // .clpr) | select(. != null) ] | maxn) as $high20
  | ([ $rows20[] | (.lopr // .clpr) | select(. != null) ] | minn) as $low20
  | ([ $rows[].nPptTotAmt | select(. != null) ] | avg) as $avgNPptTotAmt
  | ([ $rows[].mrktTotAmt | select(. != null) ] | avg) as $avgMrktTotAmt
  | ($rows20 | daily_returns) as $dailyReturns20
  | {
      srtnCd: $latest.srtnCd,
      isinCd: $latest.isinCd,
      itmsNm: $latest.itmsNm,
      bssIdxIdxNm: $latest.bssIdxIdxNm,
      firstBasDt: $rows[0].basDt,
      latestBasDt: $latest.basDt,
      observationDays: $rowCount,
      latestClpr: $latest.clpr,
      latestMkp: $latest.mkp,
      latestHipr: $latest.hipr,
      latestLopr: $latest.lopr,
      latestNav: $latest.nav,
      latestTrPrc: $latest.trPrc,
      latestTrqu: $latest.trqu,
      latestFltRt: $latest.fltRt,
      return1dPct: (ret_at($rows; 1) | roundn(4)),
      return3dPct: (ret_at($rows; 3) | roundn(4)),
      return5dPct: (ret_at($rows; 5) | roundn(4)),
      return20dPct: (ret_at($rows; 20) | roundn(4)),
      return60dPct: (ret_at($rows; 60) | roundn(4)),
      ma5: ([ $rows5[].clpr | select(. != null) ] | avg | roundn(4)),
      ma20: ([ $rows20[].clpr | select(. != null) ] | avg | roundn(4)),
      avg20TrPrc: ($avg20TrPrc | roundn(0)),
      avg20Trqu: ($avg20Trqu | roundn(0)),
      avgNPptTotAmt: ($avgNPptTotAmt | roundn(0)),
      avgMrktTotAmt: ($avgMrktTotAmt | roundn(0)),
      valueRatio20: (
        if $avg20TrPrc == null or $avg20TrPrc == 0 or $latest.trPrc == null then
          null
        else
          ($latest.trPrc / $avg20TrPrc)
        end
        | roundn(4)
      ),
      volumeRatio20: (
        if $avg20Trqu == null or $avg20Trqu == 0 or $latest.trqu == null then
          null
        else
          ($latest.trqu / $avg20Trqu)
        end
        | roundn(4)
      ),
      closeLocationPct: (
        if $latest.hipr == null or $latest.lopr == null or $latest.hipr == $latest.lopr then
          null
        else
          (($latest.clpr - $latest.lopr) / ($latest.hipr - $latest.lopr) * 100)
        end
        | roundn(4)
      ),
      closeFromHighPct: (
        pct($latest.hipr; $latest.clpr)
        | roundn(4)
      ),
      highClosePullbackPct: (
        if $latest.hipr == null or $latest.hipr == 0 or $latest.clpr == null then
          null
        else
          ((1 - ($latest.clpr / $latest.hipr)) * 100)
        end
        | roundn(4)
      ),
      high20PositionPct: (
        if $high20 == null or $low20 == null or $high20 == $low20 then
          null
        else
          (($latest.clpr - $low20) / ($high20 - $low20) * 100)
        end
        | roundn(4)
      ),
      gapTo20dHighPct: (pct($high20; $latest.clpr) | roundn(4)),
      closeVsMA5Pct: (
        ([ $rows5[].clpr | select(. != null) ] | avg) as $ma5
        | pct($ma5; $latest.clpr)
        | roundn(4)
      ),
      closeVsMA20Pct: (
        ([ $rows20[].clpr | select(. != null) ] | avg) as $ma20
        | pct($ma20; $latest.clpr)
        | roundn(4)
      ),
      ma5VsMA20Pct: (
        ([ $rows5[].clpr | select(. != null) ] | avg) as $ma5
        | ([ $rows20[].clpr | select(. != null) ] | avg) as $ma20
        | pct($ma20; $ma5)
        | roundn(4)
      ),
      dailyVolatility20Pct: ($dailyReturns20 | stddev | roundn(4)),
      maxDrawdown20Pct: ($rows20 | max_drawdown | roundn(4))
    };

def score_candidate:
  . as $candidate
  | (($candidate.valueRatio20 // 0) | clamp(0; 5)) as $valuePulse
  | (($candidate.high20PositionPct // 0) | clamp(0; 100)) as $highPosition
  | (($candidate.closeLocationPct // 0) | clamp(0; 100)) as $closeLocation
  | (
      if ($candidate.return1dPct // 0) > $overheat1dPct then
        (($candidate.return1dPct - $overheat1dPct) * $overheatPenaltyWeight)
      else
        0
      end
    ) as $oneDayOverheatPenalty
  | (
      if ($candidate.return5dPct // 0) > $overheat5dPct then
        (($candidate.return5dPct - $overheat5dPct) * $overheatPenaltyWeight)
      else
        0
      end
    ) as $fiveDayOverheatPenalty
  | (
      if ($candidate.highClosePullbackPct // 0) > $highClosePullbackFreePct then
        (($candidate.highClosePullbackPct - $highClosePullbackFreePct) * $highClosePullbackPenaltyWeight)
      else
        0
      end
    ) as $highClosePullbackPenalty
  | (
      (($candidate.return1dPct // 0) * 0.8)
      + (($candidate.return3dPct // 0) * 1.2)
      + (($candidate.return5dPct // 0) * 1.4)
      + (($candidate.return20dPct // 0) * 0.45)
      + (($candidate.closeVsMA5Pct // 0) * 0.8)
      + (($candidate.closeVsMA20Pct // 0) * 0.45)
      + (($candidate.ma5VsMA20Pct // 0) * 0.7)
      + ($valuePulse * 4)
      + ($highPosition * 0.12)
      + ($closeLocation * 0.06)
      + (if ($candidate.latestTrPrc // 0) >= ($minLatestTrPrc * 5) then 5 elif ($candidate.latestTrPrc // 0) >= ($minLatestTrPrc * 2) then 2 else 0 end)
      - (($candidate.dailyVolatility20Pct // 0) * 0.6)
      + (($candidate.maxDrawdown20Pct // 0) * 0.3)
      - $oneDayOverheatPenalty
      - $fiveDayOverheatPenalty
      - $highClosePullbackPenalty
    ) as $score
  | $candidate + {
      valuePulseScore: (($valuePulse * 4) | roundn(4)),
      oneDayOverheatPenalty: ($oneDayOverheatPenalty | roundn(4)),
      fiveDayOverheatPenalty: ($fiveDayOverheatPenalty | roundn(4)),
      highClosePullbackPenalty: ($highClosePullbackPenalty | roundn(4)),
      score: ($score | roundn(4))
    };

def filtered_rows:
  raw_etf_rows
  | map(select(not_excluded($excludeRegex)))
  | group_by(.srtnCd)
  | map(summarize_symbol)
  | map(select(
      (.observationDays >= $minDays)
      and ((.latestTrPrc // 0) >= $minLatestTrPrc)
      and ((.avg20TrPrc // 0) >= $minAvg20TrPrc)
      and ((.avgNPptTotAmt // 0) >= $minAvgNPptTotAmt)
      and (.return1dPct != null and .return1dPct >= $minReturn1dPct)
      and (.return1dPct <= $maxReturn1dPct)
      and (.return3dPct != null and .return3dPct >= $minReturn3dPct)
      and (.return5dPct != null and .return5dPct >= $minReturn5dPct)
      and (.return20dPct != null and .return20dPct >= $minReturn20dPct)
      and (.valueRatio20 != null and .valueRatio20 >= $minValueRatio20)
      and (.closeLocationPct != null and .closeLocationPct >= $minCloseLocationPct)
      and (.high20PositionPct != null and .high20PositionPct >= $minHigh20PositionPct)
      and (.closeVsMA5Pct != null and .closeVsMA5Pct >= $minCloseVsMA5Pct)
      and (.closeVsMA20Pct != null and .closeVsMA20Pct >= $minCloseVsMA20Pct)
      and (.dailyVolatility20Pct != null and .dailyVolatility20Pct <= $maxDailyVolatility20)
      and (.maxDrawdown20Pct != null and .maxDrawdown20Pct >= $maxDrawdown20Pct)
    ))
  | map(score_candidate)
  | sort_by(.score, .return5dPct, .valueRatio20, .latestTrPrc)
  | reverse;

def ranked_result:
  filtered_rows as $all
  | {
      candidateCount: ($all | length),
      rows: (
        $all[:$top]
        | to_entries
        | map(.value + {rank: (.key + 1)})
      )
    };

def metadata($result):
  {
    product: "etf",
    screen: "next_day_momentum",
    preset: $preset,
    generatedAt: (now | todateiso8601),
    filters: {
      minDays: $minDays,
      minLatestTrPrc: $minLatestTrPrc,
      minAvg20TrPrc: $minAvg20TrPrc,
      minAvgNPptTotAmt: $minAvgNPptTotAmt,
      minReturn1dPct: $minReturn1dPct,
      minReturn3dPct: $minReturn3dPct,
      minReturn5dPct: $minReturn5dPct,
      minReturn20dPct: $minReturn20dPct,
      maxReturn1dPct: $maxReturn1dPct,
      minValueRatio20: $minValueRatio20,
      minCloseLocationPct: $minCloseLocationPct,
      minHigh20PositionPct: $minHigh20PositionPct,
      minCloseVsMA5Pct: $minCloseVsMA5Pct,
      minCloseVsMA20Pct: $minCloseVsMA20Pct,
      maxDailyVolatility20: $maxDailyVolatility20,
      maxDrawdown20Pct: $maxDrawdown20Pct,
      overheat1dPct: $overheat1dPct,
      overheat5dPct: $overheat5dPct,
      overheatPenaltyWeight: $overheatPenaltyWeight,
      highClosePullbackFreePct: $highClosePullbackFreePct,
      highClosePullbackPenaltyWeight: $highClosePullbackPenaltyWeight,
      excludeRegex: $excludeRegex,
      top: $top
    },
    candidateCount: $result.candidateCount,
    rowCount: ($result.rows | length),
    rows: $result.rows
  };

def csv_rows($rows):
  (
    [
      "rank",
      "srtnCd",
      "isinCd",
      "itmsNm",
      "latestBasDt",
      "return1dPct",
      "return3dPct",
      "return5dPct",
      "return20dPct",
      "return60dPct",
      "latestFltRt",
      "valueRatio20",
      "volumeRatio20",
      "closeLocationPct",
      "closeFromHighPct",
      "highClosePullbackPct",
      "high20PositionPct",
      "gapTo20dHighPct",
      "closeVsMA5Pct",
      "closeVsMA20Pct",
      "ma5VsMA20Pct",
      "dailyVolatility20Pct",
      "maxDrawdown20Pct",
      "latestTrPrc",
      "avg20TrPrc",
      "avgNPptTotAmt",
      "latestClpr",
      "score",
      "highClosePullbackPenalty",
      "bssIdxIdxNm"
    ],
    (
      $rows[]
      | [
          .rank,
          .srtnCd,
          .isinCd,
          .itmsNm,
          .latestBasDt,
          .return1dPct,
          .return3dPct,
          .return5dPct,
          .return20dPct,
          .return60dPct,
          .latestFltRt,
          .valueRatio20,
          .volumeRatio20,
          .closeLocationPct,
          .closeFromHighPct,
          .highClosePullbackPct,
          .high20PositionPct,
          .gapTo20dHighPct,
          .closeVsMA5Pct,
          .closeVsMA20Pct,
          .ma5VsMA20Pct,
          .dailyVolatility20Pct,
          .maxDrawdown20Pct,
          .latestTrPrc,
          .avg20TrPrc,
          .avgNPptTotAmt,
          .latestClpr,
          .score,
          .highClosePullbackPenalty,
          .bssIdxIdxNm
        ]
    )
  )
  | @csv;

ranked_result as $result
| if $format == "csv" then
    csv_rows($result.rows)
  else
    metadata($result)
  end
