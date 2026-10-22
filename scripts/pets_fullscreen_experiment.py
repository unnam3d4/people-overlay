#!/usr/bin/env python3
"""Create separate full-frame vs high-resolution single-camera manifests.

Video and PETS ground truth unchanged. Experiment is NOT neural training.
"""
import json
from pathlib import Path
import argparse
if __name__=="__main__":
    parser=argparse.ArgumentParser()
    parser.add_argument("--fixtures",type=Path,required=True)
    a=parser.parse_args()
    for scene in ("single_less_crowded","single_crowded"):
        src=a.fixtures/f"pets2009_{scene}_15fps.json"
        d=json.loads(src.read_text(encoding="utf-8"))
        if d["profile"]!=0:raise SystemExit(f"Expected fullscreen: {src}")
        d["fullscreen_detail"]=True
        d["name"]+="_detail"
        d.setdefault("source",{})["experiment"]="Whole-frame scan alternates with a small-person 2x crop; source frames and full-body annotations unchanged"
        dst=a.fixtures/f"detail_pets2009_{scene}_15fps.json"
        dst.write_text(json.dumps(d,separators=(",",":")),encoding="utf-8")
        print(dst)
