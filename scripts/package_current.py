#!/usr/bin/env python3
"""Pack freshly compiled application bytes with an explicitly selected NCNN bundle.

Never reuse the application prefix of the donor EXE. The donor supplies only
model.param, model.bin and ncnnbridge.dll. No network or model export occurs.
"""
import argparse
import hashlib
import io
import json
from pathlib import Path
import struct
import zipfile

MAGIC = b'PPLAI06!'

def package(application, donor, output):
    fresh, old = application.read_bytes(), donor.read_bytes()
    if fresh[:2] != b'MZ' or fresh[-16:-8] == MAGIC:
        raise ValueError('application must be a fresh unpacked Windows executable')
    if old[-16:-8] != MAGIC:
        raise ValueError('donor has no embedded bundle')
    size = struct.unpack('<Q', old[-8:])[0]
    if not 128 <= size <= len(old)-16:
        raise ValueError('invalid donor bundle size')
    bundle = old[-16-size:-16]
    hashes = {}
    with zipfile.ZipFile(io.BytesIO(bundle)) as archive:
        names = ['model.param', 'model.bin', 'ncnnbridge.dll']
        if sorted(archive.namelist()) != sorted(names):
            raise ValueError('invalid donor members')
        for name in names:
            data = archive.read(name)  # validates ZIP CRC as well
            if len(data) < 64:
                raise ValueError('empty donor resource')
            hashes[name] = hashlib.sha256(data).hexdigest()
    result = fresh + bundle + MAGIC + struct.pack('<Q', len(bundle))
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_bytes(result)
    report = {'application_sha256': hashlib.sha256(fresh).hexdigest(),
              'donor_sha256': hashlib.sha256(old).hexdigest(),
              'resources_sha256': hashes, 'exe_sha256': hashlib.sha256(result).hexdigest(),
              'bytes': len(result)}
    output.with_suffix('.bundle.json').write_text(json.dumps(report, indent=2), encoding='utf-8')
    return report

if __name__ == '__main__':
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--application', type=Path, required=True)
    p.add_argument('--donor', type=Path, required=True)
    p.add_argument('--output', type=Path, required=True)
    a = p.parse_args()
    print(json.dumps(package(a.application, a.donor, a.output), indent=2))
