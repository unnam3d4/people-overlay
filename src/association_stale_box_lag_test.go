package main

import ("testing";"time";"math")

func TestStrongNewPersonBoxCorrectsOneSecondOldTrackWithoutLongLag(t *testing.T) {
 at:=time.Unix(410000,0)
 var tracker personTracker
 tracker.people=[]personTrack{{ID:17,Tile:0,
  Box:Rect{X:100,Y:155,W:31,H:76,Score:.81},
  LastAI:at.Add(-1100*time.Millisecond),
  LastVisual:at.Add(-700*time.Millisecond),
 }}
 tracker.nextID=17
 latest:=Rect{X:133,Y:156,W:33,H:77,Score:.88}
 ids:=tracker.assignDetectionsAt([]Rect{latest},at,at.Add(-300*time.Millisecond))
 if ids[0]!=17 {
  t.Fatalf("person changed ID despite valid neural detection, got %d",ids[0])
 }
 if math.Abs(tracker.people[0].Box.X-latest.X)>2 {
  t.Fatalf("stale human corners lag new neural position by %.2f pixels",latest.X-tracker.people[0].Box.X)
 }
}

func TestSmallYOLOJitterDoesNotShakeStationaryCornerMarkers(t *testing.T){
 at:=time.Unix(410100,0)
 var tracker personTracker
 tracker.people=[]personTrack{{ID:8,Box:Rect{X:100,Y:130,W:34,H:73,Score:.9},LastAI:at}}
 tracker.nextID=8
 ids:=tracker.assignDetectionsAt([]Rect{{X:100.8,Y:129.3,W:34.3,H:73.5,Score:.9}},at.Add(200*time.Millisecond),at.Add(100*time.Millisecond))
 if ids[0]!=8{t.Fatal("stationary person's track ID changed")}
 if math.Abs(tracker.people[0].Box.X-100)>0.5||math.Abs(tracker.people[0].Box.Y-130)>0.5{
  t.Fatalf("tiny YOLO jitter caused large corner jump: %+v",tracker.people[0].Box)
 }
}
