//go:build windows

package main

import "time"

// Pure capture policy for regression tests. A visible Direct/PicoDet CCTV tile
// may use exact-rectangle GDI BitBlt. No HWND/PrintWindow assumptions, because
// some VMS applications render black frames through per-window capture APIs.
func v4CanCropDirect(profile,configured int32,sinceAutoProbe time.Duration,diagnosticRequested bool) bool {
    if !directDetectionEnabled(profile)||diagnosticRequested {return false}
    // Auto may transition from 4x4 back to fullscreen. Periodically sample
    // complete display pixels so the two-vote layout recognizer can notice it.
    if configured==-1 && sinceAutoProbe>850*time.Millisecond {return false}
    return true
}
