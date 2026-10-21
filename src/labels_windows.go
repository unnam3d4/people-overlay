//go:build windows

package main

import "strconv"

// Compact pixel font keeps ID labels dependency-free on Windows 7.
var numberGlyphs=[10][5]uint8{
 {7,5,5,5,7}, // 0
 {2,6,2,2,7}, // 1
 {7,1,7,4,7}, // 2
 {7,1,7,1,7}, // 3
 {5,5,7,1,1}, // 4
 {7,4,7,1,7}, // 5
 {7,4,7,5,7}, // 6
 {7,1,1,1,1}, // 7
 {7,5,7,5,7}, // 8
 {7,5,7,1,7}, // 9
}

func drawTrackedCorners(d *dib,tracks []TrackView,color int32,numbers bool){
 boxes:=make([]Rect,0,len(tracks))
 for _,v:=range tracks{boxes=append(boxes,v.Box)}
 drawCorners(d,boxes,color)
 if !numbers{return}
 for _,v:=range tracks{
  if v.ID<1{continue}
  str:=strconv.Itoa(v.ID)
  labelW:=len(str)*8+7
  px:=maxI(0,minI(d.w-labelW,int(v.Box.X+2)))
  py:=maxI(0,minI(d.h-15,int(v.Box.Y)-15))
  if py<=0{py=maxI(0,minI(d.h-15,int(v.Box.Y)+3))}
  for y:=0;y<14;y++{
   for x:=0;x<labelW;x++{plot(d,px+x,py+y,15,20,20)}
  }
  for k:=0;k<len(str);k++{
   digit:=str[k]
   if digit<'0'||digit>'9'{continue}
   glyph:=numberGlyphs[digit-'0']
   for y:=0;y<5;y++{for x:=0;x<3;x++{
    if glyph[y]&(1<<uint(2-x))==0{continue}
    for yy:=0;yy<2;yy++{for xx:=0;xx<2;xx++{
     plot(d,px+4+k*8+x*2+xx,py+2+y*2+yy,245,250,250)
    }}
   }}
  }
 }
}
