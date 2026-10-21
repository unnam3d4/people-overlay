package main

import("math";"testing";"time")

func TestOverlappingCropsDoNotDuplicatePerson(t *testing.T){
 now:=time.Unix(100,0)
 var tr personTracker
 tr.layout(1920,1080,4)
 tr.observePeople([]Rect{{X:201,Y:130,W:72,H:190,Score:.8}},now)
 tr.observePeople([]Rect{{X:205,Y:134,W:75,H:193,Score:.82}},now.Add(100*time.Millisecond))
 if len(tr.views(now.Add(200*time.Millisecond)))!=1{t.Fatalf("duplicate boxes from overlapping crops: %+v",tr.views(now))}
}

func TestAdjacentCameraCannotCaptureSameID(t *testing.T){
 now:=time.Unix(100,0)
 var tr personTracker
 tr.layout(1920,1080,4)
 // On opposite sides of the 480px camera-tile boundary, these are
 // different camera feeds, not a person walking across an actual doorway.
 tr.observePeople([]Rect{{X:442,Y:140,W:64,H:125,Score:.8}},now)
 tr.observePeople([]Rect{{X:483,Y:140,W:64,H:125,Score:.8}},now.Add(100*time.Millisecond))
 trk:=tr.views(now.Add(100*time.Millisecond))
 if len(trk)!=2||trk[0].ID==trk[1].ID{t.Fatalf("camera identity conflict: %+v",trk)}
}

func TestDelayedDetectionFollowsCurrentTrack(t *testing.T){
 before:=[]TrackView{{ID:4,Box:Rect{X:100,Y:100,W:60,H:100,Score:.9}}}
 after:=[]TrackView{{ID:4,Box:Rect{X:112,Y:106,W:60,H:100,Score:.9}}}
 detections:=[]Rect{{X:101,Y:101,W:60,H:100,Score:.8}}
 result:=compensateLateDetections(detections,before,after)
 if len(result)!=1||math.Abs(result[0].X-113)>0.1||math.Abs(result[0].Y-107)>0.1{
  t.Fatalf("stale AI jumps back: %+v",result)
 }
}

func TestNoTeleportFromUnrelatedDetection(t *testing.T){
 old:=[]TrackView{{ID:1,Box:Rect{X:20,Y:20,W:40,H:90}}}
 next:=[]TrackView{{ID:1,Box:Rect{X:200,Y:200,W:40,H:90}}}
 det:=[]Rect{{X:850,Y:700,W:40,H:90}}
 corrected:=compensateLateDetections(det,old,next)
 if math.Abs(corrected[0].X-850)>0.01||math.Abs(corrected[0].Y-700)>0.01{
  t.Fatal("unrelated person was moved to another camera")
 }
}
