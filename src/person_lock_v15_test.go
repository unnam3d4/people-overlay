package main

import (
 "math"
 "testing"
 "time"
)

func TestGlobalMatchNeverAssignsOneBoxToTwoTracks(t *testing.T){
 matches:=bestGlobalMatches([][]float64{{95,90},{93,5}})
 if len(matches)!=2||matches[0]!=1||matches[1]!=0{
  t.Fatalf("must globally assign distinct boxes rather than greedy same-owner: %v",matches)
 }
 if m:=bestGlobalMatches([][]float64{{0,0},{77,0}});m[0]!=-1||m[1]!=0{
  t.Fatalf("missing observation should not steal another track: %v",m)
 }
}

func TestTwoOverlappingPeopleRemainSeparate(t *testing.T){
 var tr personTracker
 now:=time.Unix(25000,0)
 tr.layout(1920,1080,4)
 tr.observePeople([]Rect{
  {X:101,Y:120,W:60,H:135,Score:.92},
  {X:120,Y:121,W:60,H:136,Score:.88},
 },now)
 first:=tr.views(now)
 if len(first)!=2||first[0].ID==first[1].ID{
  t.Fatalf("overlapping people merged: %+v",first)
 }
 // Reverse confidence/order in the next neural scan. IDs must remain distinct.
 tr.observePeople([]Rect{
  {X:123,Y:122,W:60,H:136,Score:.99},
  {X:103,Y:121,W:60,H:135,Score:.70},
 },now.Add(220*time.Millisecond))
 second:=tr.views(now.Add(220*time.Millisecond))
 if len(second)!=2||second[0].ID!=first[0].ID||second[1].ID!=first[1].ID{
  t.Fatalf("reversed AI ordering changed tracks: %+v -> %+v",first,second)
 }
}

func TestTwoOppositeMovingPeopleKeepIDs(t *testing.T){
 var tr personTracker
 now:=time.Unix(26000,0)
 tr.observePeople([]Rect{
  {X:100,Y:120,W:44,H:100,Score:.9},
  {X:157,Y:120,W:44,H:100,Score:.85},
 },now)
 if len(tr.people)!=2{t.Fatal("setup")}
 id0,id1:=tr.people[0].ID,tr.people[1].ID
 tr.people[0].VelocityX=95
 tr.people[1].VelocityX=-95
 tr.people[0].LastStep=now;tr.people[1].LastStep=now
 tr.people[0].LastMotion=now;tr.people[1].LastMotion=now
 future:=now.Add(200*time.Millisecond)
 tr.observePeople([]Rect{
  {X:138,Y:121,W:44,H:100,Score:.99},
  {X:119,Y:120,W:44,H:100,Score:.75},
 },future)
 if len(tr.people)!=2||tr.people[0].ID!=id0||tr.people[1].ID!=id1{
  t.Fatalf("crossing merged or switched IDs: %+v",tr.people)
 }
 if math.Abs(tr.people[0].Box.X-119)>9||math.Abs(tr.people[1].Box.X-138)>9{
  t.Fatalf("trajectory association wrong: %+v",tr.people)
 }
}

func flatCCTVFrame(person bool) ([]byte,int,int){
 const w,h=120,100
 pix:=make([]byte,w*h*4)
 for i:=0;i<w*h;i++{
  val:=byte(27)
  x,y:=i%w,i/w
  if person&&x>=30&&x<70&&y>=17&&y<78{val=94}
  for k:=0;k<3;k++{pix[i*4+k]=val}
 }
 return pix,w,h
}
func TestStationaryLowTextureHumanRemainsLocked(t *testing.T){
 var tr personTracker
 start:=time.Unix(30000,0)
 tr.observePeople([]Rect{{X:30,Y:17,W:40,H:61,Score:.93}},start)
 tr.maxAIAge=20*time.Second
 frame,w,h:=flatCCTVFrame(true)
 tr.step(frame,w,h,start)
 for i:=1;i<=20;i++{
  tr.step(frame,w,h,start.Add(time.Duration(i)*200*time.Millisecond))
  out:=tr.views(start.Add(time.Duration(i)*200*time.Millisecond))
  if len(out)!=1||out[0].ID!=1{
   t.Fatalf("stationary person disappeared after %.1fs: %+v",float64(i)*.2,out)
  }
  if math.Abs(out[0].Box.X-30)>.2||math.Abs(out[0].Box.Y-17)>.2{
   t.Fatalf("stationary bounding corners drifted: %+v",out)
  }
 }
 gone,_,_:=flatCCTVFrame(false)
 tr.step(gone,w,h,start.Add(4200*time.Millisecond))
 if len(tr.views(start.Add(7*time.Second)))!=0{
  t.Fatal("removed silhouette cannot remain forever without AI validation")
 }
}

func TestNearIdenticalYOLODuplicatesNotTwoPeople(t *testing.T){
 var tr personTracker
 now:=time.Unix(32000,0)
 tr.observePeople([]Rect{
  {X:100,Y:100,W:50,H:120,Score:.91},
  {X:101,Y:101,W:50,H:121,Score:.82},
 },now)
 if len(tr.views(now))!=1{t.Fatalf("duplicate YOLO proposals: %+v",tr.views(now))}
}

func TestUnmatchedFarHumanGetsIndependentID(t *testing.T){
 var tr personTracker
 now:=time.Unix(33000,0)
 tr.observePeople([]Rect{{X:100,Y:100,W:50,H:110,Score:.94}},now)
 tr.observePeople([]Rect{
  {X:300,Y:100,W:50,H:110,Score:.92},
  {X:103,Y:100,W:50,H:110,Score:.89},
 },now.Add(300*time.Millisecond))
 if len(tr.people)!=2||tr.people[0].ID==tr.people[1].ID{
  t.Fatalf("second person captured first one's ID: %+v",tr.people)
 }
}
