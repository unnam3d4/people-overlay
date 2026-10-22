//go:build windows

package main

import (
    "testing"
    "time"
)

func TestV3UnionAndDisplayRelativeCoordinates(t *testing.T) {
    ms:=[]desktopMonitor{
        {Rect:desktopRect{Left:0,Top:0,Right:1280,Bottom:1024},Primary:true},
        {Rect:desktopRect{Left:-1024,Top:-256,Right:0,Bottom:512}},
        {Rect:desktopRect{Left:1280,Top:128,Right:2646,Bottom:896}},
    }
    rs:=v3AllRects(ms)
    full:=v3UnionRect(rs)
    if full!=(desktopRect{Left:-1024,Top:-256,Right:2646,Bottom:1024}) {
        t.Fatalf("virtual desktop union: %+v",full)
    }
    one:=v3RelativeBox(Rect{X:10,Y:20,W:35,H:80},rs[0],full)
    if one.X!=1034||one.Y!=276||one.W!=35||one.H!=80 {
        t.Fatalf("primary coords in negative-origin desktop: %+v",one)
    }
    left:=v3RelativeBox(Rect{X:0,Y:0,W:40,H:70},rs[1],full)
    if left.X!=0||left.Y!=0 {t.Fatalf("left monitor origin: %+v",left)}
    if !v3SameRects(rs,append([]desktopRect(nil),rs...)){t.Fatal("same screens not stable")}
    reverse:=append([]desktopRect(nil),rs...)
    reverse[0],reverse[1]=reverse[1],reverse[0]
    if v3SameRects(rs,reverse){t.Fatal("must invalidate tracks after screen reorder")}
}

func TestV3CapFourAndDynamicDisconnect(t *testing.T){
    var monitors []desktopMonitor
    for i:=0;i<6;i++{
        x:=int32(800*i)
        monitors=append(monitors,desktopMonitor{Rect:desktopRect{Left:x,Top:0,Right:x+800,Bottom:600}})
    }
    got:=v3AllRects(monitors)
    if len(got)!=4{t.Fatalf("expected max four screens, got %d",len(got))}
    if v3UnionRect(got).width()!=3200{t.Fatalf("union too large: %+v",v3UnionRect(got))}
    if v3SameRects(got,v3AllRects(monitors[:3])){t.Fatal("disconnect must rebuild overlays and contexts")}
    if v3UnionRect(nil)!= (desktopRect{}){t.Fatal("empty topology must be nonvalid")}
}

func TestV3IndependentDisplayEpochs(t *testing.T) {
    screens:=[]*v3Screen{{epoch:10},{epoch:10},{epoch:10}}
    first:=inferenceReply{display:0,epoch:10}
    second:=inferenceReply{display:1,epoch:10}
    if !v3AcceptReply(screens,first) || !v3AcceptReply(screens,second) {
        t.Fatal("legitimate reply rejected")
    }
    // Scene changed only on display 1; monitor 2 keeps its in-flight hit.
    screens[0].epoch=11
    if v3AcceptReply(screens,first) {t.Fatal("old scene returned stale people")}
    if !v3AcceptReply(screens,second) {t.Fatal("unrelated screen lost its result")}
    screens[1].epoch=12
    if v3AcceptReply(screens,second){t.Fatal("topology change revived old reply")}
    if v3AcceptReply(screens,inferenceReply{display:3,epoch:12}) {
        t.Fatal("out-of-range physical monitor accepted")
    }
    if v3AcceptReply(screens,inferenceReply{display:-1,epoch:12}) {
        t.Fatal("negative monitor index accepted")
    }
}

func TestV3MultiMonitorCaptureCadence(t *testing.T) {
    if v3CaptureCadence(3,1,false,false)!=300*time.Millisecond {
        t.Fatal("single Windows screen should preserve baseline idle cadence")
    }
    if v3CaptureCadence(3,4,false,false)!=110*time.Millisecond {
        t.Fatal("four screens must not wait 1.2 seconds between samples")
    }
    if v3CaptureCadence(3,4,true,true)!=66*time.Millisecond {
        t.Fatal("fast person tracking must preserve fast cadence")
    }
}

func TestV3PublishIndependentScreenCoordinatesAndIds(t *testing.T) {
    now:=time.Now()
    union:=desktopRect{Left:-800,Top:0,Right:1280,Bottom:900}
    screens:=[]*v3Screen{
        {rect:desktopRect{Left:-800,Top:0,Right:0,Bottom:600},
            tracker:personTracker{people:[]personTrack{{ID:7,Box:Rect{X:15,Y:20,W:30,H:80},LastAI:now,LastVisual:now}}}},
        {rect:desktopRect{Left:0,Top:100,Right:1280,Bottom:900},
            tracker:personTracker{people:[]personTrack{{ID:7,Box:Rect{X:25,Y:35,W:35,H:85},LastAI:now,LastVisual:now}}}},
    }
    var old []TrackView
    v3Publish(screens,union,now,&old)
    boxes.mu.RLock()
    got:=append([]TrackView(nil),boxes.items...)
    boxes.mu.RUnlock()
    if len(got)!=2 {t.Fatalf("expected independent corner sets from both screens: %+v",got)}
    if got[0].ID==got[1].ID {t.Fatal("monitor-local track ids collided")}
    if got[0].Box.X!=15 || got[0].Box.Y!=20 {t.Fatalf("left origin misaligned: %+v",got[0])}
    if got[1].Box.X!=825 || got[1].Box.Y!=135 {t.Fatalf("right monitor origin misaligned: %+v",got[1])}
}

func TestV3CameraCoverageRespectsUnseenAndSeparateScreens(t *testing.T) {
    now:=time.Unix(1700000020,0)
    monitors:=[]*v3Screen{{},{},{}}
    monitors[0].sch.lastScan=make([]time.Time,17)
    monitors[0].completed=make([]time.Time,17)
    monitors[0].sch.firstDecision=now.Add(-4*time.Second)
    monitors[0].completed[1]=now.Add(-2*time.Second)
    monitors[1].sch.lastScan=make([]time.Time,1)
    monitors[1].completed=make([]time.Time,1) // fullscreen is one camera
    monitors[1].sch.firstDecision=now.Add(-time.Second)
    monitors[2].sch.lastScan=make([]time.Time,1)
    monitors[2].completed=make([]time.Time,1)
    monitors[2].completed[0]=now.Add(-3*time.Second)
    unseen,age:=v3CameraCoverage(monitors,now)
    if unseen!=16 {t.Fatalf("expected 15 unseen 4x4 tiles and 1 fullscreen tile: %d",unseen)}
    if age!=4*time.Second {t.Fatalf("unseen camera deadline must be 4s, got %v",age)}
    monitors[0].sch.markScanned(2,now.Add(-500*time.Millisecond),false)
    // Dispatching a crop is not a successful inference. It must not
    // accidentally reduce the unseen-camera count.
    if unseenAgain,_:=v3CameraCoverage(monitors,now);unseenAgain!=16{t.Fatal("queued crop counted as completed")}
    monitors[0].completed[2]=now.Add(-500*time.Millisecond)
    unseen,age=v3CameraCoverage(monitors,now)
    if unseen!=15||age!=4*time.Second {
        t.Fatalf("scanning one CCTV camera must not erase all unseen ages: %d %v",unseen,age)
    }
}

func TestV3FullscreenWinsMoreNeuralSlotsWithoutGridStarvation(t *testing.T) {
    screens:=[]*v3Screen{{profile:0},{profile:3},{profile:2},{profile:1}}
    served:=make([]int,4)
    last:=-1
    for i:=0;i<120;i++{
        next:=v3SelectNextAI(screens,served,last)
        if next<0||next>3 {t.Fatalf("bad screen index %d",next)}
        served[next]++
        last=next
    }
    if served[0]<48 || served[0]>64 {
        t.Fatalf("single expanded CCTV must win about half NN budget, got %+v",served)
    }
    for i:=1;i<4;i++ {
        if served[i]<15 {t.Fatalf("grid monitor %d starved: %+v",i,served)}
    }
    screens[0].captureFailures=3
    next:=v3SelectNextAI(screens,served,3)
    if next==0 {t.Fatal("broken screenshot may not block other monitors")}
    screens[0].captureFailures=0
    if len(v3AllRects(nil))!=0 {t.Fatal("unexpected display")}
}

func TestV3PrecisionFullscreenDoesNotThrottleNewPersonChecks(t *testing.T) {
    if v3AIPause(0,250*time.Millisecond,true)!=0{
        t.Fatal("fullscreen precision mode must not wait idle between neural checks")
    }
    if v3FrameWait(0,1,false,false,true)!=85*time.Millisecond {
        t.Fatal("one full camera must use experimental 85ms capture policy")
    }
    for _,profile:=range []int32{1,2,3}{
        if v3AIPause(profile,250*time.Millisecond,true)<=0{
            t.Fatalf("grid mode %d should preserve inference throttle",profile)
        }
        if v3FrameWait(profile,4,false,false,true)>110*time.Millisecond{
            t.Fatalf("four monitors should retain fair capture cadence, grid %d",profile)
        }
    }
    if v3AIPause(0,250*time.Millisecond,false)==0 {
        t.Fatal("low CPU mode must be available for older Windows i3 machines")
    }
}

// FastTracking is opt-in: on 16-camera mosaics it eliminates the deliberate
// inter-inference idle time without changing the default low-CPU mode.
func TestV3FastTrackingSkipsFourByFourIdleOnlyWhenEnabled(t *testing.T) {
    busy:=250*time.Millisecond
    standard:=v3FastAIPause(3,busy,true,false)
    if standard<50*time.Millisecond {
        t.Fatalf("economical 4x4 unexpectedly lost its throttle: %s",standard)
    }
    if got:=v3FastAIPause(3,busy,true,true);got!=0{
        t.Fatalf("opt-in 4x4 fast tracking must not idle AI worker: %s",got)
    }
    for _,profile:=range []int32{0,1,2}{
        got:=v3FastAIPause(profile,busy,false,true)
        want:=v3AIPause(profile,busy,false)
        if got!=want {
            t.Fatalf("fast 4x4 setting must not change profile %d: got %s want %s",profile,got,want)
        }
    }
}
