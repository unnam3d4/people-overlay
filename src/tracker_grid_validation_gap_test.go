package main

import (
 "testing"
 "time"
)

// The measured 4x4 replay checks an individual positive camera about every
// 1.2-2.0 seconds. Hiding a confirmed ID after 1.2 seconds with no
// intermediate visual match forces a new ID on the next valid neural result.
func TestConfirmedGridHumanSurvivesOneMeasuredNeuralRoundTrip(t *testing.T) {
 start:=time.Unix(350000,0)
 var tr personTracker
 tr.enableConfirmation()
 tr.layout(960,540,4)
 tr.maxAIAge=9*time.Second
 box:=Rect{X:323,Y:155,W:32,H:59,Score:.86}
 tr.observePeople([]Rect{box},start)
 before:=tr.views(start)
 if len(before)!=1 {t.Fatalf("initial human not visible: %+v",before)}
 for _,after:=range []time.Duration{700*time.Millisecond,1200*time.Millisecond,1850*time.Millisecond}{
   current:=tr.views(start.Add(after))
   if len(current)!=1 || current[0].ID!=before[0].ID {
     t.Fatalf("4x4 CPU worker hid existing human after %v despite no negative AI evidence: %+v",after,current)
   }
 }
 // No visual evidence and no subsequent neural confirmation: bound ghosts.
 if after:=tr.views(start.Add(2750*time.Millisecond));len(after)!=0{
   t.Fatalf("unverified CCTV corners remained after 2.75 seconds: %+v",after)
 }
}

func TestGridHumanTwoIndependentMissesStillRemoveImmediately(t *testing.T) {
 now:=time.Unix(350100,0)
 var tr personTracker
 tr.enableConfirmation()
 tr.layout(960,540,4)
 tr.maxAIAge=9*time.Second
 box:=Rect{X:323,Y:155,W:32,H:59,Score:.9}
 tr.observePeople([]Rect{box},now)
 crop:=scanRegion{x:250,y:137,w:260,h:153}
 snapshot:=tr.schedulerViews(now)
 tr.noteRegionScanSnapshot(crop,now.Add(800*time.Millisecond),now.Add(650*time.Millisecond),6,snapshot)
 if len(tr.schedulerViews(now.Add(800*time.Millisecond)))!=1 {
     t.Fatal("single missed AI scan prematurely killed confirmed CCTV human")
 }
 tr.noteRegionScanSnapshot(crop,now.Add(1050*time.Millisecond),now.Add(900*time.Millisecond),6,snapshot)
 if len(tr.schedulerViews(now.Add(1050*time.Millisecond)))!=0 {
     t.Fatal("two independent 4x4 AI misses failed to kill ghost")
 }
}
