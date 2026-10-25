#!/usr/bin/env python3
"""Fair, one-to-one body-IoU evaluator for the same labelled fixture.
Frame-weighted track matching, no tracker warm-up exclusion, p50/p95 native
latency statistics, explicit synthetic fixture / CPU environment caveats."""
import json, argparse, statistics, pathlib
def iou(a,b):
    x=max(0,min(a['X']+a['W'],b['X']+b['W'])-max(a['X'],b['X']))
    y=max(0,min(a['Y']+a['H'],b['Y']+b['H'])-max(a['Y'],b['Y']))
    i=x*y; area=a['W']*a['H']+b['W']*b['H']-i
    return i/area if area>0 else 0
def boxes(tracks):
    out=[]
    for track in tracks:
        # TrackView uses Rect? normalize robustly, never fabricate data.
        r=track.get('Rect',track.get('rect'))
        if r is None: r=track.get('Box',track.get('box'))
        if r is None: r={k:track[k] for k in ('X','Y','W','H') if k in track}
        if all(k in r for k in ('X','Y','W','H')):out.append(r)
    return out
def evaluate(path):
    d=json.loads(path.read_text());pairs=[];unmatched=0;total=0
    peak=0;sample=[]
    for frame in d['frames']:
        actual=[v['box'] for v in frame['people']]
        measured=boxes(frame['tracks'])
        peak=max(peak,len(measured))
        total+=len(actual)
        possible=[]
        for gi,gt in enumerate(actual):
            for di,det in enumerate(measured):
                match=iou(gt,det)
                if match>=.5:possible.append((match,gi,di))
        possible.sort(reverse=True)
        usedgt=set();useddet=set()
        for overlap,gi,di in possible:
            if gi not in usedgt and di not in useddet:
                usedgt.add(gi);useddet.add(di)
        pairs.extend([len(usedgt)]);unmatched+=len(measured)-len(useddet)
        sample.append({"frame":frame['frame'],"found":len(measured),"matched":len(usedgt)})
    times=[q['times']['infer_ms'] for q in d['inferences']]
    prep=[q['times']['prepare_ms'] for q in d['inferences']]
    if not times: raise RuntimeError('no inference in '+str(path))
    def percentile(v,q):
        s=sorted(v);return s[min(len(s)-1,round((len(s)-1)*q))]
    num=len(d['frames'])
    return {"model":path.stem.split("__",1)[0],"scenario":d['manifest']['name'],
      "expected_people_per_frame":len(d['manifest']['frames'][0]['people']),
      "frames":num,"inferences":len(times),
      "avg_visible_tracks":round(sum(x['found'] for x in sample)/num,2),
      "peak_visible_tracks":peak,
      "matched_people_total":sum(pairs),"ground_truth_total":total,
      "frame_weighted_recall_iou_05":round(sum(pairs)/total,5) if total else None,
      "unmatched_tracks_total":unmatched,
      "infer_p50_ms":round(statistics.median(times),2),
      "infer_p95_ms":round(percentile(times,.95),2),
      "prepare_p50_ms":round(statistics.median(prep),2),
      "first_frame_matched":pairs[0],"last_frame_matched":pairs[-1],
      "total_synchronous_run_ms":round(d['wall_ms'],1)}
def main():
    p=argparse.ArgumentParser();p.add_argument('results',type=pathlib.Path)
    a=p.parse_args();rows=[]
    for f in sorted(a.results.glob('*.json')):
        if '__' in f.stem:rows.append(evaluate(f))
    if len(rows)!=4:raise RuntimeError(f'expected 4 reports got {len(rows)}')
    dest=a.results/'SUMMARY.json';dest.write_text(json.dumps(rows,indent=2),encoding="utf-8")
    print(json.dumps(rows,indent=2))
if __name__=='__main__':main()
