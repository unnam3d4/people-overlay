#!/usr/bin/env python3
"""Release gate, not a vanity CI green check: strict person-body requirements.

Read only ground-truth-scored real CCTV reports, never face landmarks or
model selftest counts. Missed appearances are counted explicitly instead of
only taking percentiles over successfully found people.
"""
import argparse
import json
import math
from pathlib import Path

REQUIRED = {
    "pets2009_single_less_crowded_15fps":(.80,.80),
    "pets2009_single_crowded_15fps":(.75,.75),
    "pets2009_grid4_four_feeds_15fps":(.75,.70),
    "pets2009_grid4_sixteen_feeds_15fps":(.65,.60),
}

def inspect(rows,label="v3v8"):
    found={}
    for r in rows:
        name=Path(r.get("report_file","").replace("\\","/")).name
        if name.startswith(label+"-"):
            if r["name"] in found:raise ValueError("Duplicate scenario: "+r["name"])
            found[r["name"]]=r
    issues=[]
    if set(found)!=set(REQUIRED):
        issues.append("Required four grounded CCTV scenarios missing or extra: "+str(sorted(set(REQUIRED)-set(found))))
    for scenario,(min_precision,min_recall) in REQUIRED.items():
        x=found.get(scenario)
        if x is None:continue
        t=x["tracking"]
        precision=t["precision"] or 0.
        recall=t["recall"] or 0.
        if precision<min_precision:
            issues.append(f"{scenario}: precision {precision:.1%} < {min_precision:.0%}")
        if recall<min_recall:
            issues.append(f"{scenario}: recall {recall:.1%} < {min_recall:.0%}")
        missed=x["episodes_missed"]
        total=x["episodes_total"]
        if total<=0 or missed>math.ceil(total*.05):
            issues.append(f"{scenario}: missing {missed}/{total} arrivals exceeds 5% allowance")
        p95=x["time_to_first_track_ms"]["p95"]
        if p95 is None or p95>2000:
            issues.append(f"{scenario}: first-person p95 {p95}ms violates <=2000ms")
    return issues

if __name__=="__main__":
    p=argparse.ArgumentParser()
    p.add_argument("scores",type=Path)
    p.add_argument("--model",default="v3v8")
    args=p.parse_args()
    failure=inspect(json.loads(args.scores.read_text(encoding="utf-8-sig")),args.model)
    if failure:
        print("QUALITY GATE FAILED - DO NOT RELEASE AS STABLE:")
        for item in failure:print(" -",item)
        raise SystemExit(1)
    print("QUALITY GATE PASSED on supplied four body-GT replay scenarios")
