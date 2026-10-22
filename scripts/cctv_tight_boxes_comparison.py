#!/usr/bin/env python3
"""Reproducible single-camera NN box snap A/B on PETS manual person-body GT."""
import argparse
import json
import sys
from pathlib import Path

def report(rows):
    pairs={}
    for x in rows:
        filename=Path(x["report_file"].replace("\\","/")).name
        if not filename.startswith("v3v8-"):
            raise ValueError("mismatched detector binaries in A/B")
        scene=x["name"].replace("_tight_boxes","")
        variant="tight" if x["name"].endswith("_tight_boxes") else "normal"
        pairs.setdefault(scene,{})[variant]=x
    if len(pairs)!=2 or any(set(v)!={"tight","normal"} for v in pairs.values()):
        raise ValueError("expected two paired single-camera scenarios")
    lines=[
        "# Experimental tracker body-box geometry A/B on PETS2009",
        "",
        "Both configurations use the same offline Windows v3 YOLOv8n EXE, same footage, manual whole-body GT, timestamp replay and model weights.",
        "",
        "| Scenario | Standard P/R | Tight boxes P/R | Missed appearances normal / tight | ID switches normal / tight |",
        "|---|---:|---:|---:|---:|"
    ]
    improved=True
    for scene,pair in sorted(pairs.items()):
        a,b=pair["normal"],pair["tight"]
        def pr(item):
            t=item["tracking"]
            return f'{t["precision"]*100:.1f}% / {t["recall"]*100:.1f}%'
        if b["tracking"]["recall"] < a["tracking"]["recall"] or b["episodes_missed"]>a["episodes_missed"]:
            improved=False
        lines.append(f'| {scene} | {pr(a)} | {pr(b)} | {a["episodes_missed"]} / {b["episodes_missed"]} | {a["tracking"]["id_switches"]} / {b["tracking"]["id_switches"]} |')
    lines+=["", "**Experimental gate:** "+("PROVISIONAL PASS: still needs crowd/grid regression" if improved else "FAIL: do not ship this tracker change"),"",
            "Do not label as measured live RVi multi-monitor performance. No Windows 7 physical host tested."]
    return "\n".join(lines)

if __name__=="__main__":
    p=argparse.ArgumentParser()
    p.add_argument("scored",type=Path)
    p.add_argument("--out",type=Path,required=True)
    args=p.parse_args()
    text=report(json.loads(args.scored.read_text(encoding="utf-8-sig")))
    args.out.parent.mkdir(parents=True,exist_ok=True)
    args.out.write_text(text,encoding="utf-8")
    print(text.encode("ascii",errors="backslashreplace").decode("ascii"))
