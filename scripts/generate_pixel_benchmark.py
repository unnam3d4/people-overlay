"""Reproducible photographic-composite PNG fixture, NOT real CCTV footage.

python generate_pixel_benchmark.py --source ../reference/bus.jpg --output ../pixel-fixture
No detector outputs are used to generate annotations. The manually inspected
source rectangles are transformed with the same integer placement as the pixels.
"""
import argparse
import hashlib
import json
import math
from pathlib import Path
from PIL import Image, ImageDraw, ImageEnhance

BOXES = [(50, 398, 246, 904), (220, 403, 349, 862), (667, 397, 810, 877)]


def generate(source, output, seconds=24, scenarios=None):
    output.mkdir(parents=True, exist_ok=True)
    frame_dir = output / "frames"
    frame_dir.mkdir(exist_ok=True)
    original = Image.open(source).convert("RGB")
    crops = [original.crop(box) for box in BOXES]
    cache = {}
    source_info = {
        "url": "https://github.com/ultralytics/ultralytics/blob/main/ultralytics/assets/bus.jpg",
        "license": "AGPL-3.0 repository distribution; source photograph attribution is not separately specified",
        "sha256": hashlib.sha256(source.read_bytes()).hexdigest(),
        "notes": "Synthetic photographic rectangular cutouts on procedural backgrounds, not real CCTV. Manually inspected source boxes; visible-person geometric ground truth. No YOLO-derived labels. Partial occlusion bbox scoring is approximate. 3rd source person is clipped at the source right edge.",
        "source_boxes_xyxy": json.dumps(BOXES),
    }

    def background(grid):
        image = Image.new("RGB", (960, 540), (76, 81, 84))
        draw = ImageDraw.Draw(image)
        cols = 4 if grid else 1
        tw, th = 960 // cols, 540 // cols
        for row in range(cols):
            for col in range(cols):
                x, y = col * tw, row * th
                draw.rectangle((x+1, y+1, x+tw-2, y+th-2), fill=(115+row*4, 120+col*3, 119))
                draw.polygon([(x,y+th),(x+tw,y+th),(x+tw*.7,y+th*.55),(x+tw*.3,y+th*.55)], fill=(81,85,87))
                for n in range(1, 5):
                    yy = int(y + th * (.55 + n*.09))
                    draw.line((x, yy, x+tw, yy), fill=(101,105,107))
                draw.rectangle((x+tw*.08,y+th*.12,x+tw*.27,y+th*.55), fill=(62,67,69))
        return image

    def place(image, people, identity, crop_id, x, y, height):
        photo = crops[crop_id]
        width = round(photo.width * height / photo.height)
        x, y = round(x), round(y)
        image.paste(photo.resize((width, height), Image.Resampling.LANCZOS), (x, y))
        people.append({"id": identity, "box": {"X": x, "Y": y, "W": width, "H": height}})

    def covered_fraction(a, b):
        width = max(0, min(a["X"]+a["W"], b["X"]+b["W"])-max(a["X"],b["X"]))
        height = max(0, min(a["Y"]+a["H"], b["Y"]+b["H"])-max(a["Y"],b["Y"]))
        return width*height/(a["W"]*a["H"])

    names = scenarios or ["single_lifecycle", "single_crossing", "grid_coverage", "grid_disturbance"]
    for scenario in names:
        grid = scenario.startswith("grid")
        base = background(grid)
        frames = []
        previous = None
        for index in range(seconds*30):
            t = index/30
            image, people = base.copy(), []
            label = "empty"
            if scenario == "single_lifecycle":
                if 1 <= t < 12:
                    x = 230 if t < 8 else 230+(t-8)*35
                    place(image, people, "person1", 0, x, 245, 175)
                    label = "stationary" if t < 8 else "moving"
                elif 19 <= t < 23:
                    place(image, people, "person1", 0, 530, 245, 175)
                    label = "reentry"
                elif 12 <= t < 19:
                    label = "disappeared"
            elif scenario == "single_crossing":
                if 1 <= t < 18:
                    x = 170+min(t-1, 15)*31
                    place(image, people, "person1", 0, x, 240, 180)
                    place(image, people, "person2", 1, 720-min(t-1, 15)*31, 255, 165)
                    label = "crossing"
                    if 8 <= t < 10:
                        # A foreground opaque object fully covers person1.
                        b = people[0]["box"]
                        ImageDraw.Draw(image).rectangle((b["X"],b["Y"],b["X"]+b["W"]-1,b["Y"]+b["H"]-1), fill=(61,67,71))
                        people = [p for p in people[1:] if covered_fraction(p["box"],b)<.5]
                        label = "occlusion"
                    elif covered_fraction(people[0]["box"], people[1]["box"])>=.5:
                        people = people[1:]
            elif scenario == "grid_coverage":
                if 1 <= t < 16:
                    for tile in range(16):
                        place(image, people, f"tile{tile+1}", tile % 3, (tile%4)*240+95, (tile//4)*135+60, 55)
                    label = "sixteen_stationary_people"
                elif t >= 16:
                    label = "all_disappeared"
            elif scenario == "grid_disturbance":
                if 1 <= t < 16 or 20 <= t < 23:
                    for tile in (0, 5, 10, 15):
                        shift = round(22 * math.sin(t*.7+tile))
                        jitter = ((index//5)%3)-1 if 6 <= t < 10 else 0
                        place(image, people, f"tile{tile+1}", tile%3, (tile%4)*240+90+shift+jitter, (tile//4)*135+55+jitter, 59)
                    label = "moving"
                    if 6 <= t < 10: label = "subject_jitter"
                    if 10 <= t < 14:
                        image = ImageEnhance.Brightness(image).enhance(.55)
                        label = "low_light"
                    if 14 <= t < 16 and index%3 and previous:
                        frames.append(dict(previous, label="repeated_frame"))
                        continue
                elif t >= 16: label = "disappeared"
            else:
                raise ValueError(scenario)
            digest = hashlib.sha256(image.tobytes()).hexdigest()[:24]
            filename = f"frames/{digest}.png"
            if digest not in cache:
                image.save(output/filename, compress_level=2)
                cache[digest] = True
            previous = {"file": filename, "people": people, "label": label}
            frames.append(previous)
        for fps in (15, 30):
            manifest = {"name": f"{scenario}_{fps}fps", "source": source_info, "fps": fps,
                        "profile": 3 if grid else 0, "sensitivity": 2, "threads": 1,
                        "frames": frames[::30//fps]}
            # Baseline measured timing, plus stress at 300/700 ms video-clock
            # lower bound. These simulate slow CPU inference *timing*, never
            # pretend to measure actual CPU or sleeping inference calls.
            for floor in (0, 300, 700):
                variant = dict(manifest)
                if floor:
                    variant["inference_delay_floor_ms"] = floor
                label = f"{scenario}_{floor}ms_{fps}fps" if floor else f"{scenario}_{fps}fps"
                variant["name"] = label
                path = output/f"{label}.json"
                path.write_text(json.dumps(variant, indent=2), encoding="utf-8")
                print(path)
    (output/"SOURCE.json").write_text(json.dumps(source_info, indent=2), encoding="utf-8")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--seconds", type=int, default=24)
    parser.add_argument("--scenario", action="append")
    args = parser.parse_args()
    generate(args.source, args.output, args.seconds, args.scenario)
