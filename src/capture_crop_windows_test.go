//go:build windows

package main

import ("testing";"time")

func TestV4FullCaptureFallbackForAutoAndDiagnostics(t *testing.T){
    old:=detectorOnly.Load()
    defer detectorOnly.Store(old)
    detectorOnly.Store(true)
    for _,tc:=range []struct{
        profile,configured int32
        age time.Duration
        diagnostic bool
        expected bool
    }{
        {3,3,20*time.Second,false,true},
        {4,4,time.Hour,false,true},
        {3,-1,100*time.Millisecond,false,true},
        {3,-1,900*time.Millisecond,false,false},
        {4,4,time.Hour,true,false},
        {0,0,0,false,true},
    }{
        got:=v4CanCropDirect(tc.profile,tc.configured,tc.age,tc.diagnostic)
        if got!=tc.expected {t.Fatalf("profile=%d configured=%d got %t, expected %t",
            tc.profile,tc.configured,got,tc.expected)}
    }
    detectorOnly.Store(false)
    if v4CanCropDirect(3,3,0,false) {t.Fatal("tracked CCTV may not use partial snapshots")}
}
func TestV4ROIStaysWithinAnyMonitor(t *testing.T){
    customGridColumns.Store(12);customGridRows.Store(7)
    defer func(){customGridColumns.Store(4);customGridRows.Store(4)}()
    width,height:=3200,1800
    regs:=regionsForProfile(width,height,4)
    if len(regs)!=85 {t.Fatalf("wanted 84 independent cells got %d",len(regs)-1)}
    pixels:=make([]byte,width*height*4)
    for i:=1;i<len(regs);i++ {
        r:=regs[i]
        if r.x<0||r.y<0||r.w<1||r.h<1||r.x+r.w>width||r.y+r.h>height {
            t.Fatalf("ROI %d out of screen %+v",i,r)
        }
        sample:=copyRegionBGRA(pixels,width,height,r)
        if len(sample)!=r.w*r.h*4 {t.Fatalf("ROI %d read wrong row stride",i)}
    }
}
