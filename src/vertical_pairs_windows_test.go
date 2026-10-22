//go:build windows

package main

import "testing"

func TestVerticalPairedCameraRegionsBoundsAndCoverage(t *testing.T) {
    regs:=benchmarkVerticalPairRegions(960,540)
    if len(regs)!=9 {t.Fatalf("8 paired camera groups plus full screen expected: %d",len(regs))}
    seen:=map[[2]int]bool{}
    for i,r:=range regs {
        if r.x<0||r.y<0||r.w<=0||r.h<=0||r.x+r.w>960||r.y+r.h>540 {
            t.Fatalf("out of bounds paired crop %d: %+v",i,r)
        }
        if i==0 {continue}
        centerX:=((i-1)%4)*240+120
        centerY:=((i-1)/4)*270+135
        if centerX<r.x||centerX>=r.x+r.w||centerY<r.y||centerY>=r.y+r.h {
            t.Fatalf("lost nominal two-camera center: crop=%d %+v",i,r)
        }
        seen[[2]int{(i-1)%4,(i-1)/4}]=true
    }
    if len(seen)!=8 {t.Fatal("not all eight groups represented")}
}
func TestVerticalPairOwnerMatchesFourByFourCameraTiles(t *testing.T){
    regs:=benchmarkVerticalPairRegions(960,540)
    s:=scanScheduler{}
    s.reset(len(regs),960,540,3)
    for row:=0;row<4;row++{
        for col:=0;col<4;col++ {
            box:=Rect{X:float64(col*240+40),Y:float64(row*135+25),W:30,H:40}
            expected:=(row/2)*4+col+1
            if owner:=s.scanOwner(box);owner!=expected {
                t.Fatalf("camera row %d col %d expected paired ROI %d got %d",row,col,expected,owner)
            }
        }
    }
    regular:=scanScheduler{}
    regular.reset(17,960,540,3)
    if regular.scanOwner(Rect{X:40,Y:160,W:20,H:50})!=5 {
        t.Fatal("ordinary 16 camera owner must remain unchanged")
    }
}
