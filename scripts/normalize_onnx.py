"""Prepare a modern YOLOv8 ONNX for ONNX Runtime 1.11.1 (Windows 7 target).
Runs ONLY on GitHub CI; end users need no Python installation.
"""
import sys
import onnx
from onnx import version_converter

path = sys.argv[1]
model = onnx.load(path, load_external_data=False)
ops = [(o.domain or "ai.onnx", o.version) for o in model.opset_import]
print("BEFORE:", "IR", model.ir_version, "opsets", ops, flush=True)
onnx.checker.check_model(model)
main = next((o.version for o in model.opset_import if o.domain in ("", "ai.onnx")), None)
if main is None:
    raise SystemExit("No ai.onnx opset")
if main > 16:
    print("Converting ai.onnx opset", main, "to 16", flush=True)
    model = version_converter.convert_version(model, 16)
    main = next(o.version for o in model.opset_import if o.domain in ("", "ai.onnx"))
if main < 7:
    raise SystemExit("Unsupported ai.onnx opset")
# ONNX Runtime 1.11.x supports up to IR 8; modern exporters may stamp IR 9
# while using only operators/fields available with IR 8.
if model.ir_version > 8:
    model.ir_version = 8
onnx.checker.check_model(model)
onnx.save(model, path)
print("AFTER:", "IR", model.ir_version, "ai.onnx opset", main, flush=True)
