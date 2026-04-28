#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/../../../../.." && pwd)"

raw_dir="$repo_root/tmp/testing/datago-daily-json-collector/raw"
output_dir="$repo_root/tmp/testing/datago-daily-json-collector/analysis"
format="json"
output_path=""
preset="balanced"
top=50
min_days=60
min_latest_tr_prc=500000000
min_avg20_tr_prc=200000000
min_avg_nppt_tot_amt=10000000000
min_return1d_pct=0
min_return3d_pct=0.5
min_return5d_pct=1.5
min_return20d_pct=2
max_return1d_pct=12
min_value_ratio20=1.3
min_close_location_pct=55
min_high20_position_pct=70
min_close_vs_ma5_pct=-0.5
min_close_vs_ma20_pct=0
max_daily_volatility20=8
max_drawdown20_pct=-18
overheat1d_pct=8
overheat5d_pct=18
overheat_penalty_weight=0.8
high_close_pullback_free_pct=2
high_close_pullback_penalty_weight=1.2
exclude_regex="레버리지|인버스|곱버스|2X|2x|3X|3x"

apply_balanced_preset() {
  min_days=60
  min_latest_tr_prc=500000000
  min_avg20_tr_prc=200000000
  min_avg_nppt_tot_amt=10000000000
  min_return1d_pct=0
  min_return3d_pct=0.5
  min_return5d_pct=1.5
  min_return20d_pct=2
  max_return1d_pct=12
  min_value_ratio20=1.3
  min_close_location_pct=55
  min_high20_position_pct=70
  min_close_vs_ma5_pct=-0.5
  min_close_vs_ma20_pct=0
  max_daily_volatility20=8
  max_drawdown20_pct=-18
  overheat1d_pct=8
  overheat5d_pct=18
  overheat_penalty_weight=0.8
  high_close_pullback_free_pct=2
  high_close_pullback_penalty_weight=1.2
  exclude_regex="레버리지|인버스|곱버스|2X|2x|3X|3x"
}

apply_aggressive_preset() {
  min_days=40
  min_latest_tr_prc=300000000
  min_avg20_tr_prc=100000000
  min_avg_nppt_tot_amt=5000000000
  min_return1d_pct=-1
  min_return3d_pct=0
  min_return5d_pct=1
  min_return20d_pct=0
  max_return1d_pct=20
  min_value_ratio20=1.1
  min_close_location_pct=45
  min_high20_position_pct=60
  min_close_vs_ma5_pct=-1.5
  min_close_vs_ma20_pct=-1
  max_daily_volatility20=15
  max_drawdown20_pct=-30
  overheat1d_pct=12
  overheat5d_pct=30
  overheat_penalty_weight=0.4
  high_close_pullback_free_pct=4
  high_close_pullback_penalty_weight=0.7
  exclude_regex="인버스|곱버스"
}

apply_preset() {
  case "$1" in
    balanced)
      apply_balanced_preset
      ;;
    aggressive)
      apply_aggressive_preset
      ;;
    *)
      echo "--preset must be balanced or aggressive: $1" >&2
      exit 2
      ;;
  esac
}

usage() {
  cat <<'USAGE'
Usage:
  screen_next_day_momentum.sh [options]

Options:
  --preset balanced|aggressive    Screening preset. Default: balanced.
  --raw-dir PATH                  Raw snapshot directory.
  --output-dir PATH               Directory for generated result files.
  --output PATH                   Exact output file path.
  --format json|csv               Output format. Default: json.
  --top N                         Number of rows to output. Default: 50.
  --min-days N                    Minimum observed ETF rows. Default: 60.
  --min-latest-tr-prc N           Minimum latest traded amount. Default: 500000000.
  --min-avg20-tr-prc N            Minimum 20-day average traded amount. Default: 200000000.
  --min-avg-nppt-tot-amt N        Minimum average net asset value. Default: 10000000000.
  --min-return1d-pct N            Minimum 1 trading-day return pct. Default: 0.
  --min-return3d-pct N            Minimum 3 trading-day return pct. Default: 0.5.
  --min-return5d-pct N            Minimum 5 trading-day return pct. Default: 1.5.
  --min-return20d-pct N           Minimum 20 trading-day return pct. Default: 2.
  --max-return1d-pct N            Maximum 1 trading-day return pct. Default: 12.
  --min-value-ratio20 N           Minimum latest traded amount / 20-day average. Default: 1.3.
  --min-close-location-pct N      Minimum latest close position inside daily range. Default: 55.
  --min-high20-position-pct N     Minimum latest close position inside 20-day range. Default: 70.
  --min-close-vs-ma5-pct N        Minimum latest close vs 5-day moving average pct. Default: -0.5.
  --min-close-vs-ma20-pct N       Minimum latest close vs 20-day moving average pct. Default: 0.
  --max-daily-volatility20 N      Maximum 20-day daily return volatility pct. Default: 8.
  --max-drawdown20-pct N          Maximum 20-day drawdown threshold pct. Default: -18.
  --overheat1d-pct N              1-day return threshold before penalty. Default: 8.
  --overheat5d-pct N              5-day return threshold before penalty. Default: 18.
  --overheat-penalty-weight N     Penalty per pct above overheat threshold. Default: 0.8.
  --high-close-pullback-free-pct N High-to-close pullback pct allowed before penalty. Default: 2.
  --high-close-pullback-penalty-weight N Penalty per pct above pullback allowance. Default: 1.2.
  --exclude-regex REGEX           ETF name/index exclusion regex. Empty string disables exclusion.
  -h, --help                      Show this help.
USAGE
}

args=("$@")
for ((i = 0; i < ${#args[@]}; i++)); do
  case "${args[$i]}" in
    --preset)
      if (( i + 1 >= ${#args[@]} )); then
        echo "--preset requires a value" >&2
        exit 2
      fi
      preset="${args[$((i + 1))]}"
      ;;
    --preset=*)
      preset="${args[$i]#*=}"
      ;;
  esac
done

apply_preset "$preset"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --preset)
      preset="$2"
      shift 2
      ;;
    --preset=*)
      preset="${1#*=}"
      shift
      ;;
    --raw-dir)
      raw_dir="$2"
      shift 2
      ;;
    --output-dir)
      output_dir="$2"
      shift 2
      ;;
    --output)
      output_path="$2"
      shift 2
      ;;
    --format)
      format="$2"
      shift 2
      ;;
    --top)
      top="$2"
      shift 2
      ;;
    --min-days)
      min_days="$2"
      shift 2
      ;;
    --min-latest-tr-prc)
      min_latest_tr_prc="$2"
      shift 2
      ;;
    --min-avg20-tr-prc)
      min_avg20_tr_prc="$2"
      shift 2
      ;;
    --min-avg-nppt-tot-amt)
      min_avg_nppt_tot_amt="$2"
      shift 2
      ;;
    --min-return1d-pct)
      min_return1d_pct="$2"
      shift 2
      ;;
    --min-return3d-pct)
      min_return3d_pct="$2"
      shift 2
      ;;
    --min-return5d-pct)
      min_return5d_pct="$2"
      shift 2
      ;;
    --min-return20d-pct)
      min_return20d_pct="$2"
      shift 2
      ;;
    --max-return1d-pct)
      max_return1d_pct="$2"
      shift 2
      ;;
    --min-value-ratio20)
      min_value_ratio20="$2"
      shift 2
      ;;
    --min-close-location-pct)
      min_close_location_pct="$2"
      shift 2
      ;;
    --min-high20-position-pct)
      min_high20_position_pct="$2"
      shift 2
      ;;
    --min-close-vs-ma5-pct)
      min_close_vs_ma5_pct="$2"
      shift 2
      ;;
    --min-close-vs-ma20-pct)
      min_close_vs_ma20_pct="$2"
      shift 2
      ;;
    --max-daily-volatility20)
      max_daily_volatility20="$2"
      shift 2
      ;;
    --max-drawdown20-pct)
      max_drawdown20_pct="$2"
      shift 2
      ;;
    --overheat1d-pct)
      overheat1d_pct="$2"
      shift 2
      ;;
    --overheat5d-pct)
      overheat5d_pct="$2"
      shift 2
      ;;
    --overheat-penalty-weight)
      overheat_penalty_weight="$2"
      shift 2
      ;;
    --high-close-pullback-free-pct)
      high_close_pullback_free_pct="$2"
      shift 2
      ;;
    --high-close-pullback-penalty-weight)
      high_close_pullback_penalty_weight="$2"
      shift 2
      ;;
    --exclude-regex)
      exclude_regex="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

case "$format" in
  json|csv)
    ;;
  *)
    echo "--format must be json or csv: $format" >&2
    exit 2
    ;;
esac

if [[ ! -d "$raw_dir" ]]; then
  echo "raw dir does not exist: $raw_dir" >&2
  exit 1
fi

files=()
while IFS= read -r path; do
  files+=("$path")
done < <(find "$raw_dir" -type f -name '*.json' | sort)

if [[ ${#files[@]} -eq 0 ]]; then
  echo "no .json snapshots found under: $raw_dir" >&2
  exit 1
fi

mkdir -p "$output_dir"
if [[ -z "$output_path" ]]; then
  output_path="$output_dir/next-day-momentum-$preset-etf-candidates.$format"
fi

jq_flags=(-s)
if [[ "$format" == "csv" ]]; then
  jq_flags=(-r -s)
fi

jq "${jq_flags[@]}" \
  --arg format "$format" \
  --arg preset "$preset" \
  --arg excludeRegex "$exclude_regex" \
  --argjson top "$top" \
  --argjson minDays "$min_days" \
  --argjson minLatestTrPrc "$min_latest_tr_prc" \
  --argjson minAvg20TrPrc "$min_avg20_tr_prc" \
  --argjson minAvgNPptTotAmt "$min_avg_nppt_tot_amt" \
  --argjson minReturn1dPct "$min_return1d_pct" \
  --argjson minReturn3dPct "$min_return3d_pct" \
  --argjson minReturn5dPct "$min_return5d_pct" \
  --argjson minReturn20dPct "$min_return20d_pct" \
  --argjson maxReturn1dPct "$max_return1d_pct" \
  --argjson minValueRatio20 "$min_value_ratio20" \
  --argjson minCloseLocationPct "$min_close_location_pct" \
  --argjson minHigh20PositionPct "$min_high20_position_pct" \
  --argjson minCloseVsMA5Pct "$min_close_vs_ma5_pct" \
  --argjson minCloseVsMA20Pct "$min_close_vs_ma20_pct" \
  --argjson maxDailyVolatility20 "$max_daily_volatility20" \
  --argjson maxDrawdown20Pct "$max_drawdown20_pct" \
  --argjson overheat1dPct "$overheat1d_pct" \
  --argjson overheat5dPct "$overheat5d_pct" \
  --argjson overheatPenaltyWeight "$overheat_penalty_weight" \
  --argjson highClosePullbackFreePct "$high_close_pullback_free_pct" \
  --argjson highClosePullbackPenaltyWeight "$high_close_pullback_penalty_weight" \
  -f "$script_dir/next_day_momentum.jq" \
  "${files[@]}" \
  > "$output_path"

printf 'wrote %s\n' "$output_path"
