package main

import (
 "math"
 "testing"
)

func TestSparseFlowFollowsSmallFilmedPersonWithoutYOLOWait(t *testing.T){
 a,w,h:=smallPhotoOnTile(t,75,true)
 b,_,_:=smallPhotoOnTile(t,81,true)
 pa,pw,ph:=grayscaleQuarter(a,w,h)
 pb,_,_:=grayscaleQuarter(b,w,h)
 box:=Rect{X:75,Y:44,W:18,H:59,Score:.85}
 dx,dy,ok:=sparsePersonFlow(pa,pb,pw,ph,box)
 if !ok||math.Abs(dx-6)>2.5||math.Abs(dy)>2.5 {
  t.Fatalf("sparse flow lost 6px filmed person step dx=%v dy=%v valid=%v",dx,dy,ok)
 }
}

func TestSparseFlowRejectsPersonToEmptyBackground(t *testing.T){
 a,w,h:=smallPhotoOnTile(t,75,true)
 b,_,_:=smallPhotoOnTile(t,75,false)
 pa,pw,ph:=grayscaleQuarter(a,w,h)
 pb,_,_:=grayscaleQuarter(b,w,h)
 box:=Rect{X:75,Y:44,W:18,H:59,Score:.85}
 if dx,dy,ok:=sparsePersonFlow(pa,pb,pw,ph,box);ok{
  t.Fatalf("empty camera falsely matched by person sparse flow dx=%v dy=%v",dx,dy)
 }
}

func TestSparseFlowIsStillOnRepeatedFrame(t *testing.T){
 a,w,h:=smallPhotoOnTile(t,75,true)
 pa,pw,ph:=grayscaleQuarter(a,w,h)
 box:=Rect{X:75,Y:44,W:18,H:59,Score:.85}
 dx,dy,ok:=sparsePersonFlow(pa,pa,pw,ph,box)
 if ok&&(dx!=0||dy!=0) {t.Fatalf("motion from identical repeated frame dx=%v dy=%v",dx,dy)}
}

func BenchmarkSparsePersonFlowTinyCCTV(b *testing.B){
 // Non-concurrent CV CPU benchmark (no NCNN, no video reader).
 w,h:=240,135
 a:=make([]byte,w*h*4)
 for i:=0;i<len(a);i+=4{a[i],a[i+1],a[i+2],a[i+3]=100,100,100,255}
 previous,pw,ph:=grayscaleQuarter(a,w,h)
 current:=append([]byte(nil),previous...)
 rect:=Rect{X:75,Y:44,W:18,H:59}
 b.ResetTimer()
 for i:=0;i<b.N;i++{_,_,_=sparsePersonFlow(previous,current,pw,ph,rect)}
}
