#!/usr/bin/env python3
"""Reject release if PicoDet V2 loses single-camera detection or adds a crowd regression."""
import json,sys
from pathlib import Path
d=json.loads(Path(sys.argv[1]).read_text())
scenes={r["model"]+"__"+r["scenario"]:r for r in d}
for name in ("one_camera_12_people","grid_4x4_48_people"):
    old=scenes["PicoDet__"+name]
    new=scenes["PicoDetV2__"+name]
    print(name,":",old["frame_weighted_recall_iou_05"],"->",
          new["frame_weighted_recall_iou_05"],"last",old["last_frame_matched"],"->",
          new["last_frame_matched"],"p95",old["infer_p95_ms"],"->",new["infer_p95_ms"])
    if new["frame_weighted_recall_iou_05"] < old["frame_weighted_recall_iou_05"] - .02:
        raise SystemExit("REGRESSION: lost recall in "+name)
    if new["last_frame_matched"] < old["last_frame_matched"]:
        raise SystemExit("REGRESSION: fewer people in last frame "+name)
    if new["infer_p95_ms"] > old["infer_p95_ms"] * 1.40:
        raise SystemExit("REGRESSION: inference too slow "+name)
improved=scenes["PicoDetV2__grid_4x4_48_people"]["frame_weighted_recall_iou_05"] > (
   scenes["PicoDet__grid_4x4_48_people"]["frame_weighted_recall_iou_05"] + .04)
if not improved: raise SystemExit("Crowd improvement too small for new release")
print("PASS: PicoDet v2 releases only after demonstrable crowd recall improvement.")
