#!/usr/bin/env python3
import json,sys
from pathlib import Path
data=json.loads(Path(sys.argv[1]).read_text(encoding="utf8"))
rows={r["model"]+"__"+r["scenario"]:r for r in data}
for scene in ("one_camera_12_people","grid_4x4_48_people"):
    old=rows["PicoDetV2__"+scene];new=rows["PicoDetV3__"+scene]
    print(f"{scene} recall {old['frame_weighted_recall_iou_05']:.2%} -> {new['frame_weighted_recall_iou_05']:.2%}, last {old['last_frame_matched']} -> {new['last_frame_matched']}, p95 {old['infer_p95_ms']:.1f} -> {new['infer_p95_ms']:.1f} ms")
    if new["frame_weighted_recall_iou_05"] < old["frame_weighted_recall_iou_05"] - .01:
        raise SystemExit("REGRESSION: worse person recall "+scene)
    if new["last_frame_matched"] < old["last_frame_matched"]:
        raise SystemExit("REGRESSION: fewer humans at end "+scene)
    if new["infer_p95_ms"] > old["infer_p95_ms"]*1.40:
        raise SystemExit("REGRESSION: inference too expensive "+scene)
improvement=rows["PicoDetV3__grid_4x4_48_people"]["frame_weighted_recall_iou_05"]-rows["PicoDetV2__grid_4x4_48_people"]["frame_weighted_recall_iou_05"]
if improvement<.02:raise SystemExit(f"Crowd gain too small ({improvement:.2%}); do NOT publish v3")
print("PASS: PicoDet v3 crowd recognition improved while single camera and CPU stable.")
