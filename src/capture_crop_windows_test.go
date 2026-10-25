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

func TestV4RegionOnlyReducesGDITransferBytes(t *testing.T) {
    for _,tc:=range []struct{c,r int}{{4,4},{6,4},{10,5},{12,8}}{
        customGridColumns.Store(int32(tc.c))
        customGridRows.Store(int32(tc.r))
        profile:=int32(4)
        if tc.c==4&&tc.r==4{profile=3}
        regions:=regionsForProfile(3200,1800,profile)
        fullPixels:=int64(3200*1800)
        var total int64
        for _,roi:=range regions[1:] {total+=int64(roi.w*roi.h)}
        if total<=0||len(regions)<2 {t.Fatal("missing camera regions")}
        avg:=total/int64(len(regions)-1)
        if avg>=fullPixels/3 {t.Fatalf("%dx%d ROI too large: avg %d of %d",tc.c,tc.r,avg,fullPixels)}
    }
    customGridColumns.Store(4)
    customGridRows.Store(4)
}

func TestV4RealGDIRegionCopyDoesNotTouchAdjacentPixels(t *testing.T){
    screen,_,_:=pDC.Call(0)
    if screen==0{t.Skip("headless Windows runner has no screen DC")}
    defer pReleaseDC.Call(0,screen)
    const width,height=60,45
    src,err:=newDIB(screen,width,height)
    if err!=nil{t.Fatalf("source DIB failed: %v",err)}
    defer src.close()
    dst,err:=newDIB(screen,width,height)
    if err!=nil{t.Fatalf("destination DIB failed: %v",err)}
    defer dst.close()
    for y:=0;y<height;y++{for x:=0;x<width;x++{
        i:=4*(y*width+x)
        src.pixels[i]=byte(x*3+1)
        src.pixels[i+1]=byte(y*4+2)
        src.pixels[i+2]=byte((x+y)*2+3)
        src.pixels[i+3]=255
        dst.pixels[i]=0x5b
        dst.pixels[i+1]=0x5b
        dst.pixels[i+2]=0x5b
        dst.pixels[i+3]=255
    }}
    pGdiFlush.Call()
    const left,top,cw,ch=12,7,22,19
    ok,_,_:=pBlt.Call(dst.dc,left,top,cw,ch,src.dc,left,top,srcCopy)
    if ok==0{t.Fatal("GDI cropped BitBlt failed")}
    pGdiFlush.Call()
    snapshot:=copyRegionBGRA(dst.pixels,width,height,scanRegion{left,top,cw,ch})
    if len(snapshot)!=cw*ch*4{t.Fatal("wrong slice size")}
    for y:=0;y<height;y++{for x:=0;x<width;x++{
        i:=4*(y*width+x)
        inside:=x>=left&&x<left+cw&&y>=top&&y<top+ch
        for c:=0;c<3;c++{
            expected:=byte(0x5b)
            if inside{expected=src.pixels[i+c]}
            if dst.pixels[i+c]!=expected{
                t.Fatalf("cropped GDI pixel changed at %d,%d c%d: %d wanted %d",x,y,c,dst.pixels[i+c],expected)
            }
        }
    }}
}
