package main

import (
 "testing"
 "time"
)

// A newly appearing person in a previously quiet CCTV tile must not wait
// behind a complete sweep of overdue empty cameras. Existing age-based
// fairness remains in force once the single newly-active tile is checked.
func TestNewMotionOnQuietCCTVTileCanPreemptOverdueIdleFeeds(t *testing.T) {
 const w,h=960,540
 regs:=regionsForProfile(w,h,3)
 var sch scanScheduler
 sch.reset(len(regs),w,h,3)
 now:=time.Unix(250000,0)
 sch.firstDecision=now.Add(-14*time.Second)
 for i:=1;i<len(regs);i++{
  sch.lastScan[i]=now.Add(-14*time.Second)
 }
 const tile=6
 sch.lastScan[tile]=now.Add(-700*time.Millisecond)
 blank:=make([]byte,w*h*4)
 changed:=append([]byte(nil),blank...)
 r:=regs[tile]
 for y:=r.y+30;y<r.y+95;y++{
  for x:=r.x+75;x<r.x+105;x++{
   idx:=(y*w+x)*4
   changed[idx],changed[idx+1],changed[idx+2]=230,230,230
  }
 }
 sch.observe(blank,w,h,regs)
 sch.observe(changed,w,h,regs)
 if got:=sch.nextAdaptive(now,3,nil,regs);got!=tile {
  t.Fatalf("fresh human-sized motion in CCTV tile %d starved by idle sweep (picked %d)",tile,got)
 }
 sch.markScanned(tile,now,true)
 if got:=sch.nextAdaptive(now.Add(100*time.Millisecond),3,nil,regs);got==tile{
  t.Fatal("new-motion urgency stayed sticky and starved all other cameras")
 }
}
