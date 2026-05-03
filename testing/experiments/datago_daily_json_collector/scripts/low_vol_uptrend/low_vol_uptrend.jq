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

def pct($start; $end):
  if $start == null or $start == 0 or $end == null then
    null
  else
    (($end / $start) - 1) * 100
  end;

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

def weekly_returns($window):
  . as $rows
  | [
      range($window; ($rows | length)) as $index
      | pct($rows[$index - $window].clpr; $rows[$index].clpr)
      | select(. != null)
    ];

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
  ((.itmsNm + " " + .bssIdxIdxNm) | test($regex; "i") | not);

def summarize_symbol:
  sort_by(.basDt) as $rows
  | ($rows | length) as $rowCount
  | ($rows[0]) as $first
  | ($rows[-1]) as $latest
  | ($threeMonthWindow | tonumber) as $threeMonthWindowNumber
  | ($weeklyWindow | tonumber) as $weeklyWindowNumber
  | ($rows | weekly_returns($weeklyWindowNumber)) as $weeklyReturns
  | ($weeklyReturns | map(select(. > 0)) | length) as $positiveWeeks
  | {
      srtnCd: $latest.srtnCd,
      isinCd: $latest.isinCd,
      itmsNm: $latest.itmsNm,
      bssIdxIdxNm: $latest.bssIdxIdxNm,
      firstBasDt: $first.basDt,
      latestBasDt: $latest.basDt,
      observationDays: $rowCount,
      latestClpr: $latest.clpr,
      latestNav: $latest.nav,
      latestTrPrc: $latest.trPrc,
      latestTrqu: $latest.trqu,
      latestFltRt: $latest.fltRt,
      return1yPct: (pct($first.clpr; $latest.clpr) | roundn(4)),
      return3mPct: (
        if $rowCount > $threeMonthWindowNumber then
          pct($rows[$rowCount - 1 - $threeMonthWindowNumber].clpr; $latest.clpr)
        else
          null
        end
        | roundn(4)
      ),
      return1mPct: (
        if $rowCount > $oneMonthWindow then
          pct($rows[$rowCount - 1 - $oneMonthWindow].clpr; $latest.clpr)
        else
          null
        end
        | roundn(4)
      ),
      weeklyReturnAvgPct: ($weeklyReturns | avg | roundn(4)),
      weeklyVolatilityPct: ($weeklyReturns | stddev | roundn(4)),
      positiveWeekRatio: (
        if ($weeklyReturns | length) == 0 then
          null
        else
          ($positiveWeeks / ($weeklyReturns | length))
        end
        | roundn(4)
      ),
      maxDrawdownPct: ($rows | max_drawdown | roundn(4)),
      avgTrPrc: ([ $rows[].trPrc | select(. != null) ] | avg | roundn(0)),
      avgNPptTotAmt: ([ $rows[].nPptTotAmt | select(. != null) ] | avg | roundn(0)),
      avgMrktTotAmt: ([ $rows[].mrktTotAmt | select(. != null) ] | avg | roundn(0)),
      weeklyReturnCount: ($weeklyReturns | length)
    };

def score_candidate:
  . as $candidate
  | (
      if ($candidate.return1mPct // 0) > $recentSurgeThresholdPct then
        (($candidate.return1mPct - $recentSurgeThresholdPct) * $recentSurgePenaltyWeight)
      else
        0
      end
    ) as $recentSurgePenalty
  | (
      (($candidate.return1yPct // 0) * 0.25)
      + (($candidate.return3mPct // 0) * 0.35)
      + (($candidate.positiveWeekRatio // 0) * 25)
      - (($candidate.weeklyVolatilityPct // 0) * 2.5)
      + (($candidate.maxDrawdownPct // 0) * 0.8)
      + (if ($candidate.avgTrPrc // 0) >= ($minAvgTrPrc * 5) then 5 elif ($candidate.avgTrPrc // 0) >= ($minAvgTrPrc * 2) then 2 else 0 end)
      + (if ($candidate.avgNPptTotAmt // 0) >= ($minAvgNPptTotAmt * 5) then 5 elif ($candidate.avgNPptTotAmt // 0) >= ($minAvgNPptTotAmt * 2) then 2 else 0 end)
      - $recentSurgePenalty
    ) as $score
  | $candidate + {
      recentSurgePenalty: ($recentSurgePenalty | roundn(4)),
      score: ($score | roundn(4))
    };

def result_rows:
  raw_etf_rows
  | map(select(not_excluded($excludeRegex)))
  | group_by(.srtnCd)
  | map(summarize_symbol)
  | map(select(
      (.observationDays >= $minDays)
      and (.return1yPct != null and .return1yPct > 0)
      and (.return3mPct != null and .return3mPct > 0)
      and (.weeklyVolatilityPct != null)
      and (.weeklyVolatilityPct <= $maxWeeklyVolatility)
      and (.maxDrawdownPct != null)
      and (.maxDrawdownPct >= $maxDrawdownPct)
      and (.positiveWeekRatio != null and .positiveWeekRatio >= $minPositiveWeekRatio)
      and ((.avgTrPrc // 0) >= $minAvgTrPrc)
      and ((.avgNPptTotAmt // 0) >= $minAvgNPptTotAmt)
    ))
  | map(score_candidate)
  | sort_by(.score, .return3mPct, .return1yPct)
  | reverse
  | .[:$top]
  | to_entries
  | map(.value + {rank: (.key + 1)});

def metadata($rows):
  {
    product: "etf",
    screen: "low_volatility_uptrend",
    generatedAt: (now | todateiso8601),
    filters: {
      minDays: $minDays,
      minAvgTrPrc: $minAvgTrPrc,
      minAvgNPptTotAmt: $minAvgNPptTotAmt,
      maxWeeklyVolatility: $maxWeeklyVolatility,
      maxDrawdownPct: $maxDrawdownPct,
      minPositiveWeekRatio: $minPositiveWeekRatio,
      oneMonthWindowTradingDays: $oneMonthWindow,
      recentSurgeThresholdPct: $recentSurgeThresholdPct,
      recentSurgePenaltyWeight: $recentSurgePenaltyWeight,
      excludeRegex: $excludeRegex,
      weeklyWindowTradingDays: $weeklyWindow,
      threeMonthWindowTradingDays: $threeMonthWindow,
      top: $top
    },
    rowCount: ($rows | length),
    rows: $rows
  };

def csv_rows($rows):
  (
    [
      "rank",
      "srtnCd",
      "isinCd",
      "itmsNm",
      "latestBasDt",
      "return1yPct",
      "return3mPct",
      "return1mPct",
      "weeklyReturnAvgPct",
      "weeklyVolatilityPct",
      "positiveWeekRatio",
      "maxDrawdownPct",
      "avgTrPrc",
      "avgNPptTotAmt",
      "latestClpr",
      "recentSurgePenalty",
      "score",
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
          .return1yPct,
          .return3mPct,
          .return1mPct,
          .weeklyReturnAvgPct,
          .weeklyVolatilityPct,
          .positiveWeekRatio,
          .maxDrawdownPct,
          .avgTrPrc,
          .avgNPptTotAmt,
          .latestClpr,
          .recentSurgePenalty,
          .score,
          .bssIdxIdxNm
        ]
    )
  )
  | @csv;

result_rows as $rows
| if $format == "csv" then
    csv_rows($rows)
  else
    metadata($rows)
  end
