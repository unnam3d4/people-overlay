//go:build windows

package main

import (
    "fmt"
    "runtime"
    "sync"
    "sync/atomic"
    "syscall"
    "time"
    "unsafe"
)

// Each physical display has its own tiny click-through layered HWND and DIB.
// This avoids one very large multi-monitor surface, independent z-order
// conflicts, and the previous single-canvas 256 MiB allocation limit.
// Keep ALL window creation and destruction on the main Win32 message thread.
type overlayScreenWindow struct {
    hwnd uintptr
    rect desktopRect
}
type multiOverlay struct {
    mu sync.Mutex
    screens []overlayScreenWindow
}
type overlayPaintState struct {
    pixels dib
    last time.Time
    lastRaise time.Time
    oldVersion int64
    oldEnabled, oldTest bool
    oldColor int32
    good bool
}
var overlayPaintOKMask atomic.Uint32
var overlayPaintFailures atomic.Uint32

var (
    pOverlayDestroyWindow = user32.NewProc("DestroyWindow")
    pOverlaySetTimer = user32.NewProc("SetTimer")
    pOverlayKillTimer = user32.NewProc("KillTimer")
    pOverlayIsVisible = user32.NewProc("IsWindowVisible")
    pOverlayGetRect = user32.NewProc("GetWindowRect")
    pOverlayWindowPID = user32.NewProc("GetWindowThreadProcessId")
    pOverlayCurrentPID = kernel32.NewProc("GetCurrentProcessId")
)
const overlayTopologyTimerID = 6011
const overlayWM_TIMER = 0x0113

func makeOverlayForRect(rect desktopRect, display int)(uintptr,error){
    if rect.width()<64 || rect.height()<64 || !v3ValidDIBSize(rect.width(),rect.height()){
        return 0,fmt.Errorf("invalid display %d overlay geometry: %dx%d",display,rect.width(),rect.height())
    }
    class,_:=syscall.UTF16PtrFromString("STATIC")
    title:="People Overlay AI"
    if display>0 {title=fmt.Sprintf("People Overlay AI - display %d",display+1)}
    name,_:=syscall.UTF16PtrFromString(title)
    hwnd,_,err:=pWindow.Call(layered|transparent|toolWindow|noActivate|topMost,
        uintptr(unsafe.Pointer(class)),uintptr(unsafe.Pointer(name)),popUp,
        uintptr(int64(rect.Left)),uintptr(int64(rect.Top)),
        uintptr(rect.width()),uintptr(rect.height()),0,0,0,0)
    if hwnd==0 {return 0,fmt.Errorf("CreateWindowExW display %d: %v",display+1,err)}
    pShow.Call(hwnd,4) // SW_SHOWNOACTIVATE; never steal focus from RVi.
    if !raiseOverlayAboveVideo(hwnd){
        logError(fmt.Errorf("initial topmost display %d failed",display+1))
    }
    // Win7 accepts WDA_MONITOR where available; WDA_EXCLUDEFROMCAPTURE
    // can be unavailable. Neither is a guarantee for every video driver.
    ok,_,_:=pAffinity.Call(hwnd,0x11)
    if ok==0 {pAffinity.Call(hwnd,1)}
    return hwnd,nil
}

func createMultiOverlay()(*multiOverlay,error){
    rects:=v3AllRects(monitorList())
    if len(rects)==0{return nil,fmt.Errorf("no usable monitors")}
    m:=&multiOverlay{}
    if err:=m.setRects(rects);err!=nil{return nil,err}
    return m,nil
}

// A reused HWND is safe only for the same physical screen slot and geometry.
// This pure policy is regression-tested without requiring a Windows desktop.
func overlayReusePlan(existing []overlayScreenWindow, next []desktopRect) []bool {
    reused:=make([]bool,len(next))
    for i,rect:=range next {
        reused[i]=i<len(existing) && existing[i].hwnd!=0 && existing[i].rect==rect
    }
    return reused
}

// Called only from the main message loop; the renderer holds mu while using
// HWNDs so a display can never disappear midway through UpdateLayeredWindow.
func (m *multiOverlay) setRects(rects []desktopRect) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    existing:=make([]desktopRect,len(m.screens))
    for i,s:=range m.screens {existing[i]=s.rect}
    if v3SameRects(existing,rects){return nil}
    // Reuse unchanged indexed monitors: hot-plugging or resizing one
    // screen must NOT destroy healthy overlays on the other screens.
    // Only reuse the same index+rect: changed indices represent different
    // camera ownership and must be reinitialised.
    pending:=make([]overlayScreenWindow,len(rects))
    reused:=overlayReusePlan(m.screens,rects)
    created:=make([]uintptr,0,len(rects))
    for i,r:=range rects{
        if reused[i] {
            pending[i]=m.screens[i]
            continue
        }
        hwnd,err:=makeOverlayForRect(r,i)
        if err!=nil{
            // Transactional rollback: old windows remain valid and visible.
            for _,newHWND:=range created {pOverlayDestroyWindow.Call(newHWND)}
            return err
        }
        pending[i]=overlayScreenWindow{hwnd:hwnd,rect:r}
        created=append(created,hwnd)
    }
    old:=m.screens
    m.screens=pending
    for i,s:=range old {
        if i>=len(reused)||!reused[i]{pOverlayDestroyWindow.Call(s.hwnd)}
    }
    // Old tracks use the OLD union origin / monitor indices.
    // Invalidate promptly to prevent drawing a person on a different screen.
    boxes.mu.Lock()
    boxes.items=nil
    boxes.updated=time.Time{}
    boxes.mu.Unlock()
    configurationGeneration.Add(1)
    version.Add(1)
    overlayPaintOKMask.Store(0)
    return nil
}

func (m *multiOverlay) checkTopology(){
    rects:=v3AllRects(monitorList())
    if len(rects)==0 {return}
    if err:=m.setRects(rects);err!=nil{logError(fmt.Errorf("overlay topology: %w",err))}
}

func (m *multiOverlay) first() uintptr {
    m.mu.Lock()
    defer m.mu.Unlock()
    if len(m.screens)==0{return 0}
    return m.screens[0].hwnd
}
func (m *multiOverlay) raiseAll(){
    m.mu.Lock()
    defer m.mu.Unlock()
    for i,s:=range m.screens{
        if !raiseOverlayAboveVideo(s.hwnd) {
            logError(fmt.Errorf("manual topmost display %d failed",i+1))
        }
    }
}
func (m *multiOverlay) close(){
    m.mu.Lock()
    defer m.mu.Unlock()
    for _,s:=range m.screens{pOverlayDestroyWindow.Call(s.hwnd)}
    m.screens=nil
}

// All views published by v3Publish carry a display-specific million prefix.
// Never let a box from monitor 1 be drawn on monitor 2, even when its global
// coordinates happen to intersect after a topology change.
func overlayViewBelongsToDisplay(v TrackView,display int) bool {
    return v.ID/1000000==display+1
}
func overlayLocalBox(box Rect, screen,union desktopRect) Rect{
    box.X-=float64(screen.Left-union.Left)
    box.Y-=float64(screen.Top-union.Top)
    return box
}
func overlayBoxOverlapsDisplay(box Rect,w,h int) bool{
    return box.X< float64(w) && box.Y< float64(h) &&
        box.X+box.W>0 && box.Y+box.H>0
}

// Check only foreign visible windows actually occupying this monitor.
// Sibling People Overlay HWNDs can be above each other in the topmost stack
// without obscuring any pixels, and must NOT cause an endless Z-order fight.
func overlayForeignOccluderAbove(hwnd uintptr, rect desktopRect, own map[uintptr]bool) bool {
    pid,_,_:=pOverlayCurrentPID.Call()
    above:=hwnd
    for i:=0;i<96;i++ {
        next,_,_:=pOverlayWindowAbove.Call(above,overlayGWPrev)
        if next==0||next==above {return false}
        above=next
        if own[next] {continue}
        var otherPID uint32
        pOverlayWindowPID.Call(next,uintptr(unsafe.Pointer(&otherPID)))
        if uintptr(otherPID)==pid {continue} // tray/dialogs of our own process
        shown,_,_:=pOverlayIsVisible.Call(next)
        if shown==0 {continue}
        var r desktopRect
        ok,_,_:=pOverlayGetRect.Call(next,uintptr(unsafe.Pointer(&r)))
        if ok==0{continue}
        dx:=minI(int(rect.Right),int(r.Right))-maxI(int(rect.Left),int(r.Left))
        dy:=minI(int(rect.Bottom),int(r.Bottom))-maxI(int(rect.Top),int(r.Top))
        if dx>20 && dy>20 {return true}
    }
    return false
}

func renderMultiOverlay(m *multiOverlay) {
    runtime.LockOSThread()
    defer runtime.UnlockOSThread()
    screen,_,_:=pDC.Call(0)
    if screen==0 {logError(fmt.Errorf("multi-overlay GetDC failed"));return}
    defer pReleaseDC.Call(0,screen)
    layers:=make(map[uintptr]*overlayPaintState)
    defer func(){for _,s:=range layers{s.pixels.close()}}()
    mode:=blend{Op:0,ConstantAlpha:255,AlphaFormat:1}
    var previousForeground uintptr
    for !stopped.Load(){
        m.mu.Lock()
        now:=time.Now()
        fg,_,_:=pOverlayForeground.Call()
        rects:=make([]desktopRect,len(m.screens))
        own:=make(map[uintptr]bool,len(m.screens))
        live:=make(map[uintptr]bool,len(m.screens))
        for i,s:=range m.screens {
            rects[i]=s.rect
            own[s.hwnd]=true
            live[s.hwnd]=true
        }
        for hwnd,state:=range layers{
            if !live[hwnd]{
                state.pixels.close()
                delete(layers,hwnd)
            }
        }
        union:=v3UnionRect(rects)
        enabledNow,testNow,colorNow:=enabled.Load(),testOnly.Load(),colour.Load()
        currentVersion:=version.Load()
        boxes.mu.RLock()
        views:=append([]TrackView(nil),boxes.items...)
        updated:=boxes.updated
        boxes.mu.RUnlock()
        mask:=uint32(0)
        for index,s:=range m.screens{
            state:=layers[s.hwnd]
            if state==nil {
                state=&overlayPaintState{oldVersion:-1}
                layers[s.hwnd]=state
            }
            if overlayNeedsRaise(fg,previousForeground,
                overlayForeignOccluderAbove(s.hwnd,s.rect,own),state.lastRaise,now) {
                if !raiseOverlayAboveVideo(s.hwnd){
                    logError(fmt.Errorf("overlay display %d SetWindowPos failed",index+1))
                }
                state.lastRaise=now
            }
            w,h:=s.rect.width(),s.rect.height()
            if state.pixels.w!=w||state.pixels.h!=h{
                state.pixels.close()
                layer,err:=newDIB(screen,w,h)
                if err!=nil{
                    logError(fmt.Errorf("display %d layer: %w",index+1,err))
                    state.good=false
                    continue
                }
                state.pixels=layer
                state.last=time.Time{}
            }
            changed:=currentVersion!=state.oldVersion ||
                colorNow!=state.oldColor || enabledNow!=state.oldEnabled ||
                testNow!=state.oldTest
            if changed || now.Sub(state.last)>=3800*time.Millisecond {
                for i:=range state.pixels.pixels {state.pixels.pixels[i]=0}
                if enabledNow{
                    if testNow{
                        drawCorners(&state.pixels,[]Rect{{X:float64(w)/2-80,Y:float64(h)/2-140,W:160,H:280}},colorNow)
                    } else if now.Sub(updated)<2800*time.Millisecond{
                        local:=make([]TrackView,0,len(views))
                        for _,v:=range views{
                            if !overlayViewBelongsToDisplay(v,index){continue}
                            v.Box=overlayLocalBox(v.Box,s.rect,union)
                            if overlayBoxOverlapsDisplay(v.Box,w,h){local=append(local,v)}
                        }
                        drawTrackedCorners(&state.pixels,local,colorNow,showNumbers.Load())
                    }
                }
                pos:=point{X:s.rect.Left,Y:s.rect.Top}
                size:=dimensions{CX:int32(w),CY:int32(h)}
                origin:=point{}
                start:=time.Now()
                ok,_,_:=pUpdate.Call(s.hwnd,screen,uintptr(unsafe.Pointer(&pos)),
                    uintptr(unsafe.Pointer(&size)),state.pixels.dc,
                    uintptr(unsafe.Pointer(&origin)),0,
                    uintptr(unsafe.Pointer(&mode)),ulwAlpha)
                lastRenderUS.Store(time.Since(start).Microseconds())
                state.last=now
                state.oldVersion=currentVersion
                state.oldEnabled,state.oldTest,state.oldColor=enabledNow,testNow,colorNow
                state.good=ok!=0
                if !state.good{
                    overlayPaintFailures.Add(1)
                    logError(fmt.Errorf("display %d UpdateLayeredWindow failed",index+1))
                }
            }
            if state.good {mask|=1<<uint(index)}
        }
        overlayPaintOKMask.Store(mask)
        previousForeground=fg
        m.mu.Unlock()
        cadence:=240*time.Millisecond
        if scanMode.Load()==3{
            cadence=100*time.Millisecond
            if fastAI.Load(){cadence=66*time.Millisecond}
        }
        if scanMode.Load()==1{cadence=350*time.Millisecond}
        time.Sleep(cadence)
    }
}
