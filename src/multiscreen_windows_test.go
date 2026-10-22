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
