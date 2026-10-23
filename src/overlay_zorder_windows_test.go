//go:build windows

package main

import (
    "testing"
    "time"
)

func TestOverlayZOrderRestoredImmediatelyWhenSmartPSSOrBrowserActivates(t *testing.T){
    t0:=time.Unix(1000000,0)
    if !overlayNeedsRaise(200,100,false,t0,t0.Add(100*time.Millisecond)){
        t.Fatal("new foreground VMS HWND must trigger immediate re-pin")
    }
    if !overlayNeedsRaise(201,200,false,t0,t0.Add(100*time.Millisecond)){
        t.Fatal("new camera video child HWND must also trigger re-pin")
    }
}

func TestOverlayDoesNotSpamSetWindowPosOnPlainDesktop(t *testing.T){
    t0:=time.Unix(1000000,0)
    for _,age:=range []time.Duration{0,50*time.Millisecond,time.Second,15*time.Second}{
        if overlayNeedsRaise(100,100,false,t0,t0.Add(age)){
            t.Fatal("plain desktop/foreground without any occlusion should not trigger re-pin")
        }
    }
}

func TestOverlayTopmostContenderCausesBoundedWatchdog(t *testing.T){
    t0:=time.Unix(1000000,0)
    if overlayNeedsRaise(200,200,true,t0,t0.Add(300*time.Millisecond)){
        t.Fatal("must not thrash Z order many times per second")
    }
    if !overlayNeedsRaise(200,200,true,t0,t0.Add(950*time.Millisecond)){
        t.Fatal("video window above overlay must be repaired during static UI")
    }
}

func TestOverlayLiftNeverActivatesOrMovesRViWindow(t *testing.T){
    const required=overlaySWPNoSize|overlaySWPNoMove|overlaySWPNoActivate|overlaySWPNoOwnerZ
    if required&overlaySWPNoActivate==0 || required&overlaySWPNoMove==0 ||
       required&overlaySWPNoSize==0{t.Fatal("overlay lift may steal focus or alter geometry")}
    if required&0x4!=0 {t.Fatal("SWP_NOZORDER would negate overlay re-pin")}
}
