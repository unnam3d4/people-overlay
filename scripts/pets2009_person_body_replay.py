#!/usr/bin/env python3
"""Reproducible PETS2009 *real human BODY* GT benchmark for People Overlay.

Research video frames (S2/L1 Time_12-34 View_001) from public Kaggle mirror.
Manual CVML full-body rectangles + IDs from OpenTraj mirror of PETS2009.
Original 768x576 / ~7 FPS. Re-sampling at 15 FPS repeats original frames;
it is NOT footage acquired natively at 15fps. Composites replicate existing
real video across independent offset streams; not authentic simultaneous RVi.
App offline EXEs don't fetch data, only CI machine does.

Source: PETS2009, University of Reading, PETS challenge research dataset.
Do not redistribute third-party frames; respect applicable dataset terms.
"""
import argparse
import hashlib
import json
from pathlib import Path
import re
import urllib.request
import xml.etree.ElementTree as ET

KAGGLE_ID="yeeandres/pets2009"
GT_URL="https://raw.githubusercontent.com/crowdbotp/OpenTraj/master/datasets/PETS-2009/data/annotations/PETS2009-S2L1.xml"
GT_SHA256="4a91380025ef20254e2157fa14f4a705e668b41d77869d330cb1c6e2a53bd9e2"
SOURCE_FPS=7.0
WIDTH,HEIGHT=768,576
OUTPUT_W,OUTPUT_H=960,540


def xml_objects(content):
    """PETS CVML gives ground truth *whole-person* center boxes and IDs."""
    root=ET.fromstring(content)
    result={}
    for frame in root.findall(".//frame"):
        if "number" not in frame.attrib: continue
        index=int(frame.attrib["number"])
        detections=[]
        for obj in frame.findall(".//object"):
            box=obj.find("box")
            if box is None: continue
            try:
                xc=float(box.get("xc"))
                yc=float(box.get("yc"))
                w=float(box.get("w"))
                h=float(box.get("h"))
                pid=str(obj.get("id"))
            except (ValueError,TypeError): continue
            if w<2 or h<2 or not pid.isdigit():continue
            detections.append((pid,(xc-w*.5,yc-h*.5,w,h)))
        result[index]=detections
    if not result or not any(result.values()):
        raise ValueError("PETS2009 CVML has no whole-person boxes")
    return result


def box_rescale(box,sx,sy,xoff,yoff,clip_w,clip_h):
    """Clip to original frame first; video mosaic and GT use same affine."""
    x,y,w,h=box
    x1=max(0.,min(WIDTH,x))
    y1=max(0.,min(HEIGHT,y))
    x2=max(0.,min(WIDTH,x+w))
    y2=max(0.,min(HEIGHT,y+h))
    if x2<=x1 or y2<=y1: return None
    return {"X":xoff+x1*sx,"Y":yoff+y1*sy,
            "W":(x2-x1)*sx,"H":(y2-y1)*sy}


def source_frames(root):
    files=sorted((Path(root)/"Crowd_PETS09"/"S2"/"L1"/"Time_12-34"/"View_001").glob("*.jpg"))
    if len(files)!=795: raise RuntimeError(f"Expected 795 PETS frames, got {len(files)}")
    from PIL import Image
    for p in (files[0],files[-1]):
        with Image.open(p) as image:
            if image.size!=(WIDTH,HEIGHT):raise RuntimeError(f"unexpected PETS image {p}: {image.size}")
    return files


def choose_window(gt,desired_count,length=795,fps=SOURCE_FPS,duration=7):
    """Pick event-rich contiguous filmed segment with no invented boxes."""
    count=max(1,int(duration*fps))
    best_idx,best=-1,-1
    for start in range(0,length-count,12):
        total=sum(len(gt.get(i,())) for i in range(start,start+count))
        if desired_count==1:
            # A less crowded sequence evaluates single-person tracking.
            score=sum(1 for i in range(start,start+count) if len(gt.get(i,()))==1)
        else:
            score=sum(1 for i in range(start,start+count) if len(gt.get(i,()))>=desired_count)
        if score>best or (score==best and total>sum(len(gt.get(i,())) for i in range(max(0,best_idx),max(0,best_idx)+count))):
            best,best_idx=score,start
    if best_idx<0:raise ValueError("cannot identify valid PETS clip")
    return best_idx


def generate(output,video_files,gt,duration=7,fps=15):
    from PIL import Image
    import numpy as np
    output.mkdir(parents=True,exist_ok=True)
    frames_dir=output/"frames"
    frames_dir.mkdir(parents=True,exist_ok=True)
    single_start=choose_window(gt,1,duration=duration)
    crossing_start=choose_window(gt,3,duration=duration)
    # 4x4 of real filmed scenes, with either 4 or 16 video feeds active.
    variants=[
      ("single_less_crowded",[0],[single_start]),
      ("single_crowded",[0],[crossing_start]),
      ("grid4_four_feeds",[0,5,10,15],[0,120,300,500]),
      ("grid4_sixteen_feeds",list(range(16)),[(40+i*39)%680 for i in range(16)]),
    ]
    manifests=[]
    for name,tiles,start_by_tile in variants:
        grid=name.startswith("grid4")
        manifest_frames=[]
        for index in range(duration*fps):
            pixels=np.full((OUTPUT_H,OUTPUT_W,3),76,dtype=np.uint8)
            people=[]
            for tile,start in zip(tiles,start_by_tile):
                origin_idx=start+min(int(index*SOURCE_FPS/fps),int(duration*SOURCE_FPS)-1)
                if origin_idx>=len(video_files):raise RuntimeError("source frame beyond downloaded video")
                with Image.open(video_files[origin_idx]) as im:
                    frame=im.convert("RGB")
                    left,top=((tile%4)*240,(tile//4)*135) if grid else (0,0)
                    tw,th=(240,135) if grid else (OUTPUT_W,OUTPUT_H)
                    frame=frame.resize((tw,th),Image.Resampling.BILINEAR)
                    pixels[top:top+th,left:left+tw]=np.asarray(frame)
                for person_id,rect in gt.get(origin_idx,()):
                    box=box_rescale(rect,tw/WIDTH,th/HEIGHT,left,top,tw,th)
                    if box is not None and box["W"]>=2 and box["H"]>=3:
                        people.append({"id":f"tile{tile}/person{person_id}","box":box})
            frame_name=f"frames/{name}_{index:04d}.png"
            Image.fromarray(pixels).save(output/frame_name,optimize=False,compress_level=1)
            manifest_frames.append({"file":frame_name,"people":people,"label":"real_filmed_person_body_boxes"})
        if not any(f["people"] for f in manifest_frames):
            raise RuntimeError("No manual person bbox GT in "+name)
        info={"dataset":"PETS2009 S2L1 Time_12-34 View_001",
              "frames_mirror":"https://www.kaggle.com/datasets/yeeandres/pets2009",
              "manual_body_gt_mirror":GT_URL,
              "manual_body_gt_sha256":GT_SHA256,
              "citation":"PETS2009: Dataset and Challenge (University of Reading), research benchmark",
              "source_fps":str(SOURCE_FPS),
              "note":"Real 7fps CCTV frames with manual full-body bounding boxes. 15fps replay repeats source frames. Multi-feed views are time-offset composites of one actual camera, not physical 16-camera RVi footage.",
              "gt_type":"FULL PERSON BODY BBOX + ID (not faces)"}
        manifest={"name":"pets2009_"+name+"_"+str(fps)+"fps","source":info,
                  "fps":fps,"profile":3 if grid else 0,"sensitivity":2,
                  "threads":1,"frames":manifest_frames}
        dest=output/(manifest["name"]+".json")
        dest.write_text(json.dumps(manifest,separators=(",",":")),encoding="utf-8")
        manifests.append(str(dest))
        print("PETS PERSON-BODY",name,len(manifest_frames),
              "GT people instances",sum(len(f["people"]) for f in manifest_frames),flush=True)
    (output/"PETS2009_PROVENANCE.json").write_text(json.dumps({
        "dataset":"PETS2009 original human-body annotation",
        "GT_url":GT_URL,"GT_sha256":GT_SHA256,"kaggle_id":KAGGLE_ID,
        "single_start":single_start,"crossing_start":crossing_start,
        "source_fps":SOURCE_FPS,"replay_fps":fps,
        "warning":"Ground truth is actual full body, but stitched tiles and repeated 7fps samples are simulations; no actual RVi capture."},indent=2),encoding="utf-8")
    return manifests


def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument("--output",type=Path,required=True)
    p.add_argument("--duration",type=int,default=7)
    p.add_argument("--fps",type=int,default=15)
    a=p.parse_args()
    if a.duration<4 or a.duration>12 or a.fps not in (7,15,30):
        raise SystemExit("duration 4..12, fps 7/15/30")
    import kagglehub
    root=kagglehub.dataset_download(KAGGLE_ID)
    files=source_frames(root)
    request=urllib.request.Request(GT_URL,headers={"User-Agent":"PeopleOverlayCCTV-Research/1.0"})
    blob=urllib.request.urlopen(request,timeout=40).read(2_000_000)
    actual=hashlib.sha256(blob).hexdigest()
    if actual!=GT_SHA256:
        raise RuntimeError(f"PETS2009 annotation unexpectedly changed: {actual}")
    gt=xml_objects(blob)
    print("VERIFIED REAL PETS2009 BODY GT",len(files),"frames",
          sum(map(len,gt.values())),"boxes",flush=True)
    generate(a.output,files,gt,a.duration,a.fps)


if __name__=="__main__":main()
