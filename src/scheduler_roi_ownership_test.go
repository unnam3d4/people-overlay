package main

import (
  "testing"
  "time"
)

// In real filmed-person 4x4 replay, x=220..240 was predicted from camera 1
// inside the padded crop of camera 2. Likewise detections at y=119..135
// leaked from row 1 into the padded crop of camera 5. Neither camera is
// truly positive; otherwise the scheduler continually revisits empty feeds.
func TestPaddedCCTVCropCannotClaimNeighborPerson(t *testing.T) {
  var s scanScheduler
  s.reset(17,960,540,3)
  at:=time.Unix(300000,0)
  s.markScannedPeople(2,at,[]Rect{
    {X:220,Y:46,W:20,H:88,Score:.78},
    {X:212,Y:42,W:16,H:92,Score:.7},
  })
  if !s.recentlyFound[2].IsZero() {
    t.Fatal("tile 2 claimed tile 1's padded human silhouette")
  }
  if s.lastScan[2]!=at {
    t.Fatal("negative crop was not marked visited")
  }
  s.markScannedPeople(5,at,[]Rect{
    {X:68,Y:119,W:71,H:15,Score:.7},
    {X:240,Y:163,W:15,H:59,Score:.8},
  })
  if !s.recentlyFound[5].IsZero(){
    t.Fatal("tile 5 was credited with people from neighbouring cameras")
  }
  s.markScannedPeople(6,at,[]Rect{{X:320,Y:168,W:31,H:55,Score:.86}})
  if s.recentlyFound[6]!=at {
    t.Fatal("a real human inside tile 6 wasn't credited")
  }
}

func TestSchedulerDoesNotPrioritizePaddingOnlyTrack(t *testing.T) {
  regs:=regionsForProfile(960,540,3)
  var s scanScheduler
  s.reset(len(regs),960,540,3)
  now:=time.Unix(300010,0)
  s.firstDecision=now.Add(-time.Second)
  for i:=1;i<len(regs);i++{s.markScanned(i,now,false)}
  // Candidate actually belongs to tile 1, although its body overlaps
  // tile 2's 12% padded inference region.
  suspect:=[]TrackView{{ID:7,Box:Rect{X:220,Y:45,W:20,H:88,Score:.4}}}
  s.lastScan[1]=now.Add(-2*time.Second)
  s.lastScan[2]=now.Add(-2*time.Second)
  got:=s.confirmationRecheck(now,suspect,regs,3)
  if got!=1 {t.Fatalf("urgent weak confirmation went to padded neighbour tile %d rather than owner tile 1",got)}
}
