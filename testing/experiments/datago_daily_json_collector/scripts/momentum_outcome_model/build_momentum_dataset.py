#!/usr/bin/env python3
from __future__ import annotations

import argparse
import csv
import json
import math
import re
from collections import defaultdict
from dataclasses import dataclass
from pathlib import Path
from typing import Any


def repo_root() -> Path:
    return Path(__file__).resolve().parents[5]


def default_raw_dir() -> Path:
    return repo_root() / "tmp/testing/datago-daily-json-collector/raw"


def default_output_dir() -> Path:
    return repo_root() / "tmp/testing/datago-daily-json-collector/analysis"


def num(value: Any) -> float | None:
    if value is None or value == "":
        return None
    try:
        return float(str(value).replace(",", ""))
    except ValueError:
        return None


def roundn(value: float | None, places: int = 4) -> float | None:
    if value is None or not math.isfinite(value):
        return None
    return round(value, places)


def avg(values: list[float | None]) -> float | None:
    usable = [value for value in values if value is not None]
    if not usable:
        return None
    return sum(usable) / len(usable)


def stddev(values: list[float | None]) -> float | None:
    usable = [value for value in values if value is not None]
    if len(usable) <= 1:
        return None
    mean = sum(usable) / len(usable)
    return math.sqrt(sum((value - mean) ** 2 for value in usable) / (len(usable) - 1))


def pct(start: float | None, end: float | None) -> float | None:
    if start is None or end is None or start == 0:
        return None
    return ((end / start) - 1) * 100


def clamp(value: float | None, low: float, high: float) -> float:
    if value is None:
        return low
    return min(max(value, low), high)


def max_drawdown(rows: list[dict[str, Any]]) -> float | None:
    peak: float | None = None
    max_drawdown_pct = 0.0
    has_price = False
    for row in rows:
        close = row.get("clpr")
        if close is None:
            continue
        has_price = True
        if peak is None or close > peak:
            peak = close
            continue
        drawdown = pct(peak, close)
        if drawdown is not None and drawdown < max_drawdown_pct:
            max_drawdown_pct = drawdown
    if not has_price:
        return None
    return max_drawdown_pct


def daily_returns(rows: list[dict[str, Any]]) -> list[float]:
    returns: list[float] = []
    for index in range(1, len(rows)):
        value = pct(rows[index - 1].get("clpr"), rows[index].get("clpr"))
        if value is not None:
            returns.append(value)
    return returns


@dataclass(frozen=True)
class Preset:
    min_days: int
    min_latest_tr_prc: float
    min_avg20_tr_prc: float
    min_avg_nppt_tot_amt: float
    min_return1d_pct: float
    min_return3d_pct: float
    min_return5d_pct: float
    min_return20d_pct: float
    max_return1d_pct: float
    min_value_ratio20: float
    min_close_location_pct: float
    min_high20_position_pct: float
    min_close_vs_ma5_pct: float
    min_close_vs_ma20_pct: float
    max_daily_volatility20: float
    max_drawdown20_pct: float
    overheat1d_pct: float
    overheat5d_pct: float
    overheat_penalty_weight: float
    high_close_pullback_free_pct: float
    high_close_pullback_penalty_weight: float
    exclude_regex: str


PRESETS = {
    "balanced": Preset(
        min_days=60,
        min_latest_tr_prc=500_000_000,
        min_avg20_tr_prc=200_000_000,
        min_avg_nppt_tot_amt=10_000_000_000,
        min_return1d_pct=0,
        min_return3d_pct=0.5,
        min_return5d_pct=1.5,
        min_return20d_pct=2,
        max_return1d_pct=12,
        min_value_ratio20=1.3,
        min_close_location_pct=55,
        min_high20_position_pct=70,
        min_close_vs_ma5_pct=-0.5,
        min_close_vs_ma20_pct=0,
        max_daily_volatility20=8,
        max_drawdown20_pct=-18,
        overheat1d_pct=8,
        overheat5d_pct=18,
        overheat_penalty_weight=0.8,
        high_close_pullback_free_pct=2,
        high_close_pullback_penalty_weight=1.2,
        exclude_regex="레버리지|인버스|곱버스|2X|2x|3X|3x",
    ),
    "aggressive": Preset(
        min_days=40,
        min_latest_tr_prc=300_000_000,
        min_avg20_tr_prc=100_000_000,
        min_avg_nppt_tot_amt=5_000_000_000,
        min_return1d_pct=-1,
        min_return3d_pct=0,
        min_return5d_pct=1,
        min_return20d_pct=0,
        max_return1d_pct=20,
        min_value_ratio20=1.1,
        min_close_location_pct=45,
        min_high20_position_pct=60,
        min_close_vs_ma5_pct=-1.5,
        min_close_vs_ma20_pct=-1,
        max_daily_volatility20=15,
        max_drawdown20_pct=-30,
        overheat1d_pct=12,
        overheat5d_pct=30,
        overheat_penalty_weight=0.4,
        high_close_pullback_free_pct=4,
        high_close_pullback_penalty_weight=0.7,
        exclude_regex="인버스|곱버스",
    ),
}


FIELDNAMES = [
    "basDt",
    "srtnCd",
    "isinCd",
    "itmsNm",
    "bssIdxIdxNm",
    "observationDays",
    "latestClpr",
    "latestMkp",
    "latestHipr",
    "latestLopr",
    "latestNav",
    "latestTrPrc",
    "latestTrqu",
    "latestFltRt",
    "return1dPct",
    "return3dPct",
    "return5dPct",
    "return20dPct",
    "return60dPct",
    "ma5",
    "ma20",
    "avg20TrPrc",
    "avg20Trqu",
    "avgNPptTotAmt",
    "avgMrktTotAmt",
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
    "balancedHit",
    "aggressiveHit",
    "overlapHit",
    "balancedScore",
    "aggressiveScore",
    "next1BasDt",
    "next1dReturnPct",
    "next3dCloseReturnPct",
    "next5dCloseReturnPct",
    "next3dHighReturnPct",
    "next3dLowReturnPct",
    "next5dHighReturnPct",
    "next5dLowReturnPct",
    "next1dUp",
    "next3dPlus3Hit",
    "next5dMinus3Hit",
    "next5dStop3BeforeTarget3",
    "next5dTarget3BeforeStop3",
    "next5dFirstEvent3Pct",
]

COMPACT_FIELDNAMES = [
    "basDt",
    "srtnCd",
    "itmsNm",
    "balancedHit",
    "aggressiveHit",
    "overlapHit",
    "balancedScore",
    "aggressiveScore",
    "latestClpr",
    "return1dPct",
    "return3dPct",
    "return5dPct",
    "return20dPct",
    "valueRatio20",
    "closeLocationPct",
    "highClosePullbackPct",
    "high20PositionPct",
    "dailyVolatility20Pct",
    "maxDrawdown20Pct",
    "latestTrPrc",
    "avg20TrPrc",
    "next1BasDt",
    "next1dReturnPct",
    "next3dHighReturnPct",
    "next5dLowReturnPct",
    "next3dPlus3Hit",
    "next5dMinus3Hit",
    "next5dFirstEvent3Pct",
]


def read_rows(raw_dir: Path, product: str) -> dict[str, list[dict[str, Any]]]:
    by_symbol: dict[str, list[dict[str, Any]]] = defaultdict(list)
    files = sorted(raw_dir.rglob("*.json"))
    if not files:
        raise FileNotFoundError(f"no .json snapshots found under {raw_dir}")

    for path in files:
        with path.open("r", encoding="utf-8") as handle:
            snapshot = json.load(handle)
        snapshot_bas_dt = str(snapshot.get("basDt") or "")
        for product_block in snapshot.get("products", []):
            if product_block.get("product") != product:
                continue
            for item in product_block.get("items", []):
                close = num(item.get("clpr"))
                code = str(item.get("srtnCd") or "")
                bas_dt = str(item.get("basDt") or snapshot_bas_dt)
                if not code or not bas_dt or close is None:
                    continue
                by_symbol[code].append(
                    {
                        "basDt": bas_dt,
                        "srtnCd": code,
                        "isinCd": str(item.get("isinCd") or ""),
                        "itmsNm": str(item.get("itmsNm") or ""),
                        "bssIdxIdxNm": str(item.get("bssIdxIdxNm") or ""),
                        "clpr": close,
                        "mkp": num(item.get("mkp")),
                        "hipr": num(item.get("hipr")),
                        "lopr": num(item.get("lopr")),
                        "trPrc": num(item.get("trPrc")),
                        "trqu": num(item.get("trqu")),
                        "nPptTotAmt": num(item.get("nPptTotAmt")),
                        "mrktTotAmt": num(item.get("mrktTotAmt")),
                        "nav": num(item.get("nav")),
                        "fltRt": num(item.get("fltRt")),
                    }
                )

    for rows in by_symbol.values():
        rows.sort(key=lambda row: row["basDt"])
    return by_symbol


def return_at(rows: list[dict[str, Any]], index: int, window: int) -> float | None:
    if index < window:
        return None
    return pct(rows[index - window].get("clpr"), rows[index].get("clpr"))


def build_features(rows: list[dict[str, Any]], index: int) -> dict[str, Any]:
    current = rows[index]
    history = rows[: index + 1]
    rows5 = history[-5:]
    rows20 = history[-20:]
    daily_returns20 = daily_returns(rows20)
    high20 = max((row.get("hipr") if row.get("hipr") is not None else row.get("clpr")) for row in rows20)
    low20 = min((row.get("lopr") if row.get("lopr") is not None else row.get("clpr")) for row in rows20)
    ma5 = avg([row.get("clpr") for row in rows5])
    ma20 = avg([row.get("clpr") for row in rows20])
    avg20_tr_prc = avg([row.get("trPrc") for row in rows20])
    avg20_trqu = avg([row.get("trqu") for row in rows20])
    close = current.get("clpr")
    high = current.get("hipr")
    low = current.get("lopr")

    return {
        "basDt": current["basDt"],
        "srtnCd": current["srtnCd"],
        "isinCd": current["isinCd"],
        "itmsNm": current["itmsNm"],
        "bssIdxIdxNm": current["bssIdxIdxNm"],
        "observationDays": index + 1,
        "latestClpr": roundn(close, 4),
        "latestMkp": roundn(current.get("mkp"), 4),
        "latestHipr": roundn(high, 4),
        "latestLopr": roundn(low, 4),
        "latestNav": roundn(current.get("nav"), 4),
        "latestTrPrc": roundn(current.get("trPrc"), 0),
        "latestTrqu": roundn(current.get("trqu"), 0),
        "latestFltRt": roundn(current.get("fltRt"), 4),
        "return1dPct": roundn(return_at(rows, index, 1), 4),
        "return3dPct": roundn(return_at(rows, index, 3), 4),
        "return5dPct": roundn(return_at(rows, index, 5), 4),
        "return20dPct": roundn(return_at(rows, index, 20), 4),
        "return60dPct": roundn(return_at(rows, index, 60), 4),
        "ma5": roundn(ma5, 4),
        "ma20": roundn(ma20, 4),
        "avg20TrPrc": roundn(avg20_tr_prc, 0),
        "avg20Trqu": roundn(avg20_trqu, 0),
        "avgNPptTotAmt": roundn(avg([row.get("nPptTotAmt") for row in history]), 0),
        "avgMrktTotAmt": roundn(avg([row.get("mrktTotAmt") for row in history]), 0),
        "valueRatio20": roundn(current.get("trPrc") / avg20_tr_prc if current.get("trPrc") is not None and avg20_tr_prc else None, 4),
        "volumeRatio20": roundn(current.get("trqu") / avg20_trqu if current.get("trqu") is not None and avg20_trqu else None, 4),
        "closeLocationPct": roundn(((close - low) / (high - low) * 100) if close is not None and high is not None and low is not None and high != low else None, 4),
        "closeFromHighPct": roundn(pct(high, close), 4),
        "highClosePullbackPct": roundn(((1 - (close / high)) * 100) if close is not None and high else None, 4),
        "high20PositionPct": roundn(((close - low20) / (high20 - low20) * 100) if close is not None and high20 != low20 else None, 4),
        "gapTo20dHighPct": roundn(pct(high20, close), 4),
        "closeVsMA5Pct": roundn(pct(ma5, close), 4),
        "closeVsMA20Pct": roundn(pct(ma20, close), 4),
        "ma5VsMA20Pct": roundn(pct(ma20, ma5), 4),
        "dailyVolatility20Pct": roundn(stddev(daily_returns20), 4),
        "maxDrawdown20Pct": roundn(max_drawdown(rows20), 4),
    }


def preset_score(row: dict[str, Any], preset: Preset) -> float:
    value_pulse = clamp(row.get("valueRatio20"), 0, 5)
    high_position = clamp(row.get("high20PositionPct"), 0, 100)
    close_location = clamp(row.get("closeLocationPct"), 0, 100)
    return1d = row.get("return1dPct") or 0
    return5d = row.get("return5dPct") or 0
    one_day_overheat_penalty = max(return1d - preset.overheat1d_pct, 0) * preset.overheat_penalty_weight
    five_day_overheat_penalty = max(return5d - preset.overheat5d_pct, 0) * preset.overheat_penalty_weight
    high_close_pullback_penalty = (
        max((row.get("highClosePullbackPct") or 0) - preset.high_close_pullback_free_pct, 0)
        * preset.high_close_pullback_penalty_weight
    )
    liquidity_bonus = 5 if (row.get("latestTrPrc") or 0) >= preset.min_latest_tr_prc * 5 else 2 if (row.get("latestTrPrc") or 0) >= preset.min_latest_tr_prc * 2 else 0
    score = (
        (row.get("return1dPct") or 0) * 0.8
        + (row.get("return3dPct") or 0) * 1.2
        + (row.get("return5dPct") or 0) * 1.4
        + (row.get("return20dPct") or 0) * 0.45
        + (row.get("closeVsMA5Pct") or 0) * 0.8
        + (row.get("closeVsMA20Pct") or 0) * 0.45
        + (row.get("ma5VsMA20Pct") or 0) * 0.7
        + value_pulse * 4
        + high_position * 0.12
        + close_location * 0.06
        + liquidity_bonus
        - (row.get("dailyVolatility20Pct") or 0) * 0.6
        + (row.get("maxDrawdown20Pct") or 0) * 0.3
        - one_day_overheat_penalty
        - five_day_overheat_penalty
        - high_close_pullback_penalty
    )
    return score


def preset_hit(row: dict[str, Any], preset: Preset) -> bool:
    name = f"{row.get('itmsNm') or ''} {row.get('bssIdxIdxNm') or ''}"
    if preset.exclude_regex and re.search(preset.exclude_regex, name, flags=re.IGNORECASE):
        return False

    checks = [
        (row.get("observationDays") or 0) >= preset.min_days,
        (row.get("latestTrPrc") or 0) >= preset.min_latest_tr_prc,
        (row.get("avg20TrPrc") or 0) >= preset.min_avg20_tr_prc,
        (row.get("avgNPptTotAmt") or 0) >= preset.min_avg_nppt_tot_amt,
        row.get("return1dPct") is not None and row["return1dPct"] >= preset.min_return1d_pct,
        row.get("return1dPct") is not None and row["return1dPct"] <= preset.max_return1d_pct,
        row.get("return3dPct") is not None and row["return3dPct"] >= preset.min_return3d_pct,
        row.get("return5dPct") is not None and row["return5dPct"] >= preset.min_return5d_pct,
        row.get("return20dPct") is not None and row["return20dPct"] >= preset.min_return20d_pct,
        row.get("valueRatio20") is not None and row["valueRatio20"] >= preset.min_value_ratio20,
        row.get("closeLocationPct") is not None and row["closeLocationPct"] >= preset.min_close_location_pct,
        row.get("high20PositionPct") is not None and row["high20PositionPct"] >= preset.min_high20_position_pct,
        row.get("closeVsMA5Pct") is not None and row["closeVsMA5Pct"] >= preset.min_close_vs_ma5_pct,
        row.get("closeVsMA20Pct") is not None and row["closeVsMA20Pct"] >= preset.min_close_vs_ma20_pct,
        row.get("dailyVolatility20Pct") is not None and row["dailyVolatility20Pct"] <= preset.max_daily_volatility20,
        row.get("maxDrawdown20Pct") is not None and row["maxDrawdown20Pct"] >= preset.max_drawdown20_pct,
    ]
    return all(checks)


def first_event(entry: float, future_rows: list[dict[str, Any]], target_pct: float, stop_pct: float) -> str | None:
    target = entry * (1 + target_pct / 100)
    stop = entry * (1 + stop_pct / 100)
    for row in future_rows:
        high = row.get("hipr") if row.get("hipr") is not None else row.get("clpr")
        low = row.get("lopr") if row.get("lopr") is not None else row.get("clpr")
        stop_hit = low is not None and low <= stop
        target_hit = high is not None and high >= target
        if stop_hit and target_hit:
            return "stop"
        if stop_hit:
            return "stop"
        if target_hit:
            return "target"
    return None


def build_outcomes(rows: list[dict[str, Any]], index: int) -> dict[str, Any]:
    current = rows[index]
    entry = current.get("clpr")
    future5 = rows[index + 1 : index + 6]
    future3 = rows[index + 1 : index + 4]
    next1 = future5[0] if len(future5) >= 1 else None
    event5 = first_event(entry, future5, target_pct=3, stop_pct=-3) if entry is not None and len(future5) >= 5 else None

    high3 = max((row.get("hipr") if row.get("hipr") is not None else row.get("clpr")) for row in future3) if len(future3) >= 3 else None
    low3 = min((row.get("lopr") if row.get("lopr") is not None else row.get("clpr")) for row in future3) if len(future3) >= 3 else None
    high5 = max((row.get("hipr") if row.get("hipr") is not None else row.get("clpr")) for row in future5) if len(future5) >= 5 else None
    low5 = min((row.get("lopr") if row.get("lopr") is not None else row.get("clpr")) for row in future5) if len(future5) >= 5 else None
    next1_return = pct(entry, next1.get("clpr")) if next1 is not None else None
    next3_high_return = pct(entry, high3)
    next5_low_return = pct(entry, low5)

    return {
        "next1BasDt": next1.get("basDt") if next1 is not None else None,
        "next1dReturnPct": roundn(next1_return, 4),
        "next3dCloseReturnPct": roundn(pct(entry, rows[index + 3].get("clpr")) if len(future3) >= 3 else None, 4),
        "next5dCloseReturnPct": roundn(pct(entry, rows[index + 5].get("clpr")) if len(future5) >= 5 else None, 4),
        "next3dHighReturnPct": roundn(next3_high_return, 4),
        "next3dLowReturnPct": roundn(pct(entry, low3), 4),
        "next5dHighReturnPct": roundn(pct(entry, high5), 4),
        "next5dLowReturnPct": roundn(next5_low_return, 4),
        "next1dUp": next1_return is not None and next1_return > 0,
        "next3dPlus3Hit": next3_high_return is not None and next3_high_return >= 3,
        "next5dMinus3Hit": next5_low_return is not None and next5_low_return <= -3,
        "next5dStop3BeforeTarget3": event5 == "stop",
        "next5dTarget3BeforeStop3": event5 == "target",
        "next5dFirstEvent3Pct": event5 or "none" if len(future5) >= 5 else None,
    }


def serialize(value: Any) -> Any:
    if value is None:
        return ""
    if isinstance(value, bool):
        return "true" if value else "false"
    return value


def build_dataset(raw_dir: Path, output: Path, product: str, min_history: int, hits_only: bool, compact: bool) -> dict[str, Any]:
    by_symbol = read_rows(raw_dir, product)
    output.parent.mkdir(parents=True, exist_ok=True)
    fieldnames = COMPACT_FIELDNAMES if compact else FIELDNAMES
    row_count = 0
    skipped_row_count = 0
    symbol_count = 0
    min_bas_dt: str | None = None
    max_bas_dt: str | None = None

    with output.open("w", encoding="utf-8", newline="") as handle:
        writer = csv.DictWriter(handle, fieldnames=fieldnames)
        writer.writeheader()
        for rows in by_symbol.values():
            if len(rows) < min_history:
                continue
            symbol_count += 1
            for index in range(min_history - 1, len(rows)):
                feature_row = build_features(rows, index)
                balanced_hit = preset_hit(feature_row, PRESETS["balanced"])
                aggressive_hit = preset_hit(feature_row, PRESETS["aggressive"])
                feature_row["balancedHit"] = balanced_hit
                feature_row["aggressiveHit"] = aggressive_hit
                feature_row["overlapHit"] = balanced_hit and aggressive_hit
                feature_row["balancedScore"] = roundn(preset_score(feature_row, PRESETS["balanced"]), 4)
                feature_row["aggressiveScore"] = roundn(preset_score(feature_row, PRESETS["aggressive"]), 4)
                feature_row.update(build_outcomes(rows, index))
                if hits_only and not (balanced_hit or aggressive_hit):
                    skipped_row_count += 1
                    continue
                writer.writerow({field: serialize(feature_row.get(field)) for field in fieldnames})
                row_count += 1
                bas_dt = feature_row["basDt"]
                min_bas_dt = bas_dt if min_bas_dt is None else min(min_bas_dt, bas_dt)
                max_bas_dt = bas_dt if max_bas_dt is None else max(max_bas_dt, bas_dt)

    return {
        "output": str(output),
        "product": product,
        "minHistory": min_history,
        "hitsOnly": hits_only,
        "compact": compact,
        "symbolCount": symbol_count,
        "rowCount": row_count,
        "skippedRowCount": skipped_row_count,
        "columnCount": len(fieldnames),
        "minBasDt": min_bas_dt,
        "maxBasDt": max_bas_dt,
    }


def main() -> None:
    parser = argparse.ArgumentParser(description="Build ETF momentum feature/outcome dataset from Datago daily JSON snapshots.")
    parser.add_argument("--raw-dir", type=Path, default=default_raw_dir())
    parser.add_argument("--output", type=Path, default=default_output_dir() / "momentum-outcome-dataset.csv")
    parser.add_argument("--product", default="etf")
    parser.add_argument("--min-history", type=int, default=60)
    parser.add_argument("--hits-only", action="store_true", help="Only write rows that hit balanced or aggressive momentum preset.")
    parser.add_argument("--compact", action="store_true", help="Write a smaller, spreadsheet-friendly column subset.")
    args = parser.parse_args()

    summary = build_dataset(args.raw_dir, args.output, args.product, args.min_history, args.hits_only, args.compact)
    print(json.dumps(summary, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
