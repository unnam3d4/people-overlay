#!/usr/bin/env python3
"""Attach exact NCNN payloads to Go EXEs. Reject absent or malformed inputs."""
from pathlib import Path
from io import BytesIO
import sys, struct, zipfile

root = Path(__file__).resolve().parent.parent
src = root / "src"
model = root / "_ncnn_model"
out = root / "RELEASE_NCNN"
out.mkdir(exist_ok=True)
for arch in ("x64", "x86"):
    inp = root / "_output" / arch
    exe = inp / ("PeopleOverlay_Win7_NCNN_" + arch + ".exe")
    required = {
        "model.param": model / "model.param",
        "model.bin": model / "model.bin",
        "ncnnbridge.dll": inp / "ncnnbridge.dll",
    }
    if not exe.exists():
        raise SystemExit(f"Missing compiled binary {exe}")
    if any(not v.exists() for v in required.values()):
        raise SystemExit(f"Missing model/dll: {required}")
    mem = BytesIO()
    with zipfile.ZipFile(mem, "w", compression=zipfile.ZIP_DEFLATED, compresslevel=5) as z:
        for name, path in required.items():
            z.write(path, arcname=name)
    body = mem.getvalue()
    with exe.open("ab") as f:
        f.write(body)
        f.write(b"PPLAI06!")
        f.write(struct.pack("<Q",len(body)))
    final = out/exe.name
    final.write_bytes(exe.read_bytes())
    print(arch, final, "bytes", final.stat().st_size, "embedded", {k:v.stat().st_size for k,v in required.items()}, flush=True)
