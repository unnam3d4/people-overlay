package main

import "sort"

type Rect struct {
 X, Y, W, H float64
 Score float32
}
func maxF(a,b float64)float64{if a>b{return a};return b}
func minF(a,b float64)float64{if a<b{return a};return b}
func overlap(a,b Rect)float64{
 x1,y1:=maxF(a.X,b.X),maxF(a.Y,b.Y)
 x2,y2:=minF(a.X+a.W,b.X+b.W),minF(a.Y+a.H,b.Y+b.H)
 w,h:=x2-x1,y2-y1
 if w<=0||h<=0{return 0}
 i:=w*h
 return i/(a.W*a.H+b.W*b.H-i)
}
// YOLOv8 output [1,84,N], person class index 0, RGB 640x640.
func decodePeople(out []float32,n,fullW,fullH int,scale,padX,padY float64,threshold float32)[]Rect{
 if n<=0||len(out)<84*n||scale<=0{return nil}
 cand:=make([]Rect,0,100)
 for i:=0;i<n;i++{
  sc:=out[4*n+i]
  if sc<threshold{continue}
  cx,cy:=float64(out[i]),float64(out[n+i])
  w,h:=float64(out[2*n+i]),float64(out[3*n+i])
  x1:=maxF(0,minF((cx-w/2-padX)/scale,float64(fullW)))
  y1:=maxF(0,minF((cy-h/2-padY)/scale,float64(fullH)))
  x2:=maxF(0,minF((cx+w/2-padX)/scale,float64(fullW)))
  y2:=maxF(0,minF((cy+h/2-padY)/scale,float64(fullH)))
  if x2-x1<6||y2-y1<9{continue}
  cand=append(cand,Rect{X:x1,Y:y1,W:x2-x1,H:y2-y1,Score:sc})
 }
 return suppressRects(cand,.45,80)
}
func suppressRects(in []Rect,iou float64,limit int)[]Rect{
 sort.Slice(in,func(i,j int)bool{return in[i].Score>in[j].Score})
 if len(in)>300{in=in[:300]}
 out:=make([]Rect,0,64)
 for _,r:=range in{
  ok:=true
  for _,p:=range out{if overlap(r,p)>iou{ok=false;break}}
  if ok{out=append(out,r)}
  if len(out)>=limit{break}
 }
 return out
}
