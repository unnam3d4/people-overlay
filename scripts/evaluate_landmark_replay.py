#!/usr/bin/env python3
"""Score manually labelled face-point presence and track identity in real video.

This is NOT bounding-box precision/recall. A CVAT face landmark can be matched
to a full human detection whose rectangle covers the landmark (one-to-one).
Unlabelled boxes are NOT counted as false positives; full-body geometry, ghost
TTL and detector precision cannot be measured from point annotations.
"""
import argparse
import json
from pathlib import Path
from evaluate_pixel_benchmark import stats


def point_in_box(p, box):
    x,y = p["x"],p["y"]
    return (box["X"] - 2 <= x <= box["X"] + box["W"] + 2 and
            box["Y"] - 2 <= y <= box["Y"] + box["H"] + 2)


def score(report):
    total = covered = switches = 0
    last_id = {}
    episodes = {}
    visits = set()
    for frame in report["frames"]:
        marks = frame.get("landmarks") or []
        tracks = frame.get("tracks") or []
        now = frame["timestamp_ms"]
        current = {m["id"] for m in marks}
        # For each ID, count a fresh appearance after a real period absent.
        for key in list(visits):
            if key not in current:
                visits.remove(key)
                last_id.pop(key, None)
        for m in marks:
            if m["id"] not in visits:
                visits.add(m["id"])
                episodes.setdefault(m["id"], []).append({"start_ms":now,"hit_ms":None})
        total += len(marks)
        choices = []
        for a, mark in enumerate(marks):
            for b, track in enumerate(tracks):
                box = track["Box"]
                if not point_in_box(mark,box): continue
                # Face expected in upper portion of human rectangle, but can
                # appear elsewhere during occlusion or odd camera perspective.
                dx = mark["x"] - (box["X"] + box["W"]*.5)
                dy = mark["y"] - (box["Y"] + box["H"]*.25)
                choices.append((dx*dx+dy*dy, a, b))
        used_m,used_t=set(),set()
        for _,a,b in sorted(choices):
            if a in used_m or b in used_t:continue
            used_m.add(a);used_t.add(b)
            mark,track = marks[a],tracks[b]
            covered += 1
            identity=mark["id"]
            ep=episodes[identity][-1]
            if ep["hit_ms"] is None:
                ep["hit_ms"] = now
            previous=last_id.get(identity)
            if previous is not None and previous!=track["ID"]:
                switches+=1
            last_id[identity]=track["ID"]
    all_episodes=[ep for all_for_person in episodes.values() for ep in all_for_person]
    latency=[ep["hit_ms"]-ep["start_ms"] for ep in all_episodes if ep["hit_ms"] is not None]
    return {"name":report["manifest"]["name"],
            "frames":len(report["frames"]),
            "manually_labelled_face_points":total,
            "covered_face_points":covered,
            "face_landmark_coverage":covered/total if total else None,
            "id_switches_on_matched_face_landmarks":switches,
            "appearances":len(all_episodes),
            "appearances_never_matched":sum(ep["hit_ms"] is None for ep in all_episodes),
            "time_to_first_face_coverage_ms":stats(latency),
            "metric_scope":"Manually annotated face-point containment by one full-body track, one-to-one. NOT box Precision/Recall, GT body bbox IoU, ghost TTL or real RVi accuracy. Four feed grid is synthesized from an authentic time-shifted video."}


def compare(rows, max_drop=.05):
    by_name={}
    for row in rows:
        label=Path(row["report_file"].replace("\\","/")).name.split("-",1)[0]
        if label in ("v19","v20"):
            by_name.setdefault(row["name"],{})[label]=row
    failures=[]
    if not by_name:
        failures.append("No real video pairs")
    for name,pair in sorted(by_name.items()):
        if set(pair)!=set(("v19","v20")):
            failures.append(f"{name}: missing reference or candidate")
            continue
        a,b=pair["v19"],pair["v20"]
        if a["face_landmark_coverage"] is None or b["face_landmark_coverage"] is None:
            failures.append(f"{name}: missing annotated face coverage")
        elif b["face_landmark_coverage"]<a["face_landmark_coverage"]-max_drop:
            failures.append(f"{name}: face coverage {a['face_landmark_coverage']:.3f} -> {b['face_landmark_coverage']:.3f}")
        if b["id_switches_on_matched_face_landmarks"]>a["id_switches_on_matched_face_landmarks"]+2:
            failures.append(f"{name}: real-video ID switches increased")
    return failures


def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument("reports",type=Path,nargs="+")
    p.add_argument("--output",type=Path,required=True)
    p.add_argument("--gate",action="store_true")
    p.add_argument("--max-coverage-drop",type=float,default=.05)
    a=p.parse_args()
    values=[]
    for path in a.reports:
        result=score(json.loads(path.read_text(encoding="utf-8-sig")))
        result["report_file"]=str(path)
        values.append(result)
        print(path.name, "face landmark coverage", result["face_landmark_coverage"],
              "switches", result["id_switches_on_matched_face_landmarks"], flush=True)
    a.output.parent.mkdir(parents=True,exist_ok=True)
    failures=compare(values,a.max_coverage_drop)
    a.output.write_text(json.dumps({"results":values,"regression_findings":failures,
                                    "passed":not failures,"note":"Landmark-only benchmark cannot assert person bbox precision/recall."},indent=2),encoding="utf-8")
    if a.gate and failures:
        for problem in failures: print("LANDMARK REGRESSION",problem)
        raise SystemExit(1)


if __name__=="__main__":main()
