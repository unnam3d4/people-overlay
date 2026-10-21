package main

import (
 "math"
 "testing"
 "time"
)

func TestV18MotionCCTVFirstAndFair(t *testing.T){
 const w,h=1600,900
 regs:=regionsForProfile(w,h,3)
 var sched scanScheduler
 sched.reset(len(regs),w,h,3)
 now:=time.Unix(50000,0)
 for i:=1;i<len(regs);i++{sched.markScanned(i,now.Add(-time.Second),false)}
 sched.activity[11]=.13
 if got:=sched.nextAdaptive(now,3,nil,regs);got!=11{
  t.Fatalf("moving CCTV target was ignored: camera %d",got)
 }
 // A permanently moving tree must never starve another camera.
 sched.markScanned(3,now.Add(-16*time.Second),false)
 if got:=sched.nextAdaptive(now,3,nil,regs);got!=3{
  t.Fatalf("unchanged camera starved by active tile: %d",got)
 }
}

func TestV18ExistingHumanWinsRecheck(t *testing.T){
 const w,h=1600,900
 regs:=regionsForProfile(w,h,3)
 var sched scanScheduler
 sched.reset(len(regs),w,h,3)
 now:=time.Unix(50500,0)
 for i:=1;i<len(regs);i++{sched.markScanned(i,now.Add(-time.Second),false)}
 sched.markScanned(5,now.Add(-4*time.Second),true)
 existing:=[]TrackView{{Box:Rect{X:30,Y:240,W:65,H:150},ID:10}}
 if got:=sched.nextAdaptive(now,3,existing,regs);got!=5{
  t.Fatalf("a known human was not rechecked, got %d",got)
 }
}

func TestV18NegativeResultFromOldFrameCannotDeleteNewPerson(t *testing.T){
 var tr personTracker
 now:=time.Unix(51000,0)
 tr.layout(800,600,4)
 discovered:=now.Add(2*time.Second)
 tr.observePeople([]Rect{{X:22,Y:20,W:70,H:120,Score:.92}},discovered)
 area:=scanRegion{x:0,y:0,w:200,h:150}
 tr.people[0].LastVisual=now.Add(2500*time.Millisecond)
 tr.noteRegionScanCaptured(area,now.Add(3*time.Second),now,1)
 tr.people[0].LastVisual=now.Add(3500*time.Millisecond)
 tr.noteRegionScanCaptured(area,now.Add(4*time.Second),now,1)
 if got:=tr.views(now.Add(4*time.Second));len(got)!=1{
  t.Fatalf("stale empty inference killed newly visible person: %+v",got)
 }
}

func TestV18NoIntercameraIDTeleportOrMerge(t *testing.T){
 var tr personTracker
 now:=time.Unix(52000,0)
 tr.layout(800,600,4)
 left:=Rect{X:143,Y:60,W:50,H:120,Score:.93}
 right:=Rect{X:206,Y:60,W:50,H:120,Score:.95}
 if tr.tileOf(left)==tr.tileOf(right){t.Fatal("test fixture did not straddle CCTV tile boundary")}
 tr.observePeople([]Rect{left,right},now)
 old:=tr.views(now)
 if len(old)!=2{t.Fatalf("independent feeds merged: %+v",old)}
 if old[0].ID==old[1].ID{t.Fatal("duplicate IDs")}
 tr.observePeople([]Rect{right,left},now.Add(200*time.Millisecond))
 got:=tr.views(now.Add(200*time.Millisecond))
 if len(got)!=2{t.Fatalf("cross-tile suppression: %+v",got)}
 for _,p:=range got{
  if p.ID==old[0].ID&&tr.tileOf(p.Box)!=tr.tileOf(old[0].Box){t.Fatal("track crossed video feeds")}
 }
}

func TestV18KnownStationaryPersonNotLostFromMotionOnlyScheduling(t *testing.T){
 var tr personTracker
 now:=time.Unix(53000,0)
 tr.maxAIAge=16*time.Second
 human,w,h:=flatCCTVFrame(true)
 tr.observePeople([]Rect{{X:30,Y:17,W:40,H:61,Score:.88}},now)
 tr.step(human,w,h,now)
 for i:=1;i<=12;i++{
  tr.step(human,w,h,now.Add(time.Duration(i)*250*time.Millisecond))
 }
 if got:=tr.views(now.Add(3*time.Second));len(got)!=1{
  t.Fatalf("immobile real person dropped before neural recheck: %+v",got)
 }
}

func TestV18FastCCTVThrottleAndBoundedGhostAge(t *testing.T){
 busy:=500*time.Millisecond
 if tmr:=throttleFor(3,busy);tmr>100*time.Millisecond{
  t.Fatalf("avoidable inactive delay between CCTV inferences: %v",tmr)
 }
 if age:=recheckTTL(17,3,3*time.Second);age>16*time.Second{
  t.Fatalf("unverified ghost can remain %.1f sec: %v",math.Round(age.Seconds()),age)
 }
}
