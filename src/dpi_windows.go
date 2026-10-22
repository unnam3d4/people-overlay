//go:build windows

package main

import "syscall"

// Win11 / mixed-DPI desktops: monitor pixel coordinates and the click-through
// overlay must use the SAME physical-pixel coordinate space.
// Before any HWND exists, request Per-Monitor v2 on Windows 10 1703+.
// Windows 8.1 supports PMv1 via shcore; Windows 7 retains system DPI awareness.
//
// Never call a new user32/shcore procedure without Find: missing imports must
// not prevent the standalone EXE from starting on Windows 7.
var (
    winSetDPIContext = user32.NewProc("SetProcessDpiAwarenessContext")
    winShcore = syscall.NewLazyDLL("shcore.dll")
    winSetDPI = winShcore.NewProc("SetProcessDpiAwareness")
)

func v3DpiAwareContext(mode int) uintptr {
    // The Win32 DPI_AWARENESS_CONTEXT values are negative pseudo handles.
    // Construct them from an unsigned value for Go 1.20 x86 and amd64.
    return ^uintptr(mode-1)
}

// Set the process DPI awareness ONCE, prior to making any GUI window,
// including a small tray-message owner. The returned string is diagnostic,
// not an external dependency of frame processing.
func configureProcessDPI() string {
    // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 == -4
    if winSetDPIContext.Find()==nil {
        if ok,_,_:=winSetDPIContext.Call(v3DpiAwareContext(4));ok!=0 {
            return "per-monitor-v2"
        }
        // On some Win10 releases PMv2 is not available, but PMv1 is.
        if ok,_,_:=winSetDPIContext.Call(v3DpiAwareContext(3));ok!=0 {
            return "per-monitor-v1"
        }
    }
    // PROCESS_PER_MONITOR_DPI_AWARE=2 (Windows 8.1+).
    // HRESULT S_OK=0, E_ACCESSDENIED means a manifest already configured DPI.
    if winSetDPI.Find()==nil {
        if hr,_,_:=winSetDPI.Call(2);int32(hr)==0 {
            return "per-monitor-v1"
        }
    }
    // Always present since Vista/Win7. No effect if already configured.
    if ok,_,_:=pDPI.Call();ok!=0 {
        return "system-aware"
    }
    return "already-set-or-unavailable"
}
