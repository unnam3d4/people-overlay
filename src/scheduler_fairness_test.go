package main

import (
	"testing"
	"time"
)

func TestConfirmationRecheckPrecedesTentativeDeadline(t *testing.T){
 regs:=regionsForProfile(960,540,3);var s scanScheduler;s.reset(len(regs),960,540,3)
 now:=time.Unix(1000,0);s.firstDecision=now
 for i:=1;i<len(regs);i++{s.lastScan[i]=now}
 candidate:=[]TrackView{{ID:1,Box:Rect{X:105,Y:60,W:20,H:55,Score:.4}}}
 s.activity[16]=1
 later:=now.Add(600*time.Millisecond)
 fallback:=s.nextAdaptive(later,3,candidate,regs)
 if fallback==1{t.Fatal("fixture must first prioritize another moving camera")}
 if got:=s.confirmationRecheck(later,candidate,regs,fallback);got!=1{t.Fatalf("candidate cannot get independent confirmation before expiry: camera=%d",got)}
 s.lastScan[9]=now.Add(-14*time.Second)
 if got:=s.confirmationRecheck(later,candidate,regs,9);got!=9{t.Fatal("candidate retry starved an overdue camera")}
}

func TestUnvisitedQuietCamerasAgeWhileOneCameraMoves(t *testing.T) {
	regs := regionsForProfile(960, 540, 3)
	var s scanScheduler
	s.reset(len(regs), 960, 540, 3)
	start := time.Unix(1000, 0)
	visited := map[int]bool{}
	for tick := 0; tick < 80; tick++ {
		s.activity[1] = 1
		now := start.Add(time.Duration(tick) * 500 * time.Millisecond)
		i := s.nextAdaptive(now, 3, nil, regs)
		visited[i] = true
		s.markScanned(i, now, false)
	}
	for i := 1; i < len(regs); i++ {
		if !visited[i] {
			t.Errorf("quiet camera %d was never visited in 40 seconds", i)
		}
	}
}
