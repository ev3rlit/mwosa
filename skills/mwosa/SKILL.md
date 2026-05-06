---
name: mwosa
description: Use when working in the mwosa repo or explaining mwosa CLI workflows, especially Datago ETF daily collection, canonical SQLite data, jq-based ETF screening, raw Datago JSON experiments, and saved screening strategies.
---

# mwosa

## First checks

- Work from the repository root.
- Before code edits, read `RULE.md` and check `git status --short --branch`.
- Prefer the installed `mwosa` CLI for user-facing commands. Use `go run` only when testing local source changes or running experiment-only tools that are not installed as CLI commands.
- When you need the complete installed command surface, read `references/cli-command-help.md`; it is generated from `mwosa --help` plus subcommand help.
- Keep stdout machine-readable for `json`, `ndjson`, `csv`, and `jq` pipelines. Put progress, diagnostics, and explanations on stderr or in chat.

## ETF daily collection

Use canonical SQLite collection when the user asks to collect ETF data for analysis:

```bash
mwosa backfill daily \
  --provider datago \
  --security-type etf \
  --from YYYY-MM-DD \
  --to YYYY-MM-DD \
  --workers 4 \
  -o json
```

For one trading date, set `--from` and `--to` to the same date. If `datago` is already the configured/default provider, `--provider datago` can be omitted, but keep it in handoff commands when clarity matters.

Verify a known ETF after collection:

```bash
mwosa get daily 069500 \
  --security-type etf \
  --from YYYY-MM-DD \
  --to YYYY-MM-DD \
  -o json
```

If provider auth is suspect, inspect or validate the provider without printing secrets:

```bash
mwosa provider doctor datago -o json
```

## Current jq screening surface

Check `references/cli-command-help.md` before recommending jq commands. The installed `mwosa v0.1.0` exposes saved strategy commands, while newer source checkouts may also expose one-off `mwosa screen etfs --jq/--jq-file`.

Installed `v0.1.0` strategy flow:

```bash
mwosa create strategy etf-weekly-leaders \
  --engine jq \
  --input etf_daily_metrics \
  --jq-file strategies/etf-weekly-leaders.jq \
  -o json

mwosa screen strategy etf-weekly-leaders \
  --alias YYYY-MM-DD-weekly-leaders \
  -o json

mwosa history screen -o table
mwosa inspect screen YYYY-MM-DD-weekly-leaders -o json
```

Do not promise runtime `--argjson` support for saved strategy execution unless the codebase has been checked again; the current CLI flags only expose `--alias` on `screen strategy`.

## Dataset schema and keys

`etf_daily_metrics` currently reads canonical ETF daily bars from SQLite. Despite the name, treat it as daily-bar records unless the codebase has added derived metrics.

Canonical strategy input rows use these JSON keys:

| Key | Meaning | Type |
| --- | --- | --- |
| `provider` | provider id, usually `datago` | string |
| `provider_group` | provider group, usually `securitiesProductPrice` | string |
| `operation` | source operation, e.g. `getETFPriceInfo` | string |
| `market` | market id, usually `krx` | string |
| `security_type` | `etf`, `etn`, or `elw` | string |
| `trading_date` | trading date, `YYYY-MM-DD` | string |
| `symbol` | KRX short code such as `069500` | string |
| `isin` | ISIN code | string |
| `name` | item name | string |
| `currency` | currency, usually `KRW` | string |
| `opening_price` | open price | numeric string |
| `highest_price` | high price | numeric string |
| `lowest_price` | low price | numeric string |
| `closing_price` | close price | numeric string |
| `price_change_from_previous_close` | absolute change from previous close | numeric string |
| `price_change_rate_from_previous_close` | percent change from previous close | numeric string |
| `traded_volume` | traded volume | numeric string |
| `traded_amount` | traded value/amount | numeric string |
| `market_capitalization` | market capitalization | numeric string |
| `extensions` | provider-specific extra fields | object of strings |

Convert numeric strings inside jq with `tonumber? // 0`.

Common Datago ETF extras are stored under `extensions` after canonical storage:

| Extension key | Meaning |
| --- | --- |
| `nPptTotAmt` | ETF net asset total amount |
| `stLstgCnt` | ETF listed share count |
| `nav` | ETF NAV |
| `bssIdxIdxNm` | underlying index name |
| `bssIdxClpr` | underlying index close |

For raw Datago JSON files, use provider-native keys instead of canonical keys:

| Raw key | Canonical key |
| --- | --- |
| `basDt` | `trading_date` |
| `srtnCd` | `symbol` |
| `isinCd` | `isin` |
| `itmsNm` | `name` |
| `mkp` | `opening_price` |
| `hipr` | `highest_price` |
| `lopr` | `lowest_price` |
| `clpr` | `closing_price` |
| `vs` | `price_change_from_previous_close` |
| `fltRt` | `price_change_rate_from_previous_close` |
| `trqu` | `traded_volume` |
| `trPrc` | `traded_amount` |
| `mrktTotAmt` | `market_capitalization` |

When writing jq for canonical SQLite screens, use canonical snake_case keys. Use raw camel-case Datago keys only inside `testing/experiments/datago_daily_json_collector` scripts or raw snapshot files.

## Practical jq examples

Latest-date liquidity leaders query:

```jq
def n: tonumber? // 0;
group_by(.symbol)
| map(max_by(.trading_date))
| sort_by((.traded_amount | n))
| reverse
| .[:50]
```

Save and run a query file on installed `v0.1.0`:

```bash
mwosa create strategy latest-liquidity-leaders \
  --engine jq \
  --input etf_daily_metrics \
  --jq-file strategies/latest-liquidity-leaders.jq \
  -o json

mwosa screen strategy latest-liquidity-leaders \
  --alias YYYY-MM-DD-liquidity-leaders \
  -o json
```

Period return leaders query from stored daily bars:

```jq
def n: tonumber? // 0;
map(select(.trading_date >= "YYYY-MM-DD" and .trading_date <= "YYYY-MM-DD"))
| group_by(.symbol)
| map(
    sort_by(.trading_date) as $rows
    | ($rows[0].closing_price | n) as $start
    | ($rows[-1].closing_price | n) as $end
    | select(length >= 2 and $start > 0 and $end > 0)
    | {
        symbol: $rows[-1].symbol,
        name: $rows[-1].name,
        from: $rows[0].trading_date,
        to: $rows[-1].trading_date,
        return_pct: (($end / $start - 1) * 100),
        traded_amount: ($rows[-1].traded_amount | n)
      }
  )
| sort_by(.return_pct)
| reverse
| .[:50]
```

For CSV output from one-off screens, prefer `-o csv` only when rows are flat. If payloads are nested, use `-o json | jq -r ...` to choose exact columns.

## Raw Datago JSON experiment scripts

Use the raw collector only when the user specifically wants date-partitioned source JSON or the existing experiment screeners. It writes one JSON file per date under `tmp/testing/datago-daily-json-collector/raw`.

```bash
DATAGO_SERVICE_KEY="$DATAGO_SERVICE_KEY" \
go run ./testing/experiments/datago_daily_json_collector \
  --products etf \
  --start-date YYYY-MM-DD \
  --end-date YYYY-MM-DD \
  --workers 2 \
  --overwrite=true
```

Run the momentum screener:

```bash
testing/experiments/datago_daily_json_collector/scripts/next_day_momentum/screen_next_day_momentum.sh \
  --raw-dir tmp/testing/datago-daily-json-collector/raw \
  --preset balanced \
  --format csv
```

Run the low-volatility uptrend screener:

```bash
testing/experiments/datago_daily_json_collector/scripts/low_vol_uptrend/screen_low_vol_uptrend.sh \
  --raw-dir tmp/testing/datago-daily-json-collector/raw \
  --format csv
```

Always pass `--raw-dir tmp/testing/datago-daily-json-collector/raw` explicitly for these scripts. When results look stale or empty, inspect the target date file and rerun with overwrite; old empty date files have caused misleading screens before.

## How to answer users

- If the user asks for a command, give the installed `mwosa` command first.
- If the user asks how jq screening works, explain the two layers: canonical SQLite strategy screens through installed `create/screen strategy`, raw JSON experiments through `testing/experiments/.../scripts`.
- Keep Korean explanations concise and command-first.
- When producing candidate files for review, create both machine-friendly JSON and small human-openable CSV when the output could be large.
