package main

import("testing";"time")

func TestCCTVRegionsCoverSmallScreens(t *testing.T){
 for _,dims:=range [][2]int{{1920,1080},{1366,768},{1280,1024},{800,600}}{
  for _,profile:=range []int32{1,2,3}{
   regions:=regionsForProfile(dims[0],dims[1],profile)
   grid:=2
   if profile==2{grid=3}
   if profile==3{grid=4}
   if len(regions)!=grid*grid+1{t.Fatalf("profile %d count=%d",profile,len(regions))}
   for _,r:=range regions{
    if r.x<0||r.y<0||r.w<1||r.h<1||r.x+r.w>dims[0]||r.y+r.h>dims[1]{
     t.Fatalf("out of bounds %v",r)
    }
   }
  }
 }
}

func TestHighMotionGetsPriority(t *testing.T){
 const w,h=800,600
 tiles:=regionsForProfile(w,h,3)
 var s scanScheduler
 s.reset(len(tiles),w,h,3)
 now:=time.Now()
 s.markScanned(0,now,false)
 for i:=1;i<len(tiles);i++{s.markScanned(i,now.Add(-time.Second),false)}
 s.activity[12]=.5
 if got:=s.next(now,3);got!=12{t.Fatalf("moving camera got %d",got)}
}

func TestStationaryCamerasStillScanned(t *testing.T){
 tiles:=regionsForProfile(640,480,2)
 var s scanScheduler
 s.reset(len(tiles),640,480,2)
 now:=time.Now()
 s.markScanned(0,now,false)
 for i:=1;i<len(tiles);i++{s.markScanned(i,now,false)}
 s.markScanned(3,now.Add(-100*time.Second),false)
 s.activity[2]=.1
 if got:=s.next(now,2);got!=3{t.Fatalf("starved stationary tile; selected %d",got)}
}

func TestFullFramePeriodic(t *testing.T){
 tiles:=regionsForProfile(640,480,3)
 var s scanScheduler
 s.reset(len(tiles),640,480,3)
 now:=time.Now()
 if got:=s.next(now,3);got!=0{t.Fatalf("first scan should be fullscreen, got %d",got)}
 s.markScanned(0,now.Add(-13*time.Second),false)
 if got:=s.next(now,3);got!=0{t.Fatalf("periodic fullscreen required, got %d",got)}
}

func TestMotionSignature(t *testing.T){
 const w,h=640,480
 frame:=make([]byte,w*h*4)
 tile:=scanRegion{0,0,320,240}
 initial:=tinySignature(frame,w,h,tile)
 after:=append([]byte(nil),frame...)
 for y:=30;y<190;y++{for x:=30;x<200;x++{
  p:=(y*w+x)*4; after[p],after[p+1],after[p+2]=255,255,255
 }}
 current:=tinySignature(after,w,h,tile)
 if movementScore(initial,current)<=.2{t.Fatal("significant movement invisible")}
 if movementScore(initial,initial)!=0{t.Fatal("stationary video must not be motion")}
}

func TestBudgetProfiles(t *testing.T){
 busy:=400*time.Millisecond
 eco,balanced,fast:=throttleFor(1,busy),throttleFor(2,busy),throttleFor(3,busy)
 if !(eco>balanced&&balanced>fast){t.Fatalf("bad budgets: %v %v %v",eco,balanced,fast)}
 if fast<50*time.Millisecond{t.Fatal("unbounded CPU pressure")}
}
