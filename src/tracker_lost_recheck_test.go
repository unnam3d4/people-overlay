package main

import (
 "testing"
 "time"
)

// Lost verified people were hidden from the urgent camera queue. On a 4x4
// layout the AI could keep visiting empty tiles until the identity expired,
// at which point the same human reappeared with a new ID.
func TestLostConfirmedHumanGetsUrgentCCTVNeuralRecheck(t *testing.T) {
 const w,h=960,540
 start:=time.Unix(200000,0)
 regs:=regionsForProfile(w,h,3)
 tile:=6
 r:=regs[tile]
 box:=Rect{X:float64(r.x)+float64(r.w)*.33,Y:float64(r.y)+float64(r.h)*.25,W:float64(r.w)*.24,H:float64(r.h)*.55,Score:.88}
 var tr personTracker
 tr.enableConfirmation()
 tr.layout(w,h,4)
 tr.observePeople([]Rect{box},start)
 original:=tr.views(start)
 if len(original)!=1 {t.Fatalf("strong person not confirmed: %+v",original)}
 now:=start.Add(time.Second)
 if views:=tr.views(now);len(views)!=0 {t.Fatalf("lost visuals must not keep old overlay visible: %+v",views)}
 pending:=tr.tentativeViews(now)
 if len(pending)!=1||pending[0].ID!=original[0].ID{t.Fatalf("lost verified person not sent for urgent recheck: %+v",pending)}
 var scheduler scanScheduler
 scheduler.reset(len(regs),w,h,3)
 scheduler.firstDecision=start
 scheduler.markScanned(tile,start,true)
 scheduler.markScanned(1,start.Add(100*time.Millisecond),false)
 if got:=scheduler.confirmationRecheck(now,pending,regs,1);got!=tile{
  t.Fatalf("lost person camera should preempt normal low-activity scan: got tile %d wanted %d",got,tile)
 }
 // Epoch/capture invariants still apply to the accelerated scan.
 tr.noteRegionScanSnapshot(regs[tile],start.Add(1100*time.Millisecond),start.Add(50*time.Millisecond),tile,original)
 if got:=tr.schedulerViews(start.Add(1100*time.Millisecond));len(got)!=1{
  t.Fatalf("one negative capture prematurely erased lost person: %+v",got)
 }
}
