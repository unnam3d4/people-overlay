#!/usr/bin/env python3
"""Reuse ONLY the embedded, verified NCNN+model bytes from v1.6.
The Windows Go program is rebuilt from current v1.7 source. This avoids
unnecessarily downloading PyTorch and rebuilding NCNN for every UI fix.
"""
from io import BytesIO
from pathlib import Path
import struct
import zipfile

root=Path(__file__).resolve().parent.parent
base=root/"BASE"
release=root/"RELEASE_NCNN"
release.mkdir(exist_ok=True)
marker=b"PPLAI06!"
for arch in ("x64","x86"):
 name=f"PeopleOverlay_Win7_NCNN_{arch}.exe"
 trusted=(base/name).read_bytes()
 if trusted[-16:-8]!=marker:
  raise SystemExit(f"Verified base {name} missing appended NCNN signature")
 length=struct.unpack("<Q",trusted[-8:])[0]
 if length<50000 or length>50_000_000 or length>len(trusted)-16:
  raise SystemExit(f"Invalid NCNN payload length for {name}")
 bundle=trusted[-16-length:-16]
 with zipfile.ZipFile(BytesIO(bundle)) as z:
  if not {"model.param","model.bin","ncnnbridge.dll"}.issubset(z.namelist()):
   raise SystemExit(f"Verified NCNN payload incomplete: {name}")
 fresh=(root/"_output"/arch/name).read_bytes()
 if fresh[-16:-8]==marker:
  raise SystemExit("Refusing nested NCNN payload")
 new=fresh+bundle+marker+struct.pack("<Q",length)
 (release/name).write_bytes(new)
 print(name,"fresh Go executable",len(fresh),"verified embedded NCNN payload",length,flush=True)
