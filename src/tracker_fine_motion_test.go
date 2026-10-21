package main

import (
    "math"
    "testing"
)

func TestAIVerifiedFineMotionFollowsTwoPixelCCTVMove(t *testing.T) {
    first,w,h:=smallPhotoOnTile(t,75,true)
    second,_,_:=smallPhotoOnTile(t,77,true)
    gray,w2,h2:=grayscaleQuarter(second,w,h)
    rect:=Rect{X:75,Y:44,W:18,H:59,Score:.9}
    a,valid:=sampleCaptureAnchor(first,scanRegion{0,0,w,h},rect)
    if !valid { t.Fatal("photographic anchor not sampled") }
    dx,dy,ok:=anchorFineMotion(a,gray,w2,h2,rect)
    if !ok||math.Abs(dx-2)>0.1||dy!=0 {
        t.Fatalf("valid 2px motion not recovered dx=%v dy=%v ok=%v",dx,dy,ok)
    }
}

func TestAIVerifiedFineMotionNeverAttachesToVacantCamera(t *testing.T) {
    first,w,h:=smallPhotoOnTile(t,75,true)
    blank,_,_:=smallPhotoOnTile(t,0,false)
    gray,w2,h2:=grayscaleQuarter(blank,w,h)
    rect:=Rect{X:75,Y:44,W:18,H:59,Score:.9}
    a,valid:=sampleCaptureAnchor(first,scanRegion{0,0,w,h},rect)
    if !valid { t.Fatal("initial person anchor missing") }
    if dx,dy,ok:=anchorFineMotion(a,gray,w2,h2,rect);ok {
        t.Fatalf("empty background accepted as person motion (%v,%v)",dx,dy)
    }
}

func TestAIVerifiedFineMotionDoesNotMoveStationaryPerson(t *testing.T){
    pixels,w,h:=smallPhotoOnTile(t,75,true)
    gray,w2,h2:=grayscaleQuarter(pixels,w,h)
    rect:=Rect{X:75,Y:44,W:18,H:59,Score:.9}
    a,valid:=sampleCaptureAnchor(pixels,scanRegion{0,0,w,h},rect)
    if !valid {t.Fatal("person texture invalid")}
    dx,dy,ok:=anchorFineMotion(a,gray,w2,h2,rect)
    if !ok||dx!=0||dy!=0{
        t.Fatalf("stationary person drifted by %v,%v ok %v",dx,dy,ok)
    }
}
