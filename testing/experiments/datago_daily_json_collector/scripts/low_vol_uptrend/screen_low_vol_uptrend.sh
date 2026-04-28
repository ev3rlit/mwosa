#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/../../../.." && pwd)"

raw_dir="$repo_root/tmp/testing/datago-daily-json-collector/raw"
output_dir="$repo_root/tmp/testing/datago-daily-json-collector/analysis"
format="json"
output_path=""
top=50
min_days=120
min_avg_tr_prc=100000000
min_avg_nppt_tot_amt=10000000000
max_weekly_volatility=3
max_drawdown_pct=-15
min_positive_week_ratio=0.55
one_month_window=20
recent_surge_threshold_pct=10
recent_surge_penalty_weight=1.5
weekly_window=5
three_month_window=60
exclude_regex="레버리지|인버스|곱버스|2X|2x|3X|3x"

usage() {
  cat <<'USAGE'
Usage:
  screen_low_vol_uptrend.sh [options]

Options:
  --raw-dir PATH                 Raw snapshot directory.
  --output-dir PATH              Directory for generated result files.
  --output PATH                  Exact output file path.
  --format json|csv              Output format. Default: json.
  --top N                        Number of candidates. Default: 50.
  --min-days N                   Minimum observed ETF rows. Default: 120.
  --min-avg-tr-prc N             Minimum average traded amount. Default: 100000000.
  --min-avg-nppt-tot-amt N       Minimum average net asset value. Default: 10000000000.
  --max-weekly-volatility N      Maximum weekly return volatility pct. Default: 3.
  --max-drawdown-pct N           Maximum drawdown threshold pct. Default: -15.
  --min-positive-week-ratio N    Minimum positive weekly return ratio. Default: 0.55.
  --one-month-window N           Trading-day window for 1M surge penalty. Default: 20.
  --recent-surge-threshold-pct N  1M return threshold before penalty. Default: 10.
  --recent-surge-penalty-weight N Penalty per pct above threshold. Default: 1.5.
  --weekly-window N              Trading-day window for weekly returns. Default: 5.
  --three-month-window N         Trading-day window for 3M returns. Default: 60.
  --exclude-regex REGEX          ETF name/index exclusion regex.
  -h, --help                     Show this help.
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
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
    --min-avg-tr-prc)
      min_avg_tr_prc="$2"
      shift 2
      ;;
    --min-avg-nppt-tot-amt)
      min_avg_nppt_tot_amt="$2"
      shift 2
      ;;
    --max-weekly-volatility)
      max_weekly_volatility="$2"
      shift 2
      ;;
    --max-drawdown-pct)
      max_drawdown_pct="$2"
      shift 2
      ;;
    --min-positive-week-ratio)
      min_positive_week_ratio="$2"
      shift 2
      ;;
    --one-month-window)
      one_month_window="$2"
      shift 2
      ;;
    --recent-surge-threshold-pct)
      recent_surge_threshold_pct="$2"
      shift 2
      ;;
    --recent-surge-penalty-weight)
      recent_surge_penalty_weight="$2"
      shift 2
      ;;
    --weekly-window)
      weekly_window="$2"
      shift 2
      ;;
    --three-month-window)
      three_month_window="$2"
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
  output_path="$output_dir/low-vol-uptrend-etf-candidates.$format"
fi

jq_flags=(-s)
if [[ "$format" == "csv" ]]; then
  jq_flags=(-r -s)
fi

jq "${jq_flags[@]}" \
  --arg format "$format" \
  --arg excludeRegex "$exclude_regex" \
  --argjson top "$top" \
  --argjson minDays "$min_days" \
  --argjson minAvgTrPrc "$min_avg_tr_prc" \
  --argjson minAvgNPptTotAmt "$min_avg_nppt_tot_amt" \
  --argjson maxWeeklyVolatility "$max_weekly_volatility" \
  --argjson maxDrawdownPct "$max_drawdown_pct" \
  --argjson minPositiveWeekRatio "$min_positive_week_ratio" \
  --argjson oneMonthWindow "$one_month_window" \
  --argjson recentSurgeThresholdPct "$recent_surge_threshold_pct" \
  --argjson recentSurgePenaltyWeight "$recent_surge_penalty_weight" \
  --argjson weeklyWindow "$weekly_window" \
  --argjson threeMonthWindow "$three_month_window" \
  -f "$script_dir/low_vol_uptrend.jq" \
  "${files[@]}" \
  > "$output_path"

printf 'wrote %s\n' "$output_path"
