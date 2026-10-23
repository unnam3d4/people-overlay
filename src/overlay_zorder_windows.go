//go:build windows

package main

import (
    "time"
)

// Windows 7-compatible Z-order maintenance for the click-through people
// marker layer. SMART PSS, browser video and VMS preview windows may promote
// their own (topmost) HWND after our overlay is created. WS_EX_TOPMOST on
// CreateWindowEx is not sufficient in that case. Reassert our Z order
// WITHOUT activating or resizing the transparent window. This does not
// change drawing, AI, inference cadence or the user's input focus.
var pOverlayForeground = user32.NewProc("GetForegroundWindow")
var pOverlayWindowAbove = user32.NewProc("GetWindow")

const (
    overlayGWPrev         = 3        // GW_HWNDPREV, next window ABOVE
    overlaySWPNoSize      = 0x0001
    overlaySWPNoMove      = 0x0002
    overlaySWPNoActivate  = 0x0010
    overlaySWPNoOwnerZ    = 0x0200
    overlayRecheckPeriod  = 900 * time.Millisecond
)

func overlayNeedsRaise(foreground, previousForeground uintptr, above bool, lastRaise, now time.Time) bool {
    if foreground!=previousForeground {
        // SMART PSS activating a camera/video child window can reorder other
        // topmost HWNDs. Re-pin immediately, not at next 3.8s repaint.
        return true
    }
    // A video client's own topmost layer may appear *after* foreground
    // settles. Avoid unnecessary Win32 calls if we already lead Z-order.
    return above && now.Sub(lastRaise)>=overlayRecheckPeriod
}

func overlayWindowAbove(hwnd uintptr) bool {
    if hwnd==0{return false}
    prev,_,_:=pOverlayWindowAbove.Call(hwnd,overlayGWPrev)
    return prev!=0
}

func raiseOverlayAboveVideo(hwnd uintptr) bool {
    if hwnd==0{return false}
    // HWND_TOPMOST == (HWND)-1. No SWP_NOZORDER: the Z-order update
    // must actually happen; SWP_NOACTIVATE preserves SMART PSS focus.
    ok,_,_:=pPos.Call(hwnd,^uintptr(0),0,0,0,0,
        overlaySWPNoSize|overlaySWPNoMove|overlaySWPNoActivate|overlaySWPNoOwnerZ)
    return ok!=0
}
