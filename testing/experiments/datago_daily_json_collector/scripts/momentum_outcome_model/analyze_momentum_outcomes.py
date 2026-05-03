#!/usr/bin/env python3
from __future__ import annotations

import argparse
import csv
import json
from collections import defaultdict
from pathlib import Path
from typing import Any, Iterable


def repo_root() -> Path:
    return Path(__file__).resolve().parents[5]


def default_output_dir() -> Path:
    return repo_root() / "tmp/testing/datago-daily-json-collector/analysis"


NUMERIC_FIELDS = {
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
    "balancedScore",
    "aggressiveScore",
    "next1dReturnPct",
    "next3dCloseReturnPct",
    "next5dCloseReturnPct",
    "next3dHighReturnPct",
    "next3dLowReturnPct",
    "next5dHighReturnPct",
    "next5dLowReturnPct",
}

BOOL_FIELDS = {
    "balancedHit",
    "aggressiveHit",
    "overlapHit",
    "next1dUp",
    "next3dPlus3Hit",
    "next5dMinus3Hit",
    "next5dStop3BeforeTarget3",
    "next5dTarget3BeforeStop3",
}


def parse_value(field: str, value: str) -> Any:
    if value == "":
        return None
    if field in BOOL_FIELDS:
        return value.lower() == "true"
    if field in NUMERIC_FIELDS:
        return float(value)
    return value


def load_dataset(path: Path) -> list[dict[str, Any]]:
    with path.open("r", encoding="utf-8", newline="") as handle:
        reader = csv.DictReader(handle)
        return [{field: parse_value(field, value) for field, value in row.items()} for row in reader]


def mean(values: Iterable[float | None]) -> float | None:
    usable = [value for value in values if value is not None]
    if not usable:
        return None
    return sum(usable) / len(usable)


def rate(rows: list[dict[str, Any]], field: str) -> float | None:
    usable = [row for row in rows if row.get(field) is not None]
    if not usable:
        return None
    return sum(1 for row in usable if row[field]) / len(usable)


def roundn(value: float | None, places: int = 4) -> float | None:
    if value is None:
        return None
    return round(value, places)


def summarize(rows: list[dict[str, Any]]) -> dict[str, Any]:
    return {
        "count": len(rows),
        "next1dUpRate": roundn(rate(rows, "next1dUp")),
        "next3dPlus3Rate": roundn(rate(rows, "next3dPlus3Hit")),
        "next5dMinus3Rate": roundn(rate(rows, "next5dMinus3Hit")),
        "next5dStop3BeforeTarget3Rate": roundn(rate(rows, "next5dStop3BeforeTarget3")),
        "next5dTarget3BeforeStop3Rate": roundn(rate(rows, "next5dTarget3BeforeStop3")),
        "avgNext1dReturnPct": roundn(mean(row.get("next1dReturnPct") for row in rows), 4),
        "avgNext3dCloseReturnPct": roundn(mean(row.get("next3dCloseReturnPct") for row in rows), 4),
        "avgNext5dCloseReturnPct": roundn(mean(row.get("next5dCloseReturnPct") for row in rows), 4),
        "avgNext3dHighReturnPct": roundn(mean(row.get("next3dHighReturnPct") for row in rows), 4),
        "avgNext5dLowReturnPct": roundn(mean(row.get("next5dLowReturnPct") for row in rows), 4),
    }


def bucket_label(value: float | None, buckets: list[tuple[float | None, float | None, str]]) -> str:
    if value is None:
        return "missing"
    for low, high, label in buckets:
        if low is not None and value < low:
            continue
        if high is not None and value >= high:
            continue
        return label
    return "other"


def bucket_summary(rows: list[dict[str, Any]], field: str, buckets: list[tuple[float | None, float | None, str]]) -> list[dict[str, Any]]:
    grouped: dict[str, list[dict[str, Any]]] = defaultdict(list)
    order = [label for _, _, label in buckets] + ["missing", "other"]
    for row in rows:
        grouped[bucket_label(row.get(field), buckets)].append(row)
    return [
        {"bucket": label, **summarize(grouped[label])}
        for label in order
        if grouped.get(label)
    ]


def preset_group(row: dict[str, Any]) -> str:
    if row.get("balancedHit") and row.get("aggressiveHit"):
        return "both"
    if row.get("balancedHit"):
        return "balanced_only"
    if row.get("aggressiveHit"):
        return "aggressive_only"
    return "neither"


def group_summary(rows: list[dict[str, Any]], key_fn) -> list[dict[str, Any]]:
    grouped: dict[str, list[dict[str, Any]]] = defaultdict(list)
    for row in rows:
        grouped[key_fn(row)].append(row)
    order = ["both", "balanced_only", "aggressive_only", "neither"]
    labels = order + sorted(label for label in grouped if label not in order)
    return [{"group": label, **summarize(grouped[label])} for label in labels if grouped.get(label)]


def top_examples(rows: list[dict[str, Any]], predicate, sort_field: str, limit: int = 10) -> list[dict[str, Any]]:
    selected = [row for row in rows if predicate(row) and row.get(sort_field) is not None]
    selected.sort(key=lambda row: row[sort_field], reverse=True)
    fields = [
        "basDt",
        "srtnCd",
        "itmsNm",
        "balancedHit",
        "aggressiveHit",
        "balancedScore",
        "aggressiveScore",
        "return5dPct",
        "return20dPct",
        "valueRatio20",
        "highClosePullbackPct",
        "next1dReturnPct",
        "next3dHighReturnPct",
        "next5dLowReturnPct",
        "next5dFirstEvent3Pct",
    ]
    return [{field: row.get(field) for field in fields} for row in selected[:limit]]


def build_report(rows: list[dict[str, Any]]) -> dict[str, Any]:
    rows_with_next1 = [row for row in rows if row.get("next1dReturnPct") is not None]
    rows_with_next3 = [row for row in rows if row.get("next3dHighReturnPct") is not None]
    rows_with_next5 = [row for row in rows if row.get("next5dLowReturnPct") is not None]

    pullback_buckets = [
        (None, 1, "0_to_1_pct"),
        (1, 3, "1_to_3_pct"),
        (3, 5, "3_to_5_pct"),
        (5, None, "5_pct_plus"),
    ]
    value_ratio_buckets = [
        (None, 1, "under_1x"),
        (1, 1.5, "1_to_1_5x"),
        (1.5, 2, "1_5_to_2x"),
        (2, 3, "2_to_3x"),
        (3, None, "3x_plus"),
    ]

    preset_groups = group_summary(rows_with_next5, preset_group)
    report = {
        "model": "momentum_outcome_descriptive_v1",
        "notes": [
            "Labels use daily OHLC only.",
            "If target and stop are both touched on the same day, stop is treated as first for conservative scoring.",
            "This is a research validation report, not an execution-grade backtest.",
        ],
        "dataset": {
            "rowCount": len(rows),
            "next1EligibleRows": len(rows_with_next1),
            "next3EligibleRows": len(rows_with_next3),
            "next5EligibleRows": len(rows_with_next5),
            "minBasDt": min((row["basDt"] for row in rows), default=None),
            "maxBasDt": max((row["basDt"] for row in rows), default=None),
        },
        "questions": {
            "q1_next3d_plus3_probability": {
                "definition": "next3dHighReturnPct >= 3",
                "overall": summarize(rows_with_next3),
                "byPresetGroup": group_summary(rows_with_next3, preset_group),
            },
            "q2_next5d_minus3_risk": {
                "definition": "next5dLowReturnPct <= -3; first-event uses +3 target and -3 stop, stop first on same-day conflict",
                "overall": summarize(rows_with_next5),
                "byPresetGroup": preset_groups,
            },
            "q3_high_close_pullback_effect": {
                "definition": "highClosePullbackPct bucket vs next-day outcome",
                "buckets": bucket_summary(rows_with_next1, "highClosePullbackPct", pullback_buckets),
            },
            "q4_value_ratio_persistence": {
                "definition": "valueRatio20 bucket vs next-day and next-3-day outcome",
                "buckets": bucket_summary(rows_with_next3, "valueRatio20", value_ratio_buckets),
            },
            "q5_preset_overlap": {
                "definition": "balanced/aggressive hit groups vs future outcome",
                "groups": preset_groups,
            },
        },
        "examples": {
            "topBalancedHitsByScore": top_examples(rows_with_next5, lambda row: row.get("balancedHit"), "balancedScore"),
            "topAggressiveHitsByScore": top_examples(rows_with_next5, lambda row: row.get("aggressiveHit"), "aggressiveScore"),
            "largePullbackExamples": top_examples(rows_with_next5, lambda row: (row.get("highClosePullbackPct") or 0) >= 5, "highClosePullbackPct"),
        },
    }
    return report


def format_pct(value: float | None) -> str:
    if value is None:
        return "n/a"
    return f"{value * 100:.2f}%"


def format_num(value: float | None) -> str:
    if value is None:
        return "n/a"
    return f"{value:.4f}"


def write_markdown(report: dict[str, Any], output: Path) -> None:
    lines = [
        "# Momentum Outcome Report",
        "",
        "일봉 기반 ETF 모멘텀 스크리너의 과거 결과를 검증한 리포트입니다.",
        "",
        "## Dataset",
        "",
        f"- rows: `{report['dataset']['rowCount']}`",
        f"- next1 eligible rows: `{report['dataset']['next1EligibleRows']}`",
        f"- next3 eligible rows: `{report['dataset']['next3EligibleRows']}`",
        f"- next5 eligible rows: `{report['dataset']['next5EligibleRows']}`",
        f"- date range: `{report['dataset']['minBasDt']}` to `{report['dataset']['maxBasDt']}`",
        "",
        "## Q1. 다음 3거래일 안에 +3%를 찍을 확률",
        "",
    ]
    q1 = report["questions"]["q1_next3d_plus3_probability"]["overall"]
    lines.append(f"- overall: `{format_pct(q1['next3dPlus3Rate'])}`")
    lines.append("")
    lines.append("## Q2. 다음 5거래일 안에 -3% 위험")
    lines.append("")
    q2 = report["questions"]["q2_next5d_minus3_risk"]["overall"]
    lines.append(f"- next5d -3% touch rate: `{format_pct(q2['next5dMinus3Rate'])}`")
    lines.append(f"- stop before +3% target rate: `{format_pct(q2['next5dStop3BeforeTarget3Rate'])}`")
    lines.append("")
    lines.append("## Q3. 고가 대비 종가 이탈률")
    lines.append("")
    for row in report["questions"]["q3_high_close_pullback_effect"]["buckets"]:
        lines.append(f"- `{row['bucket']}`: count `{row['count']}`, next1 up `{format_pct(row['next1dUpRate'])}`, avg next1 `{format_num(row['avgNext1dReturnPct'])}%`")
    lines.append("")
    lines.append("## Q4. 거래대금 배율")
    lines.append("")
    for row in report["questions"]["q4_value_ratio_persistence"]["buckets"]:
        lines.append(f"- `{row['bucket']}`: count `{row['count']}`, next1 up `{format_pct(row['next1dUpRate'])}`, next3 +3 `{format_pct(row['next3dPlus3Rate'])}`")
    lines.append("")
    lines.append("## Q5. 프리셋 교집합")
    lines.append("")
    for row in report["questions"]["q5_preset_overlap"]["groups"]:
        lines.append(f"- `{row['group']}`: count `{row['count']}`, next3 +3 `{format_pct(row['next3dPlus3Rate'])}`, next5 -3 `{format_pct(row['next5dMinus3Rate'])}`, avg next5 `{format_num(row['avgNext5dCloseReturnPct'])}%`")
    lines.append("")
    lines.append("## Notes")
    lines.append("")
    for note in report["notes"]:
        lines.append(f"- {note}")
    lines.append("")
    output.write_text("\n".join(lines), encoding="utf-8")


def main() -> None:
    output_dir = default_output_dir()
    parser = argparse.ArgumentParser(description="Analyze ETF momentum feature/outcome dataset.")
    parser.add_argument("--dataset", type=Path, default=output_dir / "momentum-outcome-dataset.csv")
    parser.add_argument("--output-json", type=Path, default=output_dir / "momentum-outcome-report.json")
    parser.add_argument("--output-md", type=Path, default=output_dir / "momentum-outcome-report.md")
    args = parser.parse_args()

    rows = load_dataset(args.dataset)
    report = build_report(rows)
    args.output_json.parent.mkdir(parents=True, exist_ok=True)
    args.output_json.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    write_markdown(report, args.output_md)
    print(json.dumps({"dataset": str(args.dataset), "outputJson": str(args.output_json), "outputMarkdown": str(args.output_md)}, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
