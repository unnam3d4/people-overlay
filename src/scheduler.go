package main

import (
 "math"
 "time"
)

// scanScheduler chooses which CCTV tile deserves the next expensive AI
// inference. The low-cost motion pass runs on snapshots of the screen.
type scanScheduler struct {
 lastScan []time.Time
 fingerprint [][]uint8
 activity []float64
 recentlyFound []time.Time
 previousProfile int32
 width,height int
 lastFull time.Time
}

func (s *scanScheduler)reset(regions int,w,h int,profile int32){
 s.lastScan=make([]time.Time,regions)
 s.fingerprint=make([][]uint8,regions)
 s.activity=make([]float64,regions)
 s.recentlyFound=make([]time.Time,regions)
 s.previousProfile=profile;s.width=w;s.height=h
 s.lastFull=time.Time{}
}

// Each tile uses 16x12 grayscale samples. Even 4x4 tiled search needs
// fewer than 3,100 pixel reads per screenshot, not another AI model.
func tinySignature(src []byte,w,h int,r scanRegion)[]uint8{
 const cols,rows=16,12
 sig:=make([]uint8,cols*rows)
 if len(src)<w*h*4||w<=0||h<=0||r.w<1||r.h<1{return sig}
 for y:=0;y<rows;y++{
  py:=r.y+(2*y+1)*r.h/(2*rows)
  if py>=h{py=h-1}
  for x:=0;x<cols;x++{
   px:=r.x+(2*x+1)*r.w/(2*cols)
   if px>=w{px=w-1}
   p:=(py*w+px)*4
   sig[y*cols+x]=uint8((int(src[p])*11+int(src[p+1])*59+int(src[p+2])*30)/100)
  }
 }
 return sig
}
func movementScore(before,after []uint8)float64{
 if len(before)!=len(after)||len(after)==0{return 0}
 changed:=0
 for i,v:=range after{
  d:=int(v)-int(before[i])
  if d < -18||d>18{changed++}
 }
 fraction:=float64(changed)/float64(len(after))
 // Small moving silhouettes may occupy only a few samples.
 if fraction<.008{return 0}
 return math.Min(1,fraction)
}
func (s *scanScheduler)observe(frame []byte,w,h int,regions []scanRegion){
 if len(regions)!=len(s.lastScan)||w!=s.width||h!=s.height{return}
 for i:=1;i<len(regions);i++{
  cur:=tinySignature(frame,w,h,regions[i])
  if s.fingerprint[i]!=nil{
   measured:=movementScore(s.fingerprint[i],cur)
   // Motion persists briefly through occasional compression/decoding stalls.
   s.activity[i]=math.Max(measured,s.activity[i]*.48)
  }
  s.fingerprint[i]=cur
 }
}
func (s *scanScheduler)markScanned(index int,now time.Time,detected bool){
 if index<0||index>=len(s.lastScan){return}
 s.lastScan[index]=now
 if index==0{s.lastFull=now}
 if detected{s.recentlyFound[index]=now}
}
func (s *scanScheduler)next(now time.Time,profile int32)int{
 if len(s.lastScan)<2{return 0}
 fullInterval:=12*time.Second
 if profile==1{fullInterval=5*time.Second}
 if profile==2{fullInterval=9*time.Second}
 if s.lastFull.IsZero()||now.Sub(s.lastFull)>=fullInterval{return 0}

 best,bestScore:=1,-1.0
 for i:=1;i<len(s.lastScan);i++{
  age:=120.0
  if !s.lastScan[i].IsZero(){age=now.Sub(s.lastScan[i]).Seconds()}
  if age<0{age=0}
  // Age grows unbounded until this tile is checked, ensuring non-moving
  // persons are still inspected even with constant motion elsewhere.
  score:=math.Min(age,120)*2.0+s.activity[i]*115
  if !s.recentlyFound[i].IsZero()&&now.Sub(s.recentlyFound[i])<4*time.Second{score+=10}
  if score>bestScore{bestScore,best=score,i}
 }
 return best
}
func throttleFor(profile int32,busy time.Duration)time.Duration{
 if busy<=0{busy=100*time.Millisecond}
 var factor float64
 var minimum,maximum time.Duration
 switch profile{
 case 1:factor,minimum,maximum=1.7,240*time.Millisecond,3*time.Second
 case 3:factor,minimum,maximum=.16,60*time.Millisecond,850*time.Millisecond
 default:factor,minimum,maximum=.55,140*time.Millisecond,1500*time.Millisecond
 }
 delay:=time.Duration(float64(busy)*factor)
 if delay<minimum{delay=minimum}
 if delay>maximum{delay=maximum}
 return delay
}
