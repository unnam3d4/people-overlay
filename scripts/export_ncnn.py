"""Export the official pretrained YOLOv8n to NCNN for offline Win7/8/10/11.
Runs on GitHub Actions; no Python needed on users' PCs.
"""
from pathlib import Path
import shutil
import sys
from ultralytics import YOLO

dest = Path(sys.argv[1]).resolve()
dest.mkdir(parents=True, exist_ok=True)
print("NCNN export: YOLOv8n COCO, 640x640, FP32", flush=True)
model = YOLO("yolov8n.pt")
folder = Path(model.export(format="ncnn", imgsz=640, half=False, batch=1))
param, weight = folder/"model.ncnn.param", folder/"model.ncnn.bin"
if not param.exists() or not weight.exists():
    # Older Ultralytics exporters use model.param/model.bin.
    param,weight = folder/"model.param",folder/"model.bin"
if not param.exists() or not weight.exists():
    raise SystemExit(f"NCNN exporter did not produce .param and .bin in {folder}")
shutil.copyfile(param,dest/"model.param")
shutil.copyfile(weight,dest/"model.bin")
print(f"NCNN ready: {weight.stat().st_size} weight bytes, {param.stat().st_size} param bytes", flush=True)
