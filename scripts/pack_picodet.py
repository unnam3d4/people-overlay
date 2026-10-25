#!/usr/bin/env python3
"""Package an isolated PicoDet NCNN Win7 EXE with its OWN model and native DLL.
This rejects donors: a YOLO payload must never masquerade as PicoDet."""
import argparse
import hashlib
import io
import json
from pathlib import Path
import struct
import zipfile

MAGIC=b"PPLAI06!"
def pack(app: Path, param: Path, weights: Path, dll: Path, dest: Path):
    prefix=app.read_bytes()
    if not prefix.startswith(b"MZ") or prefix[-16:-8]==MAGIC:
        raise ValueError("expected new unpacked Windows PE application")
    contents={"model.param":param.read_bytes(),
              "model.bin":weights.read_bytes(),
              "ncnnbridge.dll":dll.read_bytes()}
    if any(len(data)<64 for data in contents.values()):
        raise ValueError("missing or suspiciously small PicoDet input")
    if b"transpose_0.tmp_0" not in contents["model.param"] or b"transpose_7.tmp_0" not in contents["model.param"]:
        raise ValueError("model is not the reviewed four-level PicoDet non-postprocess graph")
    if not contents["ncnnbridge.dll"].startswith(b"MZ"):
        raise ValueError("PicoDet native DLL is not Windows PE")
    output=io.BytesIO()
    with zipfile.ZipFile(output,"w",compression=zipfile.ZIP_DEFLATED,compresslevel=5) as z:
        for key,data in contents.items():z.writestr(key,data)
    bundle=output.getvalue()
    exe=prefix+bundle+MAGIC+struct.pack("<Q",len(bundle))
    dest.parent.mkdir(parents=True,exist_ok=True)
    dest.write_bytes(exe)
    report={"engine":"PicoDet-S320-NCNN","input":320,"requires_person_test":True,
            "resources_sha256":{n:hashlib.sha256(v).hexdigest() for n,v in contents.items()},
            "exe_sha256":hashlib.sha256(exe).hexdigest()}
    dest.with_suffix(".bundle.json").write_text(json.dumps(report,indent=2),encoding="utf8")

if __name__=="__main__":
    p=argparse.ArgumentParser()
    for name in ("app","param","weights","dll","dest"):
        p.add_argument("--"+name,required=True,type=Path)
    args=p.parse_args()
    pack(args.app,args.param,args.weights,args.dll,args.dest)
