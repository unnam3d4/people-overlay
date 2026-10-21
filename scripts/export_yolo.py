"""Export genuine pretrained YOLOv8n weights for legacy ONNX Runtime 1.11.
Runs only on a GitHub-hosted builder, never on end-user PCs.
"""
from pathlib import Path
import shutil
import sys
from ultralytics import YOLO

dest = Path(sys.argv[1]).resolve()
dest.parent.mkdir(parents=True, exist_ok=True)
print("Exporting official COCO YOLOv8n model, fixed 640x640, opset 12...", flush=True)
model = YOLO("yolov8n.pt")
source = Path(model.export(format="onnx", imgsz=640, opset=12, simplify=False, dynamic=False))
if source.resolve() != dest:
    shutil.copyfile(source, dest)
if dest.stat().st_size < 5000000:
    raise SystemExit("ONNX export is unexpectedly small")
print("Exported", dest, "bytes", dest.stat().st_size, flush=True)
