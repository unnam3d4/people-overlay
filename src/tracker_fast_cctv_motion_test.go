package main

import ("math";"testing")

func TestFastCCTVHumanMovementAcrossOneLowFPSFrame(t *testing.T){
 first,w,h:=smallPhotoOnTile(t,75,true)
 next,_,_:=smallPhotoOnTile(t,89,true)
 pa,pw,ph:=grayscaleQuarter(first,w,h)
 pb,_,_:=grayscaleQuarter(next,w,h)
 body:=Rect{X:75,Y:44,W:18,H:59,Score:.9}
 dx,dy,ok:=motionShift(pa,pb,pw,ph,body)
 if !ok||math.Abs(dx-14)>2||math.Abs(dy)>2{
  t.Fatalf("moving human shifted 14 pixels at 7fps, block tracker only found dx=%v dy=%v ok=%v",dx,dy,ok)
 }
}

func TestExpandedCCTVMotionRejectsVacatedHuman(t *testing.T){
 first,w,h:=smallPhotoOnTile(t,75,true)
 blank,_,_:=smallPhotoOnTile(t,0,false)
 pa,pw,ph:=grayscaleQuarter(first,w,h)
 pb,_,_:=grayscaleQuarter(blank,w,h)
 body:=Rect{X:75,Y:44,W:18,H:59,Score:.9}
 _,_,ok:=motionShift(pa,pb,pw,ph,body)
 if ok{t.Fatal("empty corridor accepted as tracked human under expanded motion search")}
}
