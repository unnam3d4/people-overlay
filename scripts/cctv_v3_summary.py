#!/usr/bin/env python3
"""Summarize actual PETS2009 full-body GT replay. Do not infer real RVi monitor accuracy."""
import argparse
import json
from pathlib import Path

def pct(n):
    return "n/a" if n is None else f"{100*n:.1f}%"

def seconds(n):
    return "n/a" if n is None else f"{n/1000:.2f}s"

def make_report(rows):
    by={}
    for row in rows:
        name=Path(row["report_file"]).name
        label=name.split("-",1)[0]
        if label not in ("v19","v3"):
            raise ValueError("wrong version label "+label)
        by.setdefault(row["name"],{})[label]=row
    if len(by)!=4 or any(set(v)!={"v19","v3"} for v in by.values()):
        raise ValueError("required four paired real CCTV scenarios missing")
    lines=[
        "# People Overlay v3: PETS2009 real CCTV whole-person report",
        "",
        "**Status: evaluation, NOT field certification.** Source 795 frames of PETS2009 S2/L1 View_001 (~7fps) plus 4650 hand-labeled person-body boxes and IDs; replay resamples by repetition to 15fps. 4x4 screen mosaics replicate time-offset video, not 16 actual independent RVi streams.",
        "",
        "P=precision, R=recall, IoU≥0.5; all values count person-body boxes frame-by-frame.",
        "",
        "| Scenario | v1.9 P | v1.9 R | v3 P | v3 R | v1.9 false boxes | v3 false boxes | Missed arrivals v1.9→v3 | v3 first detection p95 |",
        "|---|---:|---:|---:|---:|---:|---:|---:|---:|"
    ]
    poor=False
    for scenario,pair in sorted(by.items()):
        old,new=pair["v19"],pair["v3"]
        a,b=old["tracking"],new["tracking"]
        recall0=a["recall"] or 0
        recall1=b["recall"] or 0
        if recall1<recall0-.03 or recall1<.5:
            poor=True
        lines.append(f'| {scenario} | {pct(a["precision"])} | {pct(a["recall"])} | {pct(b["precision"])} | {pct(b["recall"])} | {a["false_positive_boxes"]} | {b["false_positive_boxes"]} | {old["episodes_missed"]}→{new["episodes_missed"]} | {seconds(new["time_to_first_track_ms"]["p95"])} |')
    lines+=["",f'**Gate:** {"FAIL / quality below target" if poor else "PROVISIONAL / no major recall shortfall"}',
        "",
        "## Additional telemetry",
        "",
        "| Scenario | v3 TP | v3 FN | v3 ID switches | v3 AI inferences | v3 inference p95 |",
        "|---|---:|---:|---:|---:|---:|"
    ]
    for scenario,pair in sorted(by.items()):
        n=pair["v3"]
        lines.append(f'| {scenario} | {n["tracking"]["true_positive_boxes"]} | {n["tracking"]["false_negative_boxes"]} | {n["tracking"]["id_switches"]} | {n["inference_count"]} | {seconds(n["stage_times"]["infer_ms"]["p95"])} |')
    lines+=["",
        "## Non-covered conditions (critical)",
        "",
        "- This tests real filmed people on a Windows 2022 CI runner through each EXE's **offline --benchmark path**, not through v3's interactive v3Analyze multi-monitor capture engine.",
        "- No proof for native Windows 7/10 RVi rendering, BitBlt GPU overlays, actual independent monitor synchronization or detection at 48 feeds.",
        "- CPU load and latency in CI are not measurements from an old i3/i5. P95 time-to-first-track excludes totally missed appearances; missed arrivals shown separately.",
        "- No model retraining occurred; this is an independent testing suite, not training footage.",
        "- The v3 candidate must not be claimed ready for operational surveillance solely from this report.",
        ""
    ]
    return "\n".join(lines)

if __name__=="__main__":
    p=argparse.ArgumentParser()
    p.add_argument("comparison",type=Path)
    p.add_argument("--out",type=Path,required=True)
    args=p.parse_args()
    report=make_report(json.loads(args.comparison.read_text(encoding="utf-8-sig")))
    args.out.parent.mkdir(parents=True,exist_ok=True)
    args.out.write_text(report,encoding="utf-8")
    import sys
    # Windows GitHub runners often default to CP1252; non-ASCII report glyphs
    # must never turn a completed real-video benchmark into an apparent failure.
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    print(report)
