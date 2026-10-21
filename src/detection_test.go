package main

import "testing"
func TestDecodePeople(t *testing.T){
 n:=8400
 out:=make([]float32,84*n)
 out[0],out[n],out[2*n],out[3*n],out[4*n]=320,320,100,200,.9
 rs:=decodePeople(out,n,640,640,1,0,0,.4)
 if len(rs)!=1||rs[0].X!=270||rs[0].Y!=220{t.Fatalf("bad decode: %#v",rs)}
}
func TestSuppression(t *testing.T){
 rs:=suppressRects([]Rect{{0,0,100,100,.9},{2,2,100,100,.8},{150,150,50,50,.5}},.5,20)
 if len(rs)!=2{t.Fatalf("bad NMS: %#v",rs)}
}
