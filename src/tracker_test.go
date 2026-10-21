package main

import(
 "math"
 "testing"
 "time"
)
func TestTrackingKeepsIDsOnMovement(t *testing.T){
 var tr personTracker
 start:=time.Unix(1000,0)
 tr.observePeople([]Rect{{X:110,Y:90,W:45,H:110,Score:.92},{X:310,Y:120,W:53,H:115,Score:.85}},start)
 a:=tr.views(start)
 if len(a)!=2||a[0].ID==a[1].ID{t.Fatalf("initial IDs: %+v",a)}
 tr.observePeople([]Rect{{X:124,Y:98,W:46,H:111,Score:.9},{X:297,Y:127,W:52,H:116,Score:.88}},start.Add(time.Second))
 b:=tr.views(start.Add(time.Second))
 if len(b)!=2||b[0].ID!=a[0].ID||b[1].ID!=a[1].ID{t.Fatalf("changed IDs: %+v -> %+v",a,b)}
}
func TestMultiplePersonsNoIdentitySwapAcrossEmptyFrame(t *testing.T){
 var tr personTracker
 now:=time.Unix(2000,0)
 tr.observePeople([]Rect{{X:40,Y:80,W:50,H:100,Score:.9},{X:300,Y:80,W:50,H:100,Score:.8}},now)
 a:=tr.views(now)
 tr.observePeople(nil,now.Add(200*time.Millisecond))
 tr.observePeople([]Rect{{X:305,Y:84,W:50,H:100,Score:.94},{X:44,Y:83,W:50,H:100,Score:.89}},now.Add(time.Second))
 b:=tr.views(now.Add(time.Second))
 if b[0].ID!=a[0].ID||b[1].ID!=a[1].ID{t.Fatalf("identity swap: %v -> %v",a,b)}
}
func TestStationaryTrackExpires(t *testing.T){
 var tr personTracker
 now:=time.Unix(3000,0)
 tr.observePeople([]Rect{{X:50,Y:50,W:50,H:80,Score:.9}},now)
 if len(tr.views(now.Add(5*time.Second)))!=1{t.Fatal("expired too early")}
 if len(tr.views(now.Add(6*time.Second)))!=0{t.Fatal("phantom person did not expire")}
}
func TestNoFakePersonWithoutAI(t *testing.T){
 var tr personTracker
 now:=time.Unix(4000,0)
 frame:=make([]byte,320*240*4)
 tr.step(frame,320,240,now)
 for i:=range frame{frame[i]=255}
 tr.step(frame,320,240,now.Add(120*time.Millisecond))
 if len(tr.views(now.Add(time.Second)))!=0{t.Fatal("tracking invented a person without AI")}
}
func syntheticPatch(dx,dy int)([]byte,int,int){
 w,h:=120,100
 frame:=make([]byte,w*h*4)
 for y:=0;y<h;y++{for x:=0;x<w;x++{
  z:=(y*w+x)*4
  if x>=30+dx&&x<70+dx&&y>=17+dy&&y<78+dy{
   value:=uint8(((x-dx)*17+(y-dy)*29)%161+83)
   frame[z],frame[z+1],frame[z+2]=value,value,value
  }else{
   frame[z],frame[z+1],frame[z+2]=18,18,18
  }
 }}
 return frame,w,h
}
func TestImageTrackerFollowsMovingSilhouette(t *testing.T){
 var tr personTracker
 start:=time.Unix(5000,0)
 tr.observePeople([]Rect{{X:30,Y:17,W:40,H:61,Score:.9}},start)
 one,w,h:=syntheticPatch(0,0)
 two,_,_:=syntheticPatch(8,4)
 tr.step(one,w,h,start)
 tr.step(two,w,h,start.Add(130*time.Millisecond))
 out:=tr.views(start.Add(130*time.Millisecond))
 if len(out)!=1{t.Fatalf("person disappeared %+v",out)}
 if math.Abs(out[0].Box.X-38)>8||math.Abs(out[0].Box.Y-21)>8{t.Fatalf("tracking failed to follow: %+v",out)}
}
