#!/usr/bin/env python3
"""Turn real, annotated CAVIAR CCTV videos into the existing offline pixel replay.

These are *real filmed people*, not photographic cutouts. The 4x4 test is a
composite of time-offset video feeds, NOT a recording of RVi itself.
Video/XML are downloaded from the official Edinburgh CAVIAR archive during
validation, never bundled into the app or checked into GitHub.
"""
import argparse
import hashlib
import json
from pathlib import Path
import shutil
import time
import urllib.request
import xml.etree.ElementTree as ET

BASES = (
    "https://groups.inf.ed.ac.uk/vision/DATASETS/CAVIAR/CAVIARDATA2",
    "https://homepages.inf.ed.ac.uk/rbf/CAVIARDATA2",
)
CLIPS = {
    "EnterExitCrossingPaths2cor": "ceecp2gt.xml",
    "OneLeaveShopReenter1cor": "colsr1gt.xml",
}


def annotations(xml_bytes):
    """CAVIAR CVML: objectlist/entitylist, centered xc/yc or top-left x/y."""
    root = ET.fromstring(xml_bytes)
    frames = {}
    for frame in root.iter("frame"):
        if frame.get("number") is None:
            continue
        n = int(frame.get("number"))
        objects = []
        for tag in ("objectlist/object", "entitylist/entity"):
            for obj in frame.findall(tag):
                box = obj.find("box")
                if box is None:
                    continue
                try:
                    bw, bh = float(box.get("w")), float(box.get("h"))
                    if "xc" in box.attrib and "yc" in box.attrib:
                        x = float(box.get("xc")) - bw / 2
                        y = float(box.get("yc")) - bh / 2
                    else:
                        x, y = float(box.get("x")), float(box.get("y"))
                except (TypeError, ValueError):
                    continue
                if bw <= 0 or bh <= 0:
                    continue
                # Person identities belong to one source sequence only.
                objects.append((str(obj.get("id", "?")), (x, y, bw, bh)))
        frames[n] = objects
    if not frames:
        raise ValueError("CAVIAR XML contained no numbered frames")
    if not any(frames.values()):
        raise ValueError("CAVIAR XML contained no annotated human objects")
    return frames


def sha(data):
    return hashlib.sha256(data).hexdigest()


def fetch_clip(name, xml_name, output):
    output.mkdir(parents=True, exist_ok=True)
    paths = (name + ".mpg", xml_name)
    for filename in paths:
        dest = output / filename
        if dest.is_file() and dest.stat().st_size > 2000:
            continue
        errors = []
        for base in BASES:
            url = f"{base}/{name}/{filename}"
            try:
                with urllib.request.urlopen(urllib.request.Request(
                    url, headers={"User-Agent": "PeopleOverlay-CAVIAR-Research/1.0"}
                ), timeout=35) as response:
                    content = response.read(25_000_001)
                if not 2_000 < len(content) <= 25_000_000:
                    raise ValueError(f"invalid download length {len(content)}")
                if filename.endswith(".xml") and b"<" not in content[:256]:
                    raise ValueError("not XML")
                if filename.endswith(".mpg") and content[:1] == b"<":
                    raise ValueError("server served HTML rather than MPEG")
                dest.write_bytes(content)
                print(f"CAVIAR fetched {url} bytes={len(content)} SHA256={sha(content)}", flush=True)
                break
            except Exception as error:
                errors.append(f"{url}: {error}")
                time.sleep(1)
        else:
            raise RuntimeError("Unable to fetch required official CAVIAR resource: " + "; ".join(errors))
    return output / paths[0], output / paths[1]


def clamp_box(box, ow, oh, xoffset, yoffset, tw, th):
    x, y, bw, bh = box
    x1 = max(0., min(float(ow), x))
    y1 = max(0., min(float(oh), y))
    x2 = max(0., min(float(ow), x + bw))
    y2 = max(0., min(float(oh), y + bh))
    if x2 <= x1 or y2 <= y1:
        return None
    # Use exactly the same affine transform used by cv2.resize.
    return {"X": xoffset + x1 * tw / ow, "Y": yoffset + y1 * th / oh,
            "W": (x2 - x1) * tw / ow, "H": (y2 - y1) * th / oh}


def choose_start(boxes, max_frame, fps, duration):
    """Choose a crowded event for crossing; never invent an event label."""
    if not boxes:
        return 0
    best = max(boxes, key=lambda i: (len(boxes[i]), -i))
    half = int(duration * fps * .5)
    return max(0, min(best - half, max(0, max_frame - int(duration * fps))))


def generate(name, video, xml, output, duration=10, frame_rates=(15,)):
    # Imported only for fixture generation on CI builder, never target PC.
    import cv2
    import numpy as np
    gt = annotations(xml.read_bytes())
    reader = cv2.VideoCapture(str(video))
    if not reader.isOpened():
        raise RuntimeError(f"CAVIAR video could not be decoded: {video}")
    original_fps = reader.get(cv2.CAP_PROP_FPS)
    if not 10 <= original_fps <= 60:
        raise RuntimeError(f"unexpected source fps {original_fps}")
    total = int(reader.get(cv2.CAP_PROP_FRAME_COUNT))
    if total < 100:
        raise RuntimeError(f"CAVIAR source too short ({total} frames)")
    original_w, original_h = int(reader.get(3)), int(reader.get(4))
    if (original_w, original_h) != (384, 288):
        raise RuntimeError(f"unexpected CAVIAR image size {original_w}x{original_h}")
    offset = 1 if min(gt) == 1 else 0
    start = choose_start(gt, total, original_fps, duration)
    # Decode once per source frame, then sample at deterministic replay times.
    end = min(total, start + int(duration * original_fps) + 1)
    reader.set(cv2.CAP_PROP_POS_FRAMES, start)
    originals = []
    for idx in range(start, end):
        ok, frame = reader.read()
        if not ok:
            break
        originals.append(cv2.cvtColor(frame, cv2.COLOR_BGR2RGB))
    reader.release()
    if len(originals) < int(duration * original_fps * .8):
        raise RuntimeError("Truncated CAVIAR source video")
    frames_dir = output / "frames"
    frames_dir.mkdir(parents=True, exist_ok=True)
    info = {"url": f"{BASES[0]}/{name}/{name}.mpg",
            "annotation_url": f"{BASES[0]}/{name}/{xml.name}",
            "video_sha256": sha(video.read_bytes()), "xml_sha256": sha(xml.read_bytes()),
            "license": "CAVIAR EC project dataset, CC BY-SA; attribution required",
            "notes": "Real CAVIAR video, manually annotated CVML, offline pixel replay. 4x4 is a synthetic arrangement of real video frames, NOT actual RVi footage.",
            "source_fps": str(original_fps), "start_source_frame": str(start), "xml_index_offset": str(offset)}
    manifests = []
    for grid in (False, True):
        for fps in frame_rates:
            manifest_frames = []
            # Avoid placing a person into a tile and then forgetting to label
            # their appearance: each nonempty feed owns its GT.
            tiles = (0, 5, 10, 15) if grid else (0,)
            for index in range(int(duration * fps)):
                screen = np.zeros((540, 960, 3), dtype=np.uint8)
                screen[:, :, :] = (78, 80, 83)
                people = []
                for tile in tiles:
                    shift_seconds = (tile % 4) * 1.2 if grid else 0.
                    relative = min(len(originals) - 1, int((index / fps + shift_seconds) * original_fps))
                    source_idx = start + relative
                    image = originals[relative]
                    xoff = (tile % 4) * 240 if grid else 0
                    yoff = (tile // 4) * 135 if grid else 0
                    tw, th = (240, 135) if grid else (960, 540)
                    resized = cv2.resize(image, (tw, th), interpolation=cv2.INTER_AREA)
                    screen[yoff:yoff+th, xoff:xoff+tw] = resized
                    for person_id, bbox in gt.get(source_idx + offset, []):
                        adjusted = clamp_box(bbox, original_w, original_h, xoff, yoff, tw, th)
                        if adjusted is not None:
                            people.append({"id": f"{name}/tile{tile}/{person_id}", "box": adjusted})
                # Stored by frame index rather than pseudo-random hash so
                # frame correspondence and reentry can be audited.
                image_name = f"{name}-{'grid4' if grid else 'single'}-{fps}-{index:04d}.png"
                cv2.imwrite(str(frames_dir / image_name), cv2.cvtColor(screen, cv2.COLOR_RGB2BGR), [cv2.IMWRITE_PNG_COMPRESSION, 2])
                manifest_frames.append({"file": "frames/" + image_name, "people": people,
                                        "label": "real_CAVIAR_grid_4_sources" if grid else "real_CAVIAR_single"})
            if not any(f["people"] for f in manifest_frames):
                raise RuntimeError(f"no annotated people for {name}, grid={grid}")
            tag = f"caviar_{name}_{'grid4' if grid else 'single'}_{fps}fps"
            manifest = {"name": tag, "source": info, "fps": fps,
                        "profile": 3 if grid else 0, "sensitivity": 2, "threads": 1,
                        "frames": manifest_frames}
            location = output / (tag + ".json")
            location.write_text(json.dumps(manifest, separators=(",", ":")), encoding="utf-8")
            manifests.append(location)
            print(f"{tag}: frames={len(manifest_frames)} annotated_boxes={sum(len(f['people']) for f in manifest_frames)}", flush=True)
    (output / "SOURCE-CAVIAR.json").write_text(json.dumps(info, indent=2))
    return manifests


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--output", type=Path, required=True)
    p.add_argument("--cache", type=Path, required=True)
    p.add_argument("--duration", type=int, default=10)
    p.add_argument("--clip", action="append", choices=CLIPS.keys())
    a = p.parse_args()
    if not 4 <= a.duration <= 20:
        raise SystemExit("duration must be 4..20 seconds")
    a.output.mkdir(parents=True, exist_ok=True)
    for clip in (a.clip or list(CLIPS)):
        movie, gt = fetch_clip(clip, CLIPS[clip], a.cache / clip)
        generate(clip, movie, gt, a.output, duration=a.duration)


if __name__ == "__main__":
    main()
