#!/usr/bin/env python3
"""Same filmed PETS2009 body bounding boxes, one unchanged v3 Go application,
two different NCNN models: YOLO26n versus YOLOv8n.
Do not treat this offline replay as proof of real RVi capture or Windows 7.
"""
import argparse
import json
import sys
from pathlib import Path

def percentage(x):
    return "n/a" if x is None else f"{100*x:.1f}%"

def report(data):
    pairs={}
    for row in data:
        label=Path(row["report_file"].replace("\\","/")).name.split("-",1)[0]
        if label not in ("v3","v3v8"):continue
        pairs.setdefault(row["name"],{})[label]=row
    if len(pairs)!=4 or any(set(x)!={"v3","v3v8"} for x in pairs.values()):
        raise ValueError("Expected four paired real CCTV model results")
    lines=[
      "# PETS2009 YOLO26n vs YOLOv8n on identical v3 Go application",
      "",
      "Real filmed PETS CCTV frames; body IoU>=0.5, repeated 7fps -> 15fps replay, 4x4 mosaics constructed from time-offset segments of a single source camera.",
      "Both EXEs use the **same v3 source code and inference harness**, only neural model artifacts differ.",
      "",
      "| Scene | YOLO26n recall | YOLOv8n recall | YOLO26n precision | YOLOv8n precision | arrivals missed 26n | arrivals missed 8n | inference p95 26n / 8n |",
      "|---|---:|---:|---:|---:|---:|---:|---:|"
    ]
    winners={"v3":0,"v3v8":0}
    for scenario,row in sorted(pairs.items()):
        a,b=row["v3"],row["v3v8"]
        aa,bb=a["tracking"],b["tracking"]
        if aa["recall"] is not None and bb["recall"] is not None:
            if aa["recall"]>bb["recall"]:winners["v3"]+=1
            if bb["recall"]>aa["recall"]:winners["v3v8"]+=1
        ai=a["stage_times"]["infer_ms"]["p95"]
        bi=b["stage_times"]["infer_ms"]["p95"]
        lines.append(
            f'| {scenario} | {percentage(aa["recall"])} | {percentage(bb["recall"])} | '
            f'{percentage(aa["precision"])} | {percentage(bb["precision"])} | '
            f'{a["episodes_missed"]} | {b["episodes_missed"]} | {ai:.1f} / {bi:.1f} ms |'
        )
    lines += ["",f'**Scenario wins on recall:** YOLO26n = {winners["v3"]}; YOLOv8n = {winners["v3v8"]}.',
        "",
        "**Do not auto-promote a model based on one percentage alone.** Inspect missed whole-person arrivals, false boxes, delay, and variability across repeats.",
        "Data is offline pixel replay. No physical multi-display RVi test. Neither model is retrained by this test.",
        ""
    ]
    return "\n".join(lines)

if __name__=="__main__":
    p=argparse.ArgumentParser()
    p.add_argument("data",type=Path)
    p.add_argument("--out",type=Path,required=True)
    args=p.parse_args()
    output=report(json.loads(args.data.read_text(encoding="utf-8-sig")))
    args.out.parent.mkdir(parents=True,exist_ok=True)
    args.out.write_text(output,encoding="utf-8")
    if hasattr(sys.stdout,"reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8",errors="replace")
    print(output)
