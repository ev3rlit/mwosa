---
name: mwosa
description: Use when helping with installed mwosa CLI workflows, especially Datago ETF daily collection, canonical SQLite data, jq-based ETF screening, and saved screening strategies.
---

# mwosa

## First checks

- Prefer the installed `mwosa` CLI for user-facing commands.
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

Check `references/cli-command-help.md` before recommending jq commands. The installed `mwosa` CLI exposes saved strategy commands.

Saved strategy flow:

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

Do not promise runtime `--argjson` support for saved strategy execution; the captured CLI flags only expose `--alias` on `screen strategy`.

## Dataset schema and keys

`etf_daily_metrics` reads canonical ETF daily bars from SQLite. Despite the name, treat it as daily-bar records unless the captured CLI help documents derived metrics.

Canonical strategy input rows use these JSON keys:

| Key | Meaning | Type |
| --- | --- | --- |
| `provider` | provider id, usually `datago` | string |
| `provider_group` | provider group, usually `securitiesProductPrice` | string |
| `operation` | provider operation, e.g. `getETFPriceInfo` | string |
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

When writing jq for canonical SQLite screens, use canonical snake_case keys.

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

Save and run a query file:

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

## How to answer users

- If the user asks for a command, give the installed `mwosa` command first.
- If the user asks how jq screening works, explain canonical SQLite strategy screens through installed `create/screen strategy`.
- Keep Korean explanations concise and command-first.
- When producing candidate files for review, create both machine-friendly JSON and small human-openable CSV when the output could be large.
