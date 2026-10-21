package main

import (
 "image"
 _ "image/jpeg"
 "math"
 "os"
 "testing"
 "time"
)

// Reproduces a real test photograph scaled down to the tiny 16-way RVi
// camera tile: 18px wide and 59px high. A confirmed person moves by 2px
// per screen refresh while the neural worker revisits other cameras.
func smallPhotoOnTile(t *testing.T, left int, present bool) ([]byte,int,int) {
 t.Helper()
 const w,h=240,135
 pixels:=make([]byte,w*h*4)
 for y:=0;y<h;y++{for x:=0;x<w;x++{
  p:=(y*w+x)*4
  v:=uint8(107+(x/26)%6+(y/14)%5)
  pixels[p],pixels[p+1],pixels[p+2],pixels[p+3]=v,v+3,v+6,255
 }}
 if !present{return pixels,w,h}
 f,e:=os.Open("testdata/bus-reference.jpg");if e!=nil{t.Fatal(e)};defer f.Close()
 src,_,e:=image.Decode(f);if e!=nil{t.Fatal(e)}
 const top=44
 for y:=0;y<59;y++{for x:=0;x<18;x++{
  sx,sy:=50+x*196/18,398+y*506/59
  r,g,b,_:=src.At(sx,sy).RGBA()
  p:=((top+y)*w+left+x)*4
  pixels[p],pixels[p+1],pixels[p+2]=byte(b>>8),byte(g>>8),byte(r>>8)
 }}
 return pixels,w,h
}

func TestTinyMovingPhotographicHumanSurvivesLongNeuralGap(t *testing.T){
 start:=time.Unix(170000,0)
 var tr personTracker
 tr.enableConfirmation()
 tr.maxAIAge=8*time.Second
 first,w,h:=smallPhotoOnTile(t,75,true)
 tr.step(first,w,h,start)
 box:=Rect{X:75,Y:44,W:18,H:59,Score:.91}
 tr.observePeopleFromCapture([]Rect{box},nil,first,scanRegion{0,0,w,h},start,start)
 orig:=tr.views(start)
 if len(orig)!=1{t.Fatalf("person not confirmed: %+v",orig)}
 wrong:=0;missing:=0
 for i:=1;i<=23;i++{
  current,_,_:=smallPhotoOnTile(t,75+2*i,true)
  now:=start.Add(time.Duration(i)*100*time.Millisecond)
  tr.step(current,w,h,now)
  got:=tr.views(now)
  if len(got)!=1||got[0].ID!=orig[0].ID{missing++;continue}
  if err:=math.Abs(got[0].Box.X-float64(75+2*i));err>5{wrong++}
 }
 if missing>0||wrong>3{t.Fatalf("small photograph lost under 2.3s neural gap: missing=%d drifted=%d final=%+v",missing,wrong,tr.views(start.Add(2300*time.Millisecond)))}
 blank,_,_:=smallPhotoOnTile(t,0,false)
 for i:=24;i<=45;i++{tr.step(blank,w,h,start.Add(time.Duration(i)*100*time.Millisecond))}
 if got:=tr.views(start.Add(4500*time.Millisecond));len(got)!=0{t.Fatalf("departed photo prolonged as background: %+v",got)}
}
