#!/usr/bin/env python3
"""Produce standalone v1.9 EXEs with actual YOLO26n model bytes.

The previously verified v1.8 ZIP supplies both known-working Win7 DLL
binaries. Actual Windows x86/x64 model load and full inference are mandatory
in CI before a package can be called verified; do not label this phase tested.
"""
from io import BytesIO
from pathlib import Path
import struct
import zipfile
from hashlib import sha256

root = Path(__file__).resolve().parent.parent
base = root / "BASE"
model = root / "_ncnn_model"
release = root / "RELEASE_NCNN"
release.mkdir(exist_ok=True)
marker = b"PPLAI06!"
param, weight = (model / "model.param").read_bytes(), (model / "model.bin").read_bytes()
if len(param) < 100 or len(weight) < 100000:
    raise SystemExit("YOLO26n model not exported correctly")
print("YOLO26n source weight SHA256", sha256(weight).hexdigest(), flush=True)
for arch in ("x64", "x86"):
    name = f"PeopleOverlay_Win7_NCNN_{arch}.exe"
    original = (base / name).read_bytes()
    if original[-16:-8] != marker:
        raise SystemExit(f"Missing verified v1.8 embedded bundle: {name}")
    length = struct.unpack("<Q", original[-8:])[0]
    if length < 50000 or length > 60_000_000 or length > len(original)-16:
        raise SystemExit("Invalid old model package")
    old = original[-16-length:-16]
    with zipfile.ZipFile(BytesIO(old)) as z:
        dll = z.read("ncnnbridge.dll")
        old_weight = z.read("model.bin")
    if sha256(weight).digest() == sha256(old_weight).digest():
        raise SystemExit("Refusing fake v1.9: model is identical to YOLOv8n")
    mem = BytesIO()
    with zipfile.ZipFile(mem, "w", compression=zipfile.ZIP_DEFLATED, compresslevel=5) as z:
        z.writestr("model.param", param)
        z.writestr("model.bin", weight)
        z.writestr("ncnnbridge.dll", dll)
    bundle = mem.getvalue()
    fresh = original[: -16 - length]
    output = release / f"PeopleOverlay_v1.9_YOLO26n_{arch}.exe"
    output.write_bytes(fresh + bundle + marker + struct.pack("<Q", len(bundle)))
    print(output.name, "size", output.stat().st_size, "model bytes", len(weight), flush=True)
