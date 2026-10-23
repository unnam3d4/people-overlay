//go:build windows

package main

import "testing"

func TestMultiOverlayLocalCoordinatesWithNegativeMonitorOrigin(t *testing.T) {
    rects:=[]desktopRect{
        {Left:0,Top:0,Right:1280,Bottom:720},
        {Left:-1024,Top:0,Right:0,Bottom:768},
        {Left:0,Top:-900,Right:1600,Bottom:0},
    }
    union:=v3UnionRect(rects)
    if union.Left!=-1024||union.Top!=-900{
        t.Fatalf("wrong desktop union %+v",union)
    }
    for i,r:=range rects{
        onScreen:=Rect{X:float64(r.Left+100-union.Left),Y:float64(r.Top+70-union.Top),W:35,H:80}
        got:=overlayLocalBox(onScreen,r,union)
        if got.X!=100||got.Y!=70||!overlayBoxOverlapsDisplay(got,r.width(),r.height()){
            t.Fatalf("display %d incorrect relative pixel coordinates: %+v",i+1,got)
        }
    }
}

func TestMultiOverlayCannotDrawPeopleFromAnotherScreen(t *testing.T){
    sample:=[]TrackView{
        {ID:1000021,Box:Rect{X:10,Y:20,W:40,H:80}},
        {ID:2000032,Box:Rect{X:20,Y:20,W:40,H:80}},
        {ID:3000043,Box:Rect{X:30,Y:20,W:40,H:80}},
        {ID:4000054,Box:Rect{X:40,Y:20,W:40,H:80}},
    }
    for i:=0;i<4;i++{
        count:=0
        for _,v:=range sample{
            if overlayViewBelongsToDisplay(v,i){count++}
        }
        if count!=1{t.Fatalf("display %d sees %d tracks, expected one",i+1,count)}
    }
}

func TestMultiOverlayClipOffscreenBoxes(t *testing.T){
    w,h:=1280,720
    for _,r:=range []Rect{
        {X:-50,Y:20,W:30,H:50},{X:1280,Y:20,W:50,H:50},
        {X:20,Y:-100,W:50,H:99},{X:20,Y:720,W:50,H:50},
    } {
        if overlayBoxOverlapsDisplay(r,w,h){t.Fatalf("offscreen box rendered %+v",r)}
    }
    for _,r:=range []Rect{
        {X:0,Y:0,W:1,H:1},{X:1279,Y:719,W:10,H:10},
        {X:-20,Y:10,W:30,H:30},
    }{
        if !overlayBoxOverlapsDisplay(r,w,h){t.Fatalf("visible box suppressed %+v",r)}
    }
}

func TestMultiOverlayRectsAvoidGlobalCanvasLimit(t *testing.T){
    // Four 8K screens placed side-by-side exceed the 256 MiB union DIB cap
    // but EACH individual display remains supported with per-screen HWNDs.
    rects:=[]desktopRect{
        {Left:0,Top:0,Right:7680,Bottom:4320},
        {Left:7680,Top:0,Right:15360,Bottom:4320},
        {Left:15360,Top:0,Right:23040,Bottom:4320},
        {Left:23040,Top:0,Right:30720,Bottom:4320},
    }
    u:=v3UnionRect(rects)
    if v3ValidDIBSize(u.width(),u.height()){t.Fatal("expected former union-cap issue")}
    for i,r:=range rects {
        if !v3ValidDIBSize(r.width(),r.height()){t.Fatalf("monitor %d invalid",i)}
    }
}

func TestMonitorGeometryChangeDetectedRegardlessOfMonitorCount(t *testing.T){
    old:=[]desktopRect{{Left:0,Top:0,Right:1920,Bottom:1080},{Left:1920,Top:0,Right:3840,Bottom:1080}}
    updated:=[]desktopRect{{Left:0,Top:0,Right:1920,Bottom:1080},{Left:1920,Top:0,Right:3200,Bottom:1024}}
    if v3SameRects(old,updated){t.Fatal("resolution change not recognized")}
    if !v3SameRects(old,append([]desktopRect(nil),old...)){t.Fatal("stable topology considered changed")}
}
