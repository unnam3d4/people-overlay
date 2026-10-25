package main

import (
    "math"
    "sync/atomic"
    "time"
)

// Opt-in direct neural detections: no optical flow, long-term ID association,
// visual-appearance tracker or stale-marker carryover. On a dense grid a short
// TTL intentionally makes highlights intermittent between CPU scans.
var detectorOnly atomic.Bool

func directDetectionEnabled(profile int32)bool{
    return picoDetectorCapable && (detectorOnly.Load() || profile==4)
}

type detectorCell struct{
    people []Rect
    observed time.Time
    lastAlert time.Time
}
type detectorCache struct{
    cells []detectorCell
    ttl time.Duration
}
func(d *detectorCache)reset(count int,profile int32){
    d.cells=make([]detectorCell,count)
    d.ttl=1900*time.Millisecond
    if profile==0 {d.ttl=1350*time.Millisecond}
    if profile==4 {d.ttl=2300*time.Millisecond}
}
// A negative model check immediately removes previous markers. We never extend
// a positive's lifetime from static screen texture or from a previous box.
func(d *detectorCache)replace(index int,found []Rect,now time.Time)bool{
    if index<0||index>=len(d.cells)||now.IsZero(){return false}
    c:=&d.cells[index]
    had:=len(c.people)>0 && !c.observed.IsZero() && now.Sub(c.observed)<=d.ttl
    c.people=append(c.people[:0],found...)
    c.observed=now
    confident:=false
    for _,p:=range found{if p.Score>=.72{confident=true;break}}
    if !had&&confident&&(c.lastAlert.IsZero()||now.Sub(c.lastAlert)>=90*time.Second){
        c.lastAlert=now
        return true
    }
    return false
}
func(d *detectorCache)views(now time.Time)[]TrackView{
    out:=make([]TrackView,0,32)
    for i,c:=range d.cells{
        if c.observed.IsZero()||now.Before(c.observed)||now.Sub(c.observed)>d.ttl{continue}
        for j,p:=range c.people{
            if p.W<=0||p.H<=0||math.IsNaN(p.X)||math.IsNaN(p.Y){continue}
            out=append(out,TrackView{ID:(i+1)*1000+j+1,Box:p})
        }
    }
    return out
}
func(d *detectorCache)hasFresh(now time.Time)bool{
    for _,c:=range d.cells{
        if len(c.people)>0&&!c.observed.IsZero()&&!now.Before(c.observed)&&now.Sub(c.observed)<=d.ttl{return true}
    }
    return false
}

// Always scan unseen camera panes before revisits, then the oldest pane.
// Slight motion priority is bounded: it may never starve a quiet camera.
func(s *scanScheduler)nextDetector(now time.Time)int{
    if len(s.lastScan)<2{return 0}
    chosen:=1
    oldest:=time.Duration(-1)
    for i:=1;i<len(s.lastScan);i++{
        if s.lastScan[i].IsZero(){return i}
        if age:=now.Sub(s.lastScan[i]);age>oldest{chosen,oldest=i,age}
    }
    if oldest < 8*time.Second{
        for i:=1;i<len(s.lastScan);i++{
            age:=now.Sub(s.lastScan[i])
            if age<450*time.Millisecond||oldest>2*age{continue}
            if i<len(s.motionArrival)&&s.motionArrival[i]{return i}
        }
    }
    return chosen
}
