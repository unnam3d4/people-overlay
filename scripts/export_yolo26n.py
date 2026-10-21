#!/usr/bin/env python3
"""Official pretrained YOLO26n -> NCNN, CPU-targeted, static 640x640.

IMPORTANT: NCNN export uses raw one-to-many [84,8400] predictions.
Do NOT use nms=False here: the end-to-end [300,6] path is unsupported
by this Win7 NCNN backend and has different decoding.
"""
from pathlib import Path
import shutil
import sys
from ultralytics import YOLO

dest = Path(sys.argv[1]).resolve()
dest.mkdir(parents=True, exist_ok=True)
print("Exporting official YOLO26n, COCO 80 classes, 640x640, one-to-many raw output", flush=True)
model = YOLO("yolo26n.pt")
folder = Path(model.export(format="ncnn", imgsz=640, batch=1, device="cpu"))
candidates = [
    (folder / "model.ncnn.param", folder / "model.ncnn.bin"),
    (folder / "model.param", folder / "model.bin"),
]
for param, weight in candidates:
    if param.is_file() and weight.is_file() and weight.stat().st_size > 100000:
        shutil.copyfile(param, dest / "model.param")
        shutil.copyfile(weight, dest / "model.bin")
        print("Export OK", param.name, weight.name, weight.stat().st_size, flush=True)
        print("NCNN param header:", (dest / "model.param").read_text(errors="replace")[:400], flush=True)
        break
else:
    raise SystemExit("YOLO26n NCNN .param/.bin not found or empty in " + str(folder))
