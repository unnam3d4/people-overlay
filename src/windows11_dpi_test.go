//go:build windows

package main

import "testing"

func TestDpiAwarenessPseudoHandlesWin11AndWin7(t *testing.T) {
    // Matches winuser.h: (DPI_AWARENESS_CONTEXT)-4 == PerMonitorV2.
    if int64(int32(v3DpiAwareContext(4)))!=-4{
        t.Fatalf("Win11 PerMonitorV2 DPI pseudo-handle invalid: %x",v3DpiAwareContext(4))
    }
    if int64(int32(v3DpiAwareContext(3)))!=-3{
        t.Fatalf("Win10 PerMonitorV1 DPI pseudo-handle invalid: %x",v3DpiAwareContext(3))
    }
}

func TestWindows11Quad4KCanvasNoLongerRejected(t *testing.T) {
    tests:=[]struct{w,h int;want bool}{
        {3840,2160,true},       // single 4K display
        {7680,2160,true},       // two 4K displays
        {15360,2160,true},      // four side-by-side 4K displays, former 10000px bug
        {2160,8640,true},       // four vertically stacked 4K displays
        {10000,3000,true},
        {25600,2160,true},      // multi 8K until explicit memory cap
        {32000,4000,false},    // avoid > 256 MiB DIB allocation
        {15360,4500,false},    // too much RAM for one layered window
        {32768,100,false},     // Win32 max signed-16 dimension restriction
        {0,2160,false},
        {-100,100,false},
    }
    for _,c:=range tests {
        if got:=v3ValidDIBSize(c.w,c.h);got!=c.want {
            t.Errorf("canvas %dx%d got %v expected %v",c.w,c.h,got,c.want)
        }
    }
}
