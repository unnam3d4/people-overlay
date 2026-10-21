#!/usr/bin/env python3
"""Build real-video replay from a GitHub-hosted manually annotated CCTV-like MP4.

This source has manual CVAT face *points*, not body bounding boxes. The fixture
therefore emits landmarks, NOT invented person boxes. This is a second and
independent, weaker validation axis: face-in-detection coverage and ID stability.
It must NEVER be reported as standard box-based precision/recall.
"""
import argparse
from collections import defaultdict
import hashlib
import io
import json
from pathlib import Path
import urllib.request
import xml.etree.ElementTree as ET
import zipfile

REPOSITORY = "https://github.com/razayaseen5/Multi-Target-Pedestrian-Pose-CVAT-Video-Dataset"
RAW = "https://raw.githubusercontent.com/razayaseen5/Multi-Target-Pedestrian-Pose-CVAT-Video-Dataset/main/"
VIDEO = "Media_Assets/source_raw_video.mp4"
XML_ZIP = "Exported_Annotations/CVAT_Video_Head_and_Body_Keypoints_Interpolation.zip"
VIDEO_SHA256 = "8714e5ead7bafc4033232143f1c9c0a40c429bf0c397c0944a379a83eec6d47b"
XML_SHA256 = "1c47f44fe4e514528fc285fa9ee9e643b390c4b24247a437717b25d71aff3f42"


def download(path, dest, expected):
    dest.parent.mkdir(parents=True, exist_ok=True)
    if dest.is_file() and hashlib.sha256(dest.read_bytes()).hexdigest() == expected:
        return dest
    url = RAW + path
    data = urllib.request.urlopen(urllib.request.Request(
        url, headers={"User-Agent": "PeopleOverlayCI/1.0"}), timeout=35).read(8_000_001)
    got = hashlib.sha256(data).hexdigest()
    if got != expected:
        raise RuntimeError("public real-video source checksum changed: " + path + ", got " + got)
    dest.write_bytes(data)
    print("Verified filmed-person source", url, len(data), got, flush=True)
    return dest


def face_points_from_cvat(xml_bytes, *, require_real_length=True):
    root = ET.fromstring(xml_bytes)
    face = defaultdict(list)
    tracks = root.findall("track")
    for track in tracks:
        if track.get("label") != "face_Keypoints":
            continue
        track_id = "track" + str(track.get("id"))
        for point in track.findall("points"):
            if point.get("outside") == "1":
                continue
            try:
                n = int(point.get("frame"))
                coordinates = point.get("points", "").split(";")[0].split(",")
                x, y = float(coordinates[0]), float(coordinates[1])
            except (TypeError, ValueError, IndexError):
                continue
            if n < 0 or not (x >= 0 and y >= 0):
                continue
            face[n].append({"id": track_id, "x": x, "y": y})
    if require_real_length and (len(face) < 20 or not any(len(p) >= 2 for p in face.values())):
        raise RuntimeError("insufficient CVAT face landmarks to score real video")
    return dict(face)


def manifest_frames(video, landmarks, output, duration=8, fps=15, grid=False):
    import cv2
    import numpy as np
    capture = cv2.VideoCapture(str(video))
    if not capture.isOpened():
        raise RuntimeError("GitHub real pedestrian video could not be opened")
    source_fps = float(capture.get(cv2.CAP_PROP_FPS))
    width, height = int(capture.get(3)), int(capture.get(4))
    length = int(capture.get(cv2.CAP_PROP_FRAME_COUNT))
    if not (20 < source_fps < 40 and width >= 640 and height >= 360 and length >= 100):
        raise RuntimeError(f"unexpected real video metadata: {width}x{height}, {source_fps} fps, {length} frames")
    source_frames = []
    count = min(length, int((duration + (3 if grid else 0)) * source_fps) + 1)
    for i in range(count):
        ok, data = capture.read()
        if not ok:
            break
        source_frames.append(data)
    capture.release()
    if len(source_frames) < duration * source_fps:
        raise RuntimeError("truncated real video footage")
    out_frames = output / "frames"
    out_frames.mkdir(parents=True, exist_ok=True)
    entries = []
    tiles = [0, 5, 10, 15] if grid else [0]
    for i in range(duration * fps):
        screen = np.full((540, 960, 3), 82, dtype=np.uint8)
        points = []
        for tile in tiles:
            start_seconds = (tile % 4) if grid else 0
            source_frame = min(len(source_frames)-1, int((i / fps + start_seconds) * source_fps))
            # Original CVAT frame IDs start at zero and have same source video.
            frame = source_frames[source_frame]
            ox = tile % 4 * 240 if grid else 0
            oy = tile // 4 * 135 if grid else 0
            ww, hh = (240,135) if grid else (960,540)
            screen[oy:oy+hh,ox:ox+ww] = cv2.resize(frame,(ww,hh),interpolation=cv2.INTER_AREA)
            for landmark in landmarks.get(source_frame,()):
                points.append({"id":f"tile{tile}/{landmark['id']}",
                               "x":ox+landmark["x"]*ww/width,
                               "y":oy+landmark["y"]*hh/height})
        path=f"frames/github_cvat_{'grid' if grid else 'single'}_{fps}_{i:04d}.png"
        cv2.imwrite(str(output/path),screen,[cv2.IMWRITE_PNG_COMPRESSION,2])
        entries.append({"file":path,"people":[],"landmarks":points,"label":"manual_face_point_ground_truth"})
    if sum(len(x["landmarks"]) for x in entries)<50:
        raise RuntimeError("too few manually annotated face points in source frames")
    source = {"url":REPOSITORY+"/blob/main/"+VIDEO,
              "annotation_url":REPOSITORY+"/blob/main/"+XML_ZIP,
              "video_sha256":VIDEO_SHA256,"annotation_sha256":XML_SHA256,
              "license":"Source repo MIT; cite original authors",
              "notes":"Real 1280x720 MP4 with manually marked CVAT face points. No full-body box ground truth: face-hit coverage/ID only. Grid is a time-shifted four-feed mosaic, not actual 16 RVi cameras.",
              "fps":str(source_fps)}
    name=f"github_cvat_{'grid' if grid else 'single'}_{fps}fps"
    manifest={"name":name,"source":source,"fps":fps,"profile":3 if grid else 0,
              "sensitivity":2,"threads":1,"frames":entries}
    (output/(name+".json")).write_text(json.dumps(manifest,separators=(",",":")))
    print(name,"frames",len(entries),"annotated_face_points",sum(len(x["landmarks"]) for x in entries),flush=True)
    return source


def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument("--output",type=Path,required=True)
    p.add_argument("--cache",type=Path,required=True)
    p.add_argument("--duration",type=int,default=8)
    a=p.parse_args()
    if not 4<=a.duration<=12:raise SystemExit("duration must be 4..12 seconds")
    video=download(VIDEO,a.cache/"source_raw_video.mp4",VIDEO_SHA256)
    annotation=download(XML_ZIP,a.cache/"cvat_annotated.zip",XML_SHA256)
    with zipfile.ZipFile(io.BytesIO(annotation.read_bytes())) as z:
        landmarks=face_points_from_cvat(z.read("annotations.xml"))
    a.output.mkdir(parents=True,exist_ok=True)
    source=manifest_frames(video,landmarks,a.output,duration=a.duration,grid=False)
    manifest_frames(video,landmarks,a.output,duration=a.duration,grid=True)
    (a.output/"SOURCE-GITHUB-CVAT.json").write_text(json.dumps(source,indent=2))


if __name__ == "__main__":
    main()
