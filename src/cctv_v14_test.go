package main

import (
 "math"
 "testing"
 "time"
)

func TestAsyncROICaptureMatchesDirectPreprocessing(t *testing.T){
 const w,h=777,453
 pixels:=make([]byte,w*h*4)
 for i:=0;i<len(pixels);i++{pixels[i]=byte((i*71+i/123)%256)}
 r:=scanRegion{x:175,y:81,w:215,h:172}
 crop:=copyRegionBGRA(pixels,w,h,r)
 if len(crop)!=r.w*r.h*4{t.Fatalf("ROI size: %d",len(crop))}
 for _,enhance:=range []bool{false,true}{
  direct,s1,x1,y1:=prepareRegionFrameEnhanced(pixels,w,h,r,enhance)
  async,s2,x2,y2:=prepareRegionFrameEnhanced(crop,r.w,r.h,scanRegion{0,0,r.w,r.h},enhance)
  if s1!=s2||x1!=x2||y1!=y2||len(direct)!=len(async){t.Fatal("async preprocessing changed letterbox")}
  for i:=range direct{if direct[i]!=async[i]{t.Fatalf("different AI tensor at %d with enhancement=%v",i,enhance)}}
 }
 old:=crop[0]
 pixels[(r.y*w+r.x)*4]^=255
 if crop[0]!=old{t.Fatal("queued ROI must not change with the next BitBlt frame")}
}

func TestInvalidROICaptureRejected(t *testing.T){
 raw:=make([]byte,80*60*4)
 for _,r:=range []scanRegion{{-1,0,10,10},{79,0,10,10},{0,0,80,61},{0,0,0,10}}{
  if got:=copyRegionBGRA(raw,80,60,r);got!=nil{t.Fatalf("unsafe region %+v accepted",r)}
 }
}

func TestCCTVGlobalSceneCutOnlyOnMostCameras(t *testing.T){
 const w,h=800,600
 regions:=regionsForProfile(w,h,3)
 var s scanScheduler
 s.reset(len(regions),w,h,3)
 frame:=make([]byte,w*h*4)
 if s.observe(frame,w,h,regions){t.Fatal("first frame is not a scene cut")}
 patch:=append([]byte(nil),frame...)
 for y:=0;y<150;y++{for x:=0;x<200;x++{
  off:=(y*w+x)*4;patch[off]=255;patch[off+1]=255;patch[off+2]=255
 }}
 if s.observe(patch,w,h,regions){t.Fatal("person or motion in one camera should not reset every camera")}
 for i:=range patch{patch[i]=255}
 if !s.observe(patch,w,h,regions){t.Fatal("full mosaic layout replacement not detected")}
}

func TestMissingAIObservationsRemoveGhostButNotFirstMiss(t *testing.T){
 var tr personTracker
 now:=time.Unix(100,0)
 tr.layout(800,600,4)
 tr.observePeople([]Rect{{X:50,Y:40,W:40,H:90,Score:.94}},now)
 roi:=scanRegion{0,0,200,150}
 tr.noteRegionScan(roi,now.Add(150*time.Millisecond))
 if len(tr.views(now.Add(150*time.Millisecond)))!=1{t.Fatal("a single neural miss must not erase valid corners")}
 tr.noteRegionScan(roi,now.Add(350*time.Millisecond))
 if len(tr.views(now.Add(350*time.Millisecond)))!=0{t.Fatal("two missing camera scans should erase ghost corners")}
}

func TestPartialCropCannotRemovePerson(t *testing.T){
 var tr personTracker
 now:=time.Unix(100,0)
 tr.observePeople([]Rect{{X:170,Y:40,W:65,H:90,Score:.94}},now)
 partial:=scanRegion{0,0,190,150}
 tr.noteRegionScan(partial,now.Add(100*time.Millisecond))
 tr.noteRegionScan(partial,now.Add(200*time.Millisecond))
 if len(tr.views(now.Add(300*time.Millisecond)))!=1{t.Fatal("crop edge cannot count as a negative person result")}
 // A positive revisit cancels an earlier missed-camera count.
 full:=scanRegion{0,0,300,150}
 tr.noteRegionScan(full,now.Add(350*time.Millisecond))
 tr.observePeople([]Rect{{X:171,Y:40,W:65,H:90,Score:.95}},now.Add(400*time.Millisecond))
 tr.noteRegionScan(full,now.Add(500*time.Millisecond))
 if len(tr.views(now.Add(550*time.Millisecond)))!=1{t.Fatal("positive AI revisit must reset missed detections")}
}

func TestCCTVRevalidationBudgetAdaptsToSlowInference(t *testing.T){
 fast:=recheckTTL(17,3,250*time.Millisecond)
 slow:=recheckTTL(17,3,2*time.Second)
 if fast<9*time.Second||slow<=fast||slow>45*time.Second{
  t.Fatalf("bad slow-CPU recheck budget fast=%v slow=%v",fast,slow)
 }
 if math.IsNaN(float64(fast)){t.Fatal("invalid budget")}
}
