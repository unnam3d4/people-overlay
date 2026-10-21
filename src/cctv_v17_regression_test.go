package main

import (
 "testing"
 "time"
)

func TestScreenshotNestedBoxesOnlyOnePerson(t *testing.T){
 // Field report: three sets of corners on one photographed human.
 // The model inferred a full body, partial upper body and face crop.
 var tr personTracker
 now:=time.Unix(40000,0)
 tr.layout(1056,1408,4)
 body:=Rect{X:125,Y:305,W:725,H:790,Score:.76}
 torso:=Rect{X:325,Y:309,W:270,H:510,Score:.92}
 face:=Rect{X:399,Y:532,W:180,H:274,Score:.87}
 tr.observePeople([]Rect{face,torso,body},now)
 got:=tr.views(now)
 if len(got)!=1{t.Fatalf("same person got %d sets of corners: %+v",len(got),got)}
 if got[0].Box.W<600||got[0].Box.H<700{
  t.Fatalf("partial ROI box was chosen instead of full human: %+v",got)
 }
 // New face-only crop must not invent another identity or shrink box.
 oldID:=got[0].ID
 tr.observePeople([]Rect{face},now.Add(time.Second))
 got=tr.views(now.Add(time.Second))
 if len(got)!=1||got[0].ID!=oldID||got[0].Box.W<600{
  t.Fatalf("nested image reopened ghost/head box: %+v",got)
 }
}

func TestTwoNearbyWholePeopleStillTwo(t *testing.T){
 var tr personTracker
 now:=time.Unix(42000,0)
 tr.layout(1920,1080,4)
 tr.observePeople([]Rect{
  {X:120,Y:80,W:70,H:170,Score:.87},
  {X:160,Y:92,W:70,H:180,Score:.91},
 },now)
 if len(tr.views(now))!=2{t.Fatalf("two people were merged: %+v",tr.views(now))}
}

func TestStalePersonAfterTwoNegativeScansClears(t *testing.T){
 var tr personTracker
 now:=time.Unix(43000,0)
 tr.layout(960,540,4)
 tr.maxAIAge=18*time.Second
 tr.observePeople([]Rect{{X:130,Y:94,W:80,H:130,Score:.96}},now)
 camera:=scanRegion{x:0,y:0,w:250,h:160}
 // Region must fully cover the target. This one doesn't, so no misses.
 tr.noteRegionScan(camera,now.Add(time.Second))
 if len(tr.views(now.Add(time.Second)))!=1{t.Fatal("premature deletion")}
 camera=scanRegion{x:0,y:0,w:300,h:300}
 // Simulate stationary pixels still matching while neural recheck finishes.
 tr.people[0].LastVisual=now.Add(3500*time.Millisecond)
 tr.noteRegionScan(camera,now.Add(4*time.Second))
 if len(tr.views(now.Add(4*time.Second)))!=1{t.Fatal("one miss must tolerate occlusion")}
 tr.people[0].LastVisual=now.Add(7500*time.Millisecond)
 tr.noteRegionScan(camera,now.Add(8*time.Second))
 if len(tr.views(now.Add(8*time.Second)))!=0{t.Fatal("obsolete corners stay pinned after camera revisits")}
}

func TestPriorityRecheckDoesNotStarveOtherZones(t *testing.T){
 w,h:=800,600
 regs:=regionsForProfile(w,h,3)
 var s scanScheduler
 s.reset(len(regs),w,h,3)
 now:=time.Unix(44000,0)
 v:=[]TrackView{{Box:Rect{X:40,Y:60,W:70,H:100},ID:1}}
 // Single camera recently scanned: no special priority.
 index:=1
 s.markScanned(index,now,false)
 if got:=s.priorityTracked(v,regs,now.Add(time.Second));got>=0{
  t.Fatalf("fast repeated scan of tracked camera: %d",got)
 }
 got:=s.priorityTracked(v,regs,now.Add(5*time.Second))
 if got!=index{t.Fatalf("tracked camera not rechecked after 4 seconds: got %d expected %d",got,index)}
}

func TestCaptureProbeOnTwoScreens(t *testing.T){
 w,h:=120,90
 a:=make([]byte,w*h*4)
 b:=append([]byte(nil),a...)
 if p:=sampleCaptureDifference(a,b,w,h);p!=0{t.Fatalf("frozen capture changed: %d",p)}
 for y:=10;y<80;y++{for x:=10;x<90;x++{
  i:=(y*w+x)*4
  for c:=0;c<3;c++{b[i+c]=210}
 }}
 if p:=sampleCaptureDifference(a,b,w,h);p<20{
  t.Fatalf("live screenshot not seen as moving: %d",p)
 }
 if p:=sampleCaptureDifference(nil,b,w,h);p!=0{t.Fatal("invalid capture accepted")}
}

func TestFullscreenCCTVDoesNotSplitOneHumanIntoSixteen(t *testing.T){
 rs:=regionsForProfile(1920,1080,0)
 if len(rs)!=1||rs[0].x!=0||rs[0].y!=0||rs[0].w!=1920||rs[0].h!=1080{
  t.Fatalf("single expanded video is cut into many unrelated crops: %+v",rs)
 }
 var scheduler scanScheduler
 scheduler.reset(len(rs),1920,1080,0)
 if idx:=scheduler.next(time.Now(),0);idx!=0{
  t.Fatalf("single camera crop index=%d",idx)
 }
}
