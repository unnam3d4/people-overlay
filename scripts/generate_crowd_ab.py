#!/usr/bin/env python3
"""Controlled large-person benchmark. Synthetic photographic composition, NOT CCTV evidence.
Repeated, manually bounded real pedestrian photograph cutouts, deterministic GT.
Both ORIGINAL offline EXEs run identical 3200x1800 pixels and replay schedule.
"""
from pathlib import Path
import argparse, json, hashlib
from PIL import Image, ImageDraw
SOURCE = [(50,398,246,904),(220,403,349,862),(667,397,810,877)]
W,H=3200,1800
def build(out, scene, people_per_tile, grid, fps=15, duration=10):
    src=Image.open(Path(__file__).parents[1]/"src/testdata/bus-reference.jpg").convert("RGB")
    crops=[src.crop(b) for b in SOURCE]
    img=Image.new("RGB",(W,H),(90,98,98));draw=ImageDraw.Draw(img)
    truth=[]
    tw,th=W//grid,H//grid
    for tile in range(grid*grid):
        ox,oy=(tile%grid)*tw,(tile//grid)*th
        draw.rectangle((ox,oy,ox+tw-2,oy+th-2),fill=(96+(tile*11)%18,102+(tile*7)%12,106+(tile*13)%17))
        draw.rectangle((ox+4,oy+4,ox+tw-6,oy+th-6),outline=(62,67,71),width=4)
        for k in range(people_per_tile):
            # Place all full bodies without overlap. Keep image content and annotations aligned.
            source_id=(k+tile)%3
            crop=crops[source_id]
            body_h=(190 + (k%3)*23) if grid==4 else (285+(k%3)*35)
            body_w=round(crop.width*body_h/crop.height)
            # Distinct positions within each video tile.
            x=ox+int((k+1)*tw/(people_per_tile+1))-body_w//2
            y=oy+int(th*.43)+((k+tile)%3)*15
            x=max(ox+10,min(ox+tw-body_w-10,x))
            y=max(oy+10,min(oy+th-body_h-10,y))
            img.paste(crop.resize((body_w,body_h),Image.Resampling.LANCZOS),(x,y))
            truth.append({"id":f"tile{tile+1}_person{k+1}","box":{"X":x,"Y":y,"W":body_w,"H":body_h}})
    name=f"{scene}.png"
    img.save(out/name,optimize=False,compress_level=4)
    manifest={"name":scene,"source":{
        "type":"synthetic photographic composite: NOT original CCTV footage",
        "photo":"embedded Ultralytics bus-reference.jpg, same AGPL attribution in src/testdata/SOURCE.md",
        "gt":"manual source crop bounds repeated with exact affine transform",
        "image_sha256":hashlib.sha256((out/name).read_bytes()).hexdigest(),
        "limitations":"same 3 photographed people duplicated; no genuine independent people/camera variation"},
        "fps":fps,"profile":3 if grid==4 else 0,"threads":1,"sensitivity":2,
        "capture_cadence_ms":115,
        "frames":[{"file":name,"people":truth,"label":"static_people_synthetic"} for _ in range(fps*duration)]}
    (out/f"{scene}.json").write_text(json.dumps(manifest),encoding="utf8")
    print(f"{scene}: {len(truth)} annotated people, {W}x{H}, {fps*duration} frames")
def main():
    p=argparse.ArgumentParser()
    p.add_argument("--out",type=Path,required=True)
    x=p.parse_args();x.out.mkdir(parents=True,exist_ok=True)
    build(x.out,"one_camera_12_people",12,1,duration=8)
    build(x.out,"grid_4x4_48_people",3,4,duration=12)
if __name__=="__main__":main()
