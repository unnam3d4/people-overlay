"""Fail a release-candidate replay when it demonstrably worsens baseline tracking.

Input: compare JSON produced by evaluate_pixel_benchmark.py (exactly one
v19 and v20 result per scenario). Report is synthetically generated and not
a stand-in for field performance. All raw metrics remain in the artifact.
"""
import argparse
import json
from pathlib import Path


def compare(rows, tolerance=0.05):
    matched={}
    for row in rows:
        filename=Path(row["report_file"].replace("\\","/")).name
        label=filename.split("-", 1)[0]
        if label not in ("v19","v20"): continue
        key=row["name"]
        matched.setdefault(key,{})[label]=row
    failures=[]
    for scenario,pair in sorted(matched.items()):
        if set(pair)!=set(("v19","v20")):
            failures.append(f"{scenario}: baseline or candidate missing")
            continue
        old,new=pair["v19"],pair["v20"]
        a,b=old["tracking"],new["tracking"]
        if b["recall"] is None or a["recall"] is None:
            failures.append(f"{scenario}: no measurable recall")
        elif b["recall"]<a["recall"]-tolerance:
            failures.append(f"{scenario}: recall {a['recall']:.3f} -> {b['recall']:.3f}, drop exceeds {tolerance:.1%}")
        if new["episodes_missed"]>old["episodes_missed"]+1:
            failures.append(f"{scenario}: missed appearances {old['episodes_missed']} -> {new['episodes_missed']}")
        if b["id_switches"]>a["id_switches"]+2:
            failures.append(f"{scenario}: ID switches {a['id_switches']} -> {b['id_switches']}")
    if not matched:
        failures.append("no baseline/candidate pairs in comparison report")
    return len(matched), failures


if __name__=="__main__":
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument("comparison", type=Path)
    p.add_argument("--max-recall-drop",type=float,default=.05)
    a=p.parse_args()
    tested,failed=compare(json.loads(a.comparison.read_text(encoding="utf-8-sig")),a.max_recall_drop)
    print(f"Paired photographic-synthetic CCTV scenarios checked: {tested}")
    if failed:
        print("TRACKING REGRESSIONS:")
        for line in failed:print(" -",line)
        raise SystemExit(1)
    print("PASS: no material regression against v1.9 within defined limits")
