#!/usr/bin/env python3
"""Evidence-based A/B of two neural scheduling geometries on identical PETS2009 body labels.

Both runs use identical current v3 Windows EXE, YOLOv8n weights, video and ground truth.
Only crop grouping changes. Never equate this with actual RVi on physical monitors.
"""
import json
import argparse
from pathlib import Path

def fmt(x):
    return "n/a" if x is None else f"{x*100:.1f}%"

def report(rows):
    pairs={}
    for x in rows:
        filename=Path(x["report_file"].replace("\\","/")).name
        if not filename.startswith("v3v8-"):
            raise ValueError("unexpected EXE in A/B report")
        scene=x["name"].replace("_vertical_pairs","")
        variant="paired" if "_vertical_pairs" in x["name"] else "standard"
        pairs.setdefault(scene,{})[variant]=x
    if len(pairs)!=2 or any(set(v)!={"standard","paired"} for v in pairs.values()):
        raise ValueError("Require two paired PETS scenarios: four-feed and sixteen-feed")
    lines=["# Paired CCTV vertical crops — same YOLOv8n Windows v3 EXE",
           "",
           "Two repeated camera rows per AI call, eight paired ROIs vs sixteen single-tile ROIs.",
           "7-second clips of genuine PETS2009 footage, hand-labelled whole bodies; 4x4 composites are synthetic mosaics of time-offset clips.",
           "All numbers are frame-weighted IoU>=0.5 unless noted.",
           "",
           "| 4x4 feed count | P original | P paired | Recall original | Recall paired | Missed appearances original / paired | First-detect p95 original / paired | Inference counts original / paired |",
           "|---|---:|---:|---:|---:|---:|---:|---:|"]
    pass_quality=True
    for scene,items in sorted(pairs.items()):
        a,b=items["standard"],items["paired"]
        at,bt=a["tracking"],b["tracking"]
        if bt["recall"] is None or at["recall"] is None:
            raise ValueError("missing person-body recall")
        if bt["recall"]<at["recall"] or b["episodes_missed"]>a["episodes_missed"]:
            pass_quality=False
        ap95=a["time_to_first_track_ms"]["p95"]
        bp95=b["time_to_first_track_ms"]["p95"]
        n="16" if "sixteen" in scene else "4"
        lines.append(f'| {n} | {fmt(at["precision"])} | {fmt(bt["precision"])} | {fmt(at["recall"])} | {fmt(bt["recall"])} | {a["episodes_missed"]} / {b["episodes_missed"]} | {ap95} / {bp95} ms | {a["inference_count"]} / {b["inference_count"]} |')
    lines+=["", "**Promotion gate:** "+("PASS on these clips; independent repeats still necessary" if pass_quality else "FAIL: paired grouping hurts at least one scenario"),"",
    "This is research-only geometry; no automatic production activation. The 2-second 48-camera SLA remains unproven. CPU timings are Windows CI machine, not weak target i3/i5."]
    return "\n".join(lines),pass_quality

if __name__=="__main__":
    p=argparse.ArgumentParser()
    p.add_argument("result",type=Path)
    p.add_argument("--out",type=Path,required=True)
    a=p.parse_args()
    content,passed=report(json.loads(a.result.read_text(encoding="utf-8-sig")))
    a.out.parent.mkdir(parents=True,exist_ok=True)
    a.out.write_text(content,encoding="utf-8")
    print(content.encode("ascii",errors="backslashreplace").decode("ascii"))
    # Never exit with failure for a genuine model disadvantage:
    # preserve all raw evidence and let reviewers see a negative result.
