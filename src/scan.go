package main

import("math";"time")
type scanRegion struct{x,y,w,h int}
type regionCache struct{found []Rect; updated time.Time}
func zones(w,h int)[5]scanRegion{
 x,y:=w/2,h/2
 return [5]scanRegion{{0,0,w,h},{0,0,x,y},{x,0,w-x,y},{0,y,x,h-y},{x,y,w-x,h-y}}
}
func prepareRegionFrame(src []byte,fullW,fullH int,reg scanRegion)([]float32,float64,float64,float64){
 const n=640
 if fullW<1||fullH<1||reg.w<1||reg.h<1{return nil,0,0,0}
 scale:=math.Min(float64(n)/float64(reg.w),float64(n)/float64(reg.h))
 dw,dh:=int(float64(reg.w)*scale+.5),int(float64(reg.h)*scale+.5)
 if dw>n{dw=n};if dh>n{dh=n}
 px,py:=(n-dw)/2,(n-dh)/2
 out:=make([]float32,3*n*n)
 for i:=range out{out[i]=114.0/255.0}
 for y:=0;y<dh;y++{
  sy:=int((float64(y)+.5)/scale)
  if sy>=reg.h{sy=reg.h-1}
  for x:=0;x<dw;x++{
   sx:=int((float64(x)+.5)/scale)
   if sx>=reg.w{sx=reg.w-1}
   i:=((reg.y+sy)*fullW+reg.x+sx)*4
   p:=(y+py)*n+x+px
   out[p]=float32(src[i+2])/255
   out[n*n+p]=float32(src[i+1])/255
   out[2*n*n+p]=float32(src[i])/255
  }
 }
 return out,scale,float64(px),float64(py)
}
func combineCaches(c [5]regionCache,now time.Time)[]Rect{
 all:=make([]Rect,0,100)
 for _,item:=range c{if !item.updated.IsZero()&&now.Sub(item.updated)<=12*time.Second{all=append(all,item.found...)}}
 return suppressRects(all,.5,80)
}
