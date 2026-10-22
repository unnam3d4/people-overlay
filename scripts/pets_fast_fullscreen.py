#!/usr/bin/env python3
"""Same real PETS2009 frame pixels, full-body ground truth and YOLO model.
Only fullscreen capture cadence / AI throttle differs. Not training."""
import json,argparse
from pathlib import Path
if __name__=="__main__":
    p=argparse.ArgumentParser()
    p.add_argument("--fixtures",type=Path,required=True)
    a=p.parse_args()
    for kind in ("single_less_crowded","single_crowded"):
        source=a.fixtures/f"pets2009_{kind}_15fps.json"
        data=json.loads(source.read_text(encoding="utf8"))
        assert data["profile"]==0
        data["name"]+="_fast_fullscreen"
        data["fast_fullscreen"]=True
        data.setdefault("source",{})["experiment"]="Capture 85ms and no artificial neural idle on fullscreen camera"
        dest=a.fixtures/f"fast_pets2009_{kind}_15fps.json"
        dest.write_text(json.dumps(data),encoding="utf8")
        print(dest)
