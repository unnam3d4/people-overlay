package main

import (
 "testing"
 "time"
)

// The real filmed-person CI revealed that the v20 confirmed state vanished
// between expensive NN crops even though people remained on screen: with an
// 800ms scan cadence, a 700ms AI+visual gap instantly hid all corners.
// Keep short bounded grace for verified humans; still hide stale identities.
func TestConfirmedHumanVisibleBetweenMeasuredNNScans(t *testing.T) {
 start:=time.Unix(260000,0)
 tr:=&personTracker{}
 tr.enableConfirmation()
 tr.maxAIAge=8*time.Second
 tr.observePeople([]Rect{{X:40,Y:20,W:38,H:90,Score:.91}},start)
 if got:=tr.views(start.Add(1050*time.Millisecond));len(got)!=1{
  t.Fatalf("verified person hidden during 800-1100ms sequential NN gap: %+v",got)
 }
 if got:=tr.views(start.Add(1850*time.Millisecond));len(got)!=0{
  t.Fatalf("unverified corners persist beyond bounded loss period: %+v",got)
 }
 if got:=tr.views(start.Add(3*time.Second));len(got)!=0{
  t.Fatalf("no new neural evidence revived expired person: %+v",got)
 }
}
