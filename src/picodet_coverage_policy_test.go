//go:build picodet

package main

import (
    "testing"
    "time"
)

func TestPicoDetCCTVFirstVisitCoverageBurst(t *testing.T) {
    s:=scanScheduler{}
    regs:=regionsForProfile(3200,1800,3)
    if len(regs)!=17 {t.Fatalf("expected 16 tiles and full scan, got %d",len(regs))}
    s.reset(len(regs),3200,1800,3)
    now:=time.Now()
    forced:=0
    for i:=0;i<24;i++{
        candidate:=s.firstVisitRegion(regs)
        if candidate<1 {break}
        got:=s.ensureFirstCoverage(3,regs,1)
        if got!=1 {forced++}
        // Assume the index returned to the scheduler is sent to the worker.
        s.markScanned(got,now,false)
        now=now.Add(130*time.Millisecond)
    }
    if forced<7 {t.Fatalf("PicoDet coverage too conservative, forced=%d",forced)}
    if modelMaxTrackedPeople<50 {t.Fatalf("50+ people cannot be tracked: %d",modelMaxTrackedPeople)}
    if minSelftestPersons!=3 {t.Fatal("weakened real-person regression gate")}
}
func TestPicoDetUnvisitedQuietFeedCannotStarve(t *testing.T) {
    s:=scanScheduler{}
    regs:=regionsForProfile(3200,1800,3)
    s.reset(len(regs),3200,1800,3)
    now:=time.Now()
    // Always revisit the same already populated camera if the policy permits.
    for i:=0;i<36;i++{
        choice:=s.ensureFirstCoverage(3,regs,1)
        s.markScanned(choice,now,false)
        now=now.Add(140*time.Millisecond)
    }
    if s.firstVisitRegion(regs)!=-1 {t.Fatalf("some quiet camera never visited: %+v",s.lastScan)}
}
