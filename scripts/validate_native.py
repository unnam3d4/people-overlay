#!/usr/bin/env python3
"""Validate the shipped x64 NCNN DLL against official FP32 PyTorch weights.

Builder-only requirements: Python x64, ultralytics==8.4.78, numpy, OpenCV,
PyTorch CPU and Go 1.20.14. No Python is needed by the release EXE.
Inputs and output directories must be explicit. Model downloads require
--download; existing files are reused and their SHA256 is recorded.
This tests real bus/zidane images, not CCTV accuracy or Windows 7 hardware.
"""
import argparse
import ctypes
import hashlib
import io
import json
import os
from pathlib import Path
import shutil
import struct
import subprocess
import time
import urllib.request
import zipfile


GO_HARNESS = r'''package main
import("bytes";"encoding/binary";"encoding/json";"fmt";"os";"strconv")
func must(err error){if err!=nil{panic(err)}}
func num(s string)int{v,e:=strconv.Atoi(s);must(e);return v}
func f(s string)float64{v,e:=strconv.ParseFloat(s,64);must(e);return v}
func main(){
 a:=os.Args
 if a[1]=="prepare"{
  raw,e:=os.ReadFile(a[4]);must(e);w,h:=num(a[2]),num(a[3])
  tensor,scale,px,py:=prepareRegionFrame(raw,w,h,scanRegion{0,0,w,h})
  var b bytes.Buffer;must(binary.Write(&b,binary.LittleEndian,tensor));must(os.WriteFile(a[5],b.Bytes(),0600))
  must(json.NewEncoder(os.Stdout).Encode([]float64{scale,px,py}));return
 }
 if a[1]=="decode"{
  raw,e:=os.ReadFile(a[2]);must(e);values:=make([]float32,len(raw)/4)
  must(binary.Read(bytes.NewReader(raw),binary.LittleEndian,values))
  boxes:=decodePeople(values,num(a[3]),num(a[4]),num(a[5]),f(a[6]),f(a[7]),f(a[8]),.25)
  must(json.NewEncoder(os.Stdout).Encode(boxes));return
 }
 panic(fmt.Sprint("unknown mode ",a[1]))
}
'''


def digest(data):
    return hashlib.sha256(data).hexdigest()


def pe_imports(data):
    """Inspect import names without executing the file or installing PE tools."""
    pe = struct.unpack_from('<I', data, 60)[0]
    count = struct.unpack_from('<H', data, pe + 6)[0]
    opt = pe + 24
    optional_size = struct.unpack_from('<H', data, pe + 20)[0]
    magic = struct.unpack_from('<H', data, opt)[0]
    sections = [struct.unpack_from('<IIII', data, opt + optional_size + i*40 + 8) for i in range(count)]

    def offset(rva):
        for virtual_size, address, raw_size, raw_offset in sections:
            if address <= rva < address + max(virtual_size, raw_size):
                return raw_offset + rva - address
        return rva

    def string(rva):
        start = offset(rva)
        return data[start:data.index(b'\0', start)].decode()

    directory = opt + (112 if magic == 523 else 96)
    pos = offset(struct.unpack_from('<I', data, directory + 8)[0])
    imports = {}
    while any(struct.unpack_from('<IIIII', data, pos)):
        original, _, _, name, first = struct.unpack_from('<IIIII', data, pos)
        at = offset(original or first)
        step = 8 if magic == 523 else 4
        functions = []
        while True:
            value = struct.unpack_from('<Q' if step == 8 else '<I', data, at)[0]
            if not value:
                break
            functions.append('#' + str(value & 65535) if value >> (step*8-1) else string(value + 2))
            at += step
        imports[string(name)] = functions
        pos += 20
    return imports


def extract(exe, dest):
    data = exe.read_bytes()
    if data[-16:-8] != b'PPLAI06!':
        raise ValueError('missing embedded bundle: ' + str(exe))
    length = struct.unpack('<Q', data[-8:])[0]
    if length > len(data)-16:
        raise ValueError('invalid embedded bundle length')
    dest.mkdir(parents=True, exist_ok=True)
    hashes = {}
    with zipfile.ZipFile(io.BytesIO(data[-16-length:-16])) as archive:
        if set(archive.namelist()) != {'model.param', 'model.bin', 'ncnnbridge.dll'}:
            raise ValueError('unexpected bundle members')
        for name in archive.namelist():
            resource = archive.read(name)
            (dest/name).write_bytes(resource)
            hashes[name] = digest(resource)
    return {'exe_sha256': digest(data), 'resources_sha256': hashes,
            'exe_imports': pe_imports(data), 'dll_imports': pe_imports((dest/'ncnnbridge.dll').read_bytes())}


class Native:
    def __init__(self, folder):
        self.dll = ctypes.WinDLL(str(folder/'ncnnbridge.dll'))
        self.dll.PPLLoad.argtypes = [ctypes.c_wchar_p, ctypes.c_wchar_p]
        self.dll.PPLLoad.restype = ctypes.c_int
        ptr = ctypes.POINTER(ctypes.c_float)
        self.dll.PPLInfer.argtypes = [ptr, ctypes.c_int, ptr, ctypes.c_int, ctypes.POINTER(ctypes.c_int)]
        self.dll.PPLInfer.restype = ctypes.c_int
        self.dll.PPLSetThreads.argtypes = [ctypes.c_int]
        self.dll.PPLSetThreads.restype = None
        self.dll.PPLClose.argtypes = []
        self.dll.PPLClose.restype = None
        code = self.dll.PPLLoad(str(folder/'model.param'), str(folder/'model.bin'))
        if code:
            raise RuntimeError('PPLLoad: ' + str(code))

    def infer(self, tensor, threads=1):
        import numpy as np
        self.dll.PPLSetThreads(threads)
        out = np.empty(84*8400, dtype=np.float32)
        count = ctypes.c_int()
        ptr = ctypes.POINTER(ctypes.c_float)
        start = time.perf_counter()
        code = self.dll.PPLInfer(tensor.ctypes.data_as(ptr), tensor.size, out.ctypes.data_as(ptr), out.size, ctypes.byref(count))
        elapsed = (time.perf_counter()-start)*1000
        if code or count.value != 8400:
            raise RuntimeError(f'PPLInfer: code={code}, count={count.value}')
        out = out.reshape(84, count.value)
        if not np.isfinite(out).all():
            raise ValueError('nonfinite native output')
        return out, elapsed

    def close(self):
        self.dll.PPLClose()


def run_harness(harness, *args):
    run = subprocess.run([str(harness), *map(str, args)], check=True, capture_output=True, text=True)
    return json.loads(run.stdout)


def official_boxes(raw, width, height, transform):
    import numpy as np
    import torch
    from ultralytics.utils.nms import non_max_suppression
    found = non_max_suppression(torch.from_numpy(raw.copy())[None], conf_thres=.25,
                               iou_thres=.45, classes=[0], max_det=80, nc=80)[0].numpy()
    scale, px, py = transform
    found[:, [0, 2]] = np.clip((found[:, [0, 2]]-px)/scale, 0, width)
    found[:, [1, 3]] = np.clip((found[:, [1, 3]]-py)/scale, 0, height)
    return found[(found[:, 2]-found[:, 0] >= 6) & (found[:, 3]-found[:, 1] >= 9), :5]


def compare_boxes(actual, reference):
    import numpy as np
    if len(actual) != len(reference):
        return {'count_equal': False, 'actual_count': len(actual), 'reference_count': len(reference)}
    used, ious, coordinate_deltas, score_deltas = set(), [], [], []
    for box in actual:
        best = (-1, -1.)
        for j, ref in enumerate(reference):
            if j in used:
                continue
            inter = np.maximum(0, np.minimum(box[2:4], ref[2:4])-np.maximum(box[:2], ref[:2])).prod()
            union = (box[2:4]-box[:2]).prod() + (ref[2:4]-ref[:2]).prod() - inter
            iou = float(inter/union) if union else 0.
            if iou > best[1]:
                best = j, iou
        j, iou = best
        used.add(j)
        ious.append(iou)
        coordinate_deltas.append(float(np.max(np.abs(box[:4]-reference[j, :4]))))
        score_deltas.append(float(abs(box[4]-reference[j, 4])))
    return {'count_equal': True, 'count': len(actual), 'min_iou': min(ious, default=1.),
            'max_coordinate_error_px': max(coordinate_deltas, default=0.),
            'max_score_error': max(score_deltas, default=0.)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--v18', type=Path, required=True, help='YOLOv8n x64 portable EXE')
    parser.add_argument('--v19', type=Path, required=True, help='YOLO26n x64 portable EXE')
    parser.add_argument('--work', type=Path, required=True)
    parser.add_argument('--go', type=Path, required=True)
    parser.add_argument('--download', action='store_true')
    args = parser.parse_args()
    args.work = args.work.resolve()
    args.work.mkdir(parents=True, exist_ok=True)
    os.environ['YOLO_CONFIG_DIR'] = str(args.work)
    import cv2
    import numpy as np
    import torch
    import ultralytics
    from ultralytics import YOLO
    from ultralytics.data.augment import LetterBox
    if ultralytics.__version__ != '8.4.78':
        raise RuntimeError('Reference must use ultralytics==8.4.78')
    torch.set_num_threads(1)
    source = Path(__file__).resolve().parent.parent/'src'
    harness_dir = args.work/'go-harness'
    harness_dir.mkdir(exist_ok=True)
    for name in ('scan.go', 'detection.go'):
        shutil.copyfile(source/name, harness_dir/name)
    (harness_dir/'main.go').write_text(GO_HARNESS)
    harness = harness_dir/'validate.exe'
    subprocess.run([str(args.go.resolve()), 'build', '-o', str(harness), 'main.go', 'scan.go', 'detection.go'],
                   cwd=harness_dir, check=True)
    report = {'reference': {'ultralytics': ultralytics.__version__, 'torch': torch.__version__,
                           'opencv': cv2.__version__, 'torch_threads': 1,
                           'go_sources_sha256': {n: digest((source/n).read_bytes()) for n in ('scan.go', 'detection.go')}},
              'limitations': ['Host Windows x64 CPU only; not a Windows 7 or old-CPU runtime test.',
                              'bus/zidane are still images, not CCTV accuracy or tracking benchmarks.',
                              'NCNN was built with OpenMP disabled; thread=2 setting does not prove parallelism.'],
              'models': {}}
    prepared = []
    for image_name in ('bus', 'zidane'):
        path = args.work/(image_name+'.jpg')
        image = cv2.imread(str(path))
        if image is None:
            raise ValueError('missing test image ' + str(path))
        height, width = image.shape[:2]
        bgra, tensor_file = args.work/(image_name+'.bgra'), args.work/(image_name+'.f32')
        cv2.cvtColor(image, cv2.COLOR_BGR2BGRA).tofile(bgra)
        transform = run_harness(harness, 'prepare', width, height, bgra, tensor_file)
        tensor = np.fromfile(tensor_file, np.float32).reshape(3, 640, 640)
        standard = LetterBox((640, 640), auto=False)(image=image)[:, :, ::-1].transpose(2, 0, 1).astype(np.float32)/255
        delta = np.abs(tensor-standard)
        prepared.append((image_name, width, height, transform, tensor, {'sha256': digest(path.read_bytes()),
                         'input_sha256': digest(tensor.tobytes()), 'opencv_preprocess_max_error': float(delta.max()),
                         'opencv_preprocess_mean_error': float(delta.mean())}))
    for model_name, exe in [('yolov8n', args.v18), ('yolo26n', args.v19)]:
        pt = args.work/(model_name+'.pt')
        if not pt.is_file():
            if not args.download:
                raise FileNotFoundError(f'{pt}: use --download for official weights')
            tag = 'v8.4.0' if model_name == 'yolo26n' else 'v8.3.0'
            urllib.request.urlretrieve(f'https://github.com/ultralytics/assets/releases/download/{tag}/{model_name}.pt', pt)
        folder = args.work/model_name
        details = extract(exe.resolve(), folder)
        details['pt_sha256'] = digest(pt.read_bytes())
        details['images'] = []
        model = YOLO(str(pt)).model.float().eval()
        # Do this BEFORE fuse: otherwise YOLO26 drops the one-to-many head.
        if hasattr(model, 'end2end'):
            model.end2end = False
        model.fuse(verbose=False)
        head = model.model[-1]
        head.export, head.format, head.dynamic, head.xyxy = True, 'ncnn', False, False
        native = Native(folder)
        try:
            for image_name, width, height, transform, tensor, image_info in prepared:
                t = torch.from_numpy(tensor.copy())[None]
                with torch.inference_mode():
                    model(t)  # warmup
                    start = time.perf_counter()
                    reference = model(t)
                    torch_ms = (time.perf_counter()-start)*1000
                    if isinstance(reference, tuple):
                        reference = reference[0]
                    reference = reference[0].numpy()
                if reference.shape != (84, 8400):
                    raise ValueError('unexpected PyTorch shape ' + str(reference.shape))
                native.infer(tensor)
                native_out, native_ms = native.infer(tensor)
                two_out, two_ms = native.infer(tensor, 2)
                raw_file = args.work/(model_name+'-'+image_name+'-native.f32')
                native_out.tofile(raw_file)
                go = run_harness(harness, 'decode', raw_file, 8400, width, height, *transform) or []
                actual = np.array([[r['X'], r['Y'], r['X']+r['W'], r['Y']+r['H'], r['Score']] for r in go], dtype=np.float64).reshape(-1, 5)
                expected_native = official_boxes(native_out, width, height, transform)
                expected_torch = official_boxes(reference, width, height, transform)
                delta = np.abs(native_out-reference)
                row = {'image': image_name, **image_info, 'shape': list(native_out.shape),
                       'native_ms': native_ms, 'native_threads2_ms': two_ms, 'torch_ms': torch_ms,
                       'raw_bbox_max_abs_error': float(delta[:4].max()),
                       'raw_score_max_abs_error': float(delta[4:].max()),
                       'raw_mean_abs_error': float(delta.mean()),
                       'threads_output_max_abs_error': float(np.abs(native_out-two_out).max()),
                       'go_vs_ultralytics_on_same_native_raw': compare_boxes(actual, expected_native),
                       'native_vs_torch_detections': compare_boxes(actual, expected_torch),
                       'native_boxes_xyxy_score': actual.tolist(), 'torch_boxes_xyxy_score': expected_torch.tolist()}
                details['images'].append(row)
                print(model_name, image_name, json.dumps({k: row[k] for k in ('native_ms', 'raw_bbox_max_abs_error', 'raw_score_max_abs_error', 'go_vs_ultralytics_on_same_native_raw', 'native_vs_torch_detections')}), flush=True)
        finally:
            native.close()
        report['models'][model_name] = details
    checks = []
    for details in report['models'].values():
        for row in details['images']:
            decoder = row['go_vs_ultralytics_on_same_native_raw']
            reference = row['native_vs_torch_detections']
            checks.append(decoder['count_equal'] and decoder.get('max_coordinate_error_px', 1) < .001
                          and reference['count_equal'] and reference.get('min_iou', 0) > .99
                          and reference.get('max_score_error', 1) < .001
                          and row['raw_bbox_max_abs_error'] < .1 and row['raw_score_max_abs_error'] < .001)
    report['passed'] = all(checks)
    target = args.work/'native-validation.json'
    target.write_text(json.dumps(report, indent=2), encoding='utf-8')
    print('Report:', target, 'PASS' if report['passed'] else 'FAIL', flush=True)
    if not report['passed']:
        raise SystemExit(1)


if __name__ == '__main__':
    main()
