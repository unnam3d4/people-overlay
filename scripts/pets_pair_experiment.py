#!/usr/bin/env python3
"""Generate a test-only 8-ROI vertical-pair experiment from genuine PETS2009 PNGs.

The copies reference original real video frames and unchanged person-body GT.
No new image generation, no ground-truth modifications, no model retraining.
"""
import argparse
import json
from pathlib import Path

def make_pair_manifests(root):
    items=[]
    for which in ("grid4_four_feeds", "grid4_sixteen_feeds"):
        p=root / f"pets2009_{which}_15fps.json"
        manifest=json.loads(p.read_text(encoding="utf-8"))
        if manifest["profile"]!=3 or not manifest["frames"]:
            raise ValueError(f"Incorrect input fixture: {p}")
        manifest["paired_cctv"]=True
        manifest["name"]+="_vertical_pairs"
        info=manifest.setdefault("source",{})
        info["experiment"]="2 vertically stacked 4x4 camera tiles in a single NCNN call; body annotations and frames unchanged"
        dst=root / f"paired_pets2009_{which}_15fps.json"
        dst.write_text(json.dumps(manifest,separators=(",",":")),encoding="utf-8")
        items.append(dst)
    # Separate controlled study of temporal box geometry, using the same
    # two source manifests and independent manual whole-person labels.
    for which in ("single_less_crowded","single_crowded"):
        p=root / f"pets2009_{which}_15fps.json"
        manifest=json.loads(p.read_text(encoding="utf-8"))
        if manifest["profile"]!=0:raise ValueError("not a fullscreen one-camera fixture")
        manifest["tight_ai_boxes"]=True
        manifest["name"]+="_tight_boxes"
        manifest.setdefault("source",{})["experiment"]="Strong neural body boxes replace stale interpolated tracker outlines"
        dst=root / f"tight_pets2009_{which}_15fps.json"
        dst.write_text(json.dumps(manifest,separators=(",",":")),encoding="utf-8")
        items.append(dst)
    return items

if __name__=="__main__":
    parser=argparse.ArgumentParser()
    parser.add_argument("--fixtures",type=Path,required=True)
    args=parser.parse_args()
    for item in make_pair_manifests(args.fixtures):
        print("Paired CCTV benchmark manifest:",item)
