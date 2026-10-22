//go:build windows

package main

import "testing"

func TestMonitorPriorityAndNegativeOrigin(t *testing.T) {
    list:=[]desktopMonitor{
        {Rect:desktopRect{Left:1920,Top:0,Right:4480,Bottom:1440}},
        {Rect:desktopRect{Left:-1280,Top:-200,Right:0,Bottom:824}},
        {Rect:desktopRect{Left:0,Top:0,Right:1920,Bottom:1080},Primary:true},
    }
    sortMonitors(list)
    if !list[0].Primary { t.Fatalf("primary monitor not first: %+v",list) }
    if list[1].Rect.Left!=-1280 || list[2].Rect.Left!=1920 {t.Fatalf("secondary monitor ordering wrong: %+v",list)}
    left,ok:=selectedMonitor(list,1)
    if !ok || left.Rect.Left!=-1280 || left.Rect.Top!=-200 || left.Rect.width()!=1280 || left.Rect.height()!=1024 {
        t.Fatalf("secondary screen rectangle corrupted: %+v",left)
    }
    // BitBlt/SetWindowPos accept signed screen positions as pointer-sized
    // LPARAMs on both x86 and x64; no negative origin may wrap into +X.
    if int32(uint32(uintptr(int64(left.Rect.Left)))) != left.Rect.Left {
        t.Fatal("negative virtual-desktop coordinate conversion failed")
    }
}
func TestMonitorSelectionAfterDisconnectFailsSafe(t *testing.T){
    list:=[]desktopMonitor{
       {Rect:desktopRect{Right:1920,Bottom:1080},Primary:true},
    }
    selected,ok:=selectedMonitor(list,7)
    if !ok||selected.Rect.width()!=1920||!selected.Primary {
       t.Fatalf("disconnected second monitor did not fall back to primary: %+v",selected)
    }
    if _,ok:=selectedMonitor(nil,0);ok {t.Fatal("no monitor must not be considered valid")}
}
