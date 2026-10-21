package main

import (
    "testing"
    "time"
)

// A realistic initial 4x4 sweep still must revisit confirmed humans.
// The v20 replay regression (moving 4x4 recall 55% -> 13%) appeared when
// all previously unvisited quiet tiles outranked four known people for
// several consecutive seconds. Prevent that without starving quiet feeds.
func TestLiveHumansRecheckedDuringCCTVStartupFairSweep(t *testing.T) {
    const w,h=960,540
    regs:=regionsForProfile(w,h,3)
    var s scanScheduler
    s.reset(len(regs),w,h,3)
    begin:=time.Unix(140000,0)
    s.firstDecision=begin
    trackedTiles:=[]int{1,6,11,16}
    tracked:=make([]TrackView,0,len(trackedTiles))
    latest:=make(map[int]time.Time)
    for _,idx:=range trackedTiles{
        r:=regs[idx]
        tracked=append(tracked,TrackView{ID:idx,Box:Rect{
            X:float64(r.x)+float64(r.w)*.35,
            Y:float64(r.y)+float64(r.h)*.25,
            W:float64(r.w)*.24,H:float64(r.h)*.44,Score:.87}})
        latest[idx]=begin
        s.markScanned(idx,begin,true)
        s.activity[idx]=.04
    }
    visited:=map[int]bool{}
    for tick:=0;tick<60;tick++{
        at:=begin.Add(time.Duration(tick+1)*450*time.Millisecond)
        i:=s.nextAdaptive(at,3,tracked,regs)
        if i<1||i>=len(regs){t.Fatalf("invalid CCTV tile %d",i)}
        visited[i]=true
        isTracked:=false
        for _,a:=range trackedTiles{if i==a{isTracked=true;break}}
        s.markScanned(i,at,isTracked)
        if isTracked{latest[i]=at}
        if tick>=12{
            for _,idx:=range trackedTiles{
                if age:=at.Sub(latest[idx]);age>5300*time.Millisecond{
                    t.Fatalf("active camera %d was not checked for %v at %v; quiet-camera sweep monopolized inference",idx,age,at.Sub(begin))
                }
            }
        }
    }
    for i:=1;i<len(regs);i++{
        if !visited[i]{t.Errorf("quiet camera %d never scanned in 27s",i)}
    }
}
