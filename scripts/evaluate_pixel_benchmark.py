"""Score raw pixel-replay reports. All missed episodes and censored tails remain explicit.

python evaluate_pixel_benchmark.py baseline.json candidate.json --output comparison.json
"""
import argparse
import json
import math
from pathlib import Path


def iou(a, b):
    inter = max(0, min(a["X"]+a["W"], b["X"]+b["W"])-max(a["X"], b["X"])) * max(0, min(a["Y"]+a["H"], b["Y"]+b["H"])-max(a["Y"], b["Y"]))
    union = a["W"]*a["H"]+b["W"]*b["H"]-inter
    return inter/union if union > 0 else 0


def match_boxes(truth, predicted, threshold=.5):
    """Hungarian one-to-one assignment: maximum cardinality, then maximum IoU."""
    if not truth or not predicted:
        return []
    n = max(len(truth), len(predicted))
    overlaps = [[iou(a, b) for b in predicted] for a in truth]
    cost = [[0.0]*n for _ in range(n)]
    for a in range(len(truth)):
        for b in range(len(predicted)):
            if overlaps[a][b] >= threshold:
                cost[a][b] = -(n+1+overlaps[a][b])
    u, v, p, way = [0.]*(n+1), [0.]*(n+1), [0]*(n+1), [0]*(n+1)
    for a in range(1,n+1):
        p[0] = a
        j0 = 0
        minimum, used = [math.inf]*(n+1), [False]*(n+1)
        while True:
            used[j0] = True
            i0, delta, j1 = p[j0], math.inf, 0
            for j in range(1,n+1):
                if not used[j]:
                    cur = cost[i0-1][j-1]-u[i0]-v[j]
                    if cur < minimum[j]: minimum[j], way[j] = cur, j0
                    if minimum[j] < delta: delta, j1 = minimum[j], j
            for j in range(n+1):
                if used[j]: u[p[j]] += delta; v[j] -= delta
                else: minimum[j] -= delta
            j0 = j1
            if p[j0] == 0: break
        while True:
            j1 = way[j0]
            p[j0] = p[j1]
            j0 = j1
            if j0 == 0: break
    return [(p[j]-1,j-1) for j in range(1,n+1)
            if p[j] and p[j]<=len(truth) and j<=len(predicted) and overlaps[p[j]-1][j-1]>=threshold]


def stats(values):
    values = sorted(values)
    def percentile(f):
        if not values: return None
        at = (len(values)-1)*f
        lo, hi = math.floor(at), math.ceil(at)
        return values[lo]+(values[hi]-values[lo])*(at-lo)
    return {"count":len(values), "mean":sum(values)/len(values) if values else None,
            "p50":percentile(.5), "p95":percentile(.95), "max":max(values) if values else None}


def pr(tp, fp, fn):
    return {"true_positive_boxes":tp, "false_positive_boxes":fp, "false_negative_boxes":fn,
            "precision":tp/(tp+fp) if tp+fp else None, "recall":tp/(tp+fn) if tp+fn else None}


def evaluate(report, threshold=.5):
    frames = report["frames"]
    tp = fp = fn = switches = duplicates = 0
    active, episodes, disappearances = {}, [], []
    drift, normalized_drift = [], []
    all_track_ids = set()
    phase_counts = {}
    for frame in frames:
        now = frame["timestamp_ms"]
        people, tracks = frame.get("people") or [], frame.get("tracks") or []
        visible = {person["id"] for person in people}
        present_track_ids = {track["ID"] for track in tracks}
        all_track_ids |= present_track_ids
        for identity in list(active):
            if identity not in visible:
                ep = active.pop(identity)
                ep["end_ms"] = now
                if ep["last_track_id"] is not None:
                    disappearances.append({"person":identity, "start_ms":now, "track_id":ep["last_track_id"], "clear_ms":None, "censored":False})
        for disappearance in disappearances:
            if disappearance["clear_ms"] is None and not disappearance["censored"]:
                if disappearance["track_id"] not in present_track_ids:
                    disappearance["clear_ms"] = now
                elif disappearance["person"] in visible:
                    disappearance["censored"] = True
        for person in people:
            identity = person["id"]
            if identity not in active:
                ep = {"person":identity,"start_ms":now,"end_ms":None,"first_track_ms":None,"last_track_id":None,"matched_frames":0,"visible_frames":0}
                active[identity] = ep
                episodes.append(ep)
            active[identity]["visible_frames"] += 1
        pairs = match_boxes([p["box"] for p in people], [p["Box"] for p in tracks], threshold)
        tp += len(pairs); fp += len(tracks)-len(pairs); fn += len(people)-len(pairs)
        counts = phase_counts.setdefault(frame.get("label","unlabeled"), [0,0,0])
        counts[0] += len(pairs); counts[1] += len(tracks)-len(pairs); counts[2] += len(people)-len(pairs)
        matched_prediction = {b for _,b in pairs}
        duplicates += sum(1 for j,t in enumerate(tracks) if j not in matched_prediction and any(iou(t["Box"],p["box"])>=threshold for p in people))
        for a,b in pairs:
            person, track = people[a], tracks[b]
            ep = active[person["id"]]
            if ep["first_track_ms"] is None: ep["first_track_ms"] = now
            if ep["last_track_id"] is not None and ep["last_track_id"] != track["ID"]: switches += 1
            ep["last_track_id"] = track["ID"]
            ep["matched_frames"] += 1
            g,p = person["box"],track["Box"]
            error = math.hypot(g["X"]+g["W"]/2-p["X"]-p["W"]/2,g["Y"]+g["H"]/2-p["Y"]-p["H"]/2)
            drift.append(error); normalized_drift.append(error/max(1,math.hypot(g["W"],g["H"])))
    interval = 1000/report["manifest"].get("fps",30)
    end = frames[-1]["timestamp_ms"]+interval if frames else 0
    for ep in episodes:
        if ep["end_ms"] is None: ep["end_ms"] = end
        ep["tt_first_track_ms"] = None if ep["first_track_ms"] is None else ep["first_track_ms"]-ep["start_ms"]
    for disappearance in disappearances:
        if disappearance["clear_ms"] is None: disappearance["censored"] = True
        disappearance["ghost_ms"] = None if disappearance["clear_ms"] is None else disappearance["clear_ms"]-disappearance["start_ms"]
    inferences = report.get("inferences") or []
    dtp = dfp = dfn = 0
    for event in inferences:
        gt = frames[event["capture_frame"]].get("people") or []
        roi = event["region"]
        cropped = []
        for p in gt:
            b = p["box"]
            x,y = max(b["X"],roi["X"]),max(b["Y"],roi["Y"])
            w,h = min(b["X"]+b["W"],roi["X"]+roi["W"])-x,min(b["Y"]+b["H"],roi["Y"]+roi["H"])-y
            if w>0 and h>0 and w*h >= .5*b["W"]*b["H"]:
                cropped.append({"X":x,"Y":y,"W":w,"H":h})
        boxes = event.get("detections") or []
        pairs = match_boxes(cropped,boxes,threshold)
        dtp += len(pairs); dfp += len(boxes)-len(pairs); dfn += len(cropped)-len(pairs)
    stage_names = ("load_ms","scheduler_ms","tracking_ms","apply_ms")
    stage = {name:stats([f.get("times",{}).get(name,0) for f in frames if f.get("captured")]) for name in stage_names}
    for name in ("copy_ms","prepare_ms","infer_ms","decode_ms"):
        stage[name] = stats([event.get("times",{}).get(name,0) for event in inferences])
    stage["delivery_age_ms"] = stats([event["applied_ms"]-event["captured_ms"] for event in inferences if event.get("applied_ms") is not None])
    track_metrics = pr(tp,fp,fn)
    track_metrics.update(id_switches=switches, duplicate_boxes=duplicates, unique_track_ids=len(all_track_ids),
                         false_positive_box_seconds=fp*interval/1000, missed_person_seconds=fn*interval/1000)
    return {"name":report["manifest"].get("name"), "capture_aware":report.get("capture_aware"), "fps":report["manifest"].get("fps"),
            "frames":len(frames),"duration_ms":end,"wall_ms":report.get("wall_ms"),"inference_count":len(inferences),
            "unapplied_inferences":sum(e.get("applied_ms") is None for e in inferences),
            "scene_resets":sum(bool(f.get("scene_reset")) for f in frames),
            "tracking":track_metrics,"detector_on_captured_roi":pr(dtp,dfp,dfn),
            "by_phase":{name:pr(*counts) for name,counts in phase_counts.items()},
            "time_to_first_track_ms":stats([e["tt_first_track_ms"] for e in episodes if e["tt_first_track_ms"] is not None]),
            "episodes_total":len(episodes),"episodes_missed":sum(e["first_track_ms"] is None for e in episodes),"episodes":episodes,
            "ghost_after_disappearance_ms":stats([d["ghost_ms"] for d in disappearances if d["ghost_ms"] is not None]),
            "ghost_censored_events":sum(d["censored"] for d in disappearances),"ghost_events":disappearances,
            "matched_center_error_px":stats(drift),"matched_center_error_box_diagonal_fraction":stats(normalized_drift),"stage_times":stage,
            "method":"Hungarian one-to-one maximum-cardinality then max-IoU assignment at IoU >= "+str(threshold)+"; ID switches within continuous GT visibility episodes including temporary tracking misses; full reentry starts a new episode. TT-first-track excludes misses from percentiles but reports all missed episodes. Ghost duration follows the last matched track ID until removal, censors reentry/end. Center error only on matches. ROI GT includes >=50% visible area and is clipped. Counts are frame-weighted. Offline PNG replay excludes disk time from simulated inference latency and does not measure concurrent contention or real capture/display latency."}


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("reports", type=Path, nargs="+")
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--iou", type=float, default=.5)
    args = parser.parse_args()
    result = []
    for path in args.reports:
        scored = evaluate(json.loads(path.read_text(encoding="utf-8-sig")), args.iou)
        scored["report_file"] = str(path)
        result.append(scored)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(result,indent=2),encoding="utf-8")
    for r in result:
        print(r["name"],"P/R",r["tracking"]["precision"],r["tracking"]["recall"],"ID switches",r["tracking"]["id_switches"],"missed episodes",r["episodes_missed"])
