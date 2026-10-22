//go:build windows

package main

import (
    "runtime"
    "sort"
    "syscall"
    "unsafe"
)

// Windows monitor coordinates are relative to the *virtual* desktop.
// A secondary display can have negative X/Y; cropping at 0,0 silently
// captures the primary display even while the user watches RVi elsewhere.
type desktopRect struct {Left, Top, Right, Bottom int32}
func (r desktopRect) width() int {return int(r.Right-r.Left)}
func (r desktopRect) height() int{return int(r.Bottom-r.Top)}
type desktopMonitor struct {
    Rect desktopRect
    Primary bool
}
type monitorInfo struct {
    Size uint32
    Monitor desktopRect
    Work desktopRect
    Flags uint32
}

var (
    enumDisplayMonitors = user32.NewProc("EnumDisplayMonitors")
    getMonitorInfo = user32.NewProc("GetMonitorInfoW")
    monitorCallback = syscall.NewCallback(func(hmon, hdc, rc, data uintptr) uintptr {
        monitors := (*[]desktopMonitor)(unsafe.Pointer(data))
        info:=monitorInfo{Size:uint32(unsafe.Sizeof(monitorInfo{}))}
        ok,_,_:=getMonitorInfo.Call(hmon,uintptr(unsafe.Pointer(&info)))
        if ok!=0 && info.Monitor.width()>0 && info.Monitor.height()>0 {
            *monitors=append(*monitors,desktopMonitor{
                Rect: info.Monitor, Primary:info.Flags&1!=0,
            })
        }
        return 1
    })
)

func sortMonitors(monitors []desktopMonitor) {
    sort.SliceStable(monitors,func(i,j int)bool{
        if monitors[i].Primary!=monitors[j].Primary {return monitors[i].Primary}
        if monitors[i].Rect.Left!=monitors[j].Rect.Left{return monitors[i].Rect.Left<monitors[j].Rect.Left}
        return monitors[i].Rect.Top<monitors[j].Rect.Top
    })
}
func selectedMonitor(monitors []desktopMonitor, selection int32)(desktopMonitor,bool){
    if len(monitors)==0 {return desktopMonitor{},false}
    index:=int(selection)
    if index<0||index>=len(monitors){index=0}
    return monitors[index],true
}
func monitorList()[]desktopMonitor{
    var all []desktopMonitor
    ok,_,_:=enumDisplayMonitors.Call(0,0,monitorCallback,uintptr(unsafe.Pointer(&all)))
    runtime.KeepAlive(&all)
    if ok==0 || len(all)==0 {
        w,_,_:=pMetric.Call(0);h,_,_:=pMetric.Call(1)
        if w<64||h<64{return nil}
        return []desktopMonitor{{Rect:desktopRect{Right:int32(w),Bottom:int32(h)},Primary:true}}
    }
    sortMonitors(all)
    return all
}
func activeDesktopRect()desktopRect{
    m,ok:=selectedMonitor(monitorList(),displaySelection.Load())
    if !ok{return desktopRect{}}
    return m.Rect
}
