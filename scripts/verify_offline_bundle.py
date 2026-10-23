#!/usr/bin/env python3
"""Validate every offline release EXE without third-party Python modules.

This runs ONLY on CI/release packaging machines; operator PCs run the EXE
directly without Python or Internet.
"""
import hashlib
import io
import json
from pathlib import Path
import struct
import sys
import zipfile

MAGIC=b"PPLAI06!"
ALLOWED={"model.param","model.bin","ncnnbridge.dll"}
ARCH={"x64":0x8664,"x86":0x014c}

def machine(pe):
    if len(pe)<0x40 or pe[:2]!=b"MZ":
        raise ValueError("not Windows PE executable")
    off=struct.unpack_from("<I",pe,0x3C)[0]
    if off+6>len(pe) or pe[off:off+4]!=b"PE\\x00\\x00":
        raise ValueError("invalid PE NT header")
    return struct.unpack_from("<H",pe,off+4)[0]

def verify(path):
    expected=ARCH["x64" if path.stem.endswith("_x64") else "x86"]
    app=path.read_bytes()
    if machine(app)!=expected:
        raise ValueError(f"{path.name}: EXE bitness not matched")
    if len(app)<16 or app[-16:-8]!=MAGIC:
        raise ValueError(f"{path.name}: no embedded offline bundle")
    size=struct.unpack_from("<Q",app,len(app)-8)[0]
    if size<128 or size>256*1024*1024 or size>len(app)-16:
        raise ValueError(f"{path.name}: invalid bundle length")
    data=app[-16-size:-16]
    with zipfile.ZipFile(io.BytesIO(data)) as z:
        if set(z.namelist())!=ALLOWED or len(z.namelist())!=3:
            raise ValueError(f"{path.name}: missing/foreign embedded resources")
        resources={name:z.read(name) for name in ALLOWED}
    if machine(resources["ncnnbridge.dll"])!=expected:
        raise ValueError(f"{path.name}: embedded DLL bitness mismatch")
    if min(map(len,resources.values()))<64:
        raise ValueError(f"{path.name}: empty model or bridge")
    manifest=path.with_suffix(".bundle.json")
    if not manifest.exists():
        raise ValueError(f"{path.name}: missing build manifest")
    metadata=json.loads(manifest.read_text(encoding="utf8"))
    if metadata["exe_sha256"]!=hashlib.sha256(app).hexdigest():
        raise ValueError(f"{path.name}: packaged EXE checksum mismatch")
    for name,data in resources.items():
        if metadata["resources_sha256"][name]!=hashlib.sha256(data).hexdigest():
            raise ValueError(f"{path.name}: resource {name} integrity mismatch")
    print(f"OK {path.name}: offline model, Windows PE {expected:04X} and native bridge embedded; {len(app)} bytes")

def main():
    paths=sorted(Path(sys.argv[1]).glob("PeopleOverlay_v3_YOLO*.exe"))
    if len(paths)!=4:
        raise ValueError(f"expected x86/x64 x 2 built-in models; got {len(paths)} EXEs")
    for path in paths:
        verify(path)

if __name__=="__main__":
    main()
