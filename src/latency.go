package main

import "math"

// Detection ran on an earlier frame. Shift its box by displacement measured
// by the lightweight optical tracker since that frame. Prevents AI update
// from jumping an otherwise smooth overlay backwards in time.
func compensateLateDetections(found []Rect,atCapture,current []TrackView)[]Rect{
 if len(found)==0||len(atCapture)==0||len(current)==0{return found}
 nowByID:=make(map[int]Rect,len(current))
 for _,v:=range current{nowByID[v.ID]=v.Box}
 out:=append([]Rect(nil),found...)
 for i,d:=range out{
  best,score:=0,0.0
  var first,last Rect
  for _,past:=range atCapture{
   cur,ok:=nowByID[past.ID];if !ok{continue}
   iou:=overlap(d,past.Box)
   dx:=((d.X+d.W*.5)-(past.Box.X+past.Box.W*.5))/math.Max(16,d.W)
   dy:=((d.Y+d.H*.5)-(past.Box.Y+past.Box.H*.5))/math.Max(16,d.H)
   dist:=math.Hypot(dx,dy)
   if iou<.17&&dist>.45{continue}
   quality:=iou*2+1/(1+dist)
   if quality>score{best=past.ID;score=quality;first=past.Box;last=cur}
  }
  if best==0{continue}
  dx:=last.X-first.X
  dy:=last.Y-first.Y
  // Do not apply large shifts caused by ambiguous cross-person matching.
  limit:=math.Max(14,math.Min(64,d.W*.5))
  dx=math.Max(-limit,math.Min(limit,dx))
  limit=math.Max(14,math.Min(64,d.H*.5))
  dy=math.Max(-limit,math.Min(limit,dy))
  out[i].X+=dx
  out[i].Y+=dy
 }
 return out
}
