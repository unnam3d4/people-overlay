//go:build picodet

package main

import (
  "testing"
  "time"
)

func TestPicoDetKeepVerifiedHumanDuringLongCCTVScan(t *testing.T) {
 now:=time.Unix(5555,0)
 good:=personTrack{
  Box:Rect{X:20,Y:20,W:60,H:100},ID:1,
  State:trackConfirmed,LastAI:now.Add(-3300*time.Millisecond),
  LastVisual:now.Add(-80*time.Millisecond),AnchorValid:true,
 }
 tr:=personTracker{confirmationRequired:true,columns:4,maxAIAge:9*time.Second,people:[]personTrack{good}}
 got:=tr.views(now)
 if len(got)!=1 {t.Fatalf("recently visually verified human blinked after 3.3s AI: %+v",got)}
}

func TestPicoDetVisualRevalidationNeverKeepsStaleOrNegativeGhost(t *testing.T) {
 now:=time.Unix(5555,0)
 p:=personTrack{
  Box:Rect{X:20,Y:20,W:60,H:100},ID:1,State:trackConfirmed,
  LastAI:now.Add(-3300*time.Millisecond),
  LastVisual:now.Add(-80*time.Millisecond),AnchorValid:true,
 }
 for _,tc:=range []struct{name string;value personTrack}{
  {"stale_visual",func()personTrack{v:=p;v.LastVisual=now.Add(-600*time.Millisecond);return v}()},
  {"no_anchor",func()personTrack{v:=p;v.AnchorValid=false;return v}()},
  {"negative_ai",func()personTrack{v:=p;v.AIMisses=1;return v}()},
  {"expired_neural",func()personTrack{v:=p;v.LastAI=now.Add(-4700*time.Millisecond);return v}()},
 }{
   tr:=personTracker{confirmationRequired:true,columns:4,maxAIAge:9*time.Second,people:[]personTrack{tc.value}}
   if got:=tr.views(now);len(got)!=0 {t.Errorf("%s leaked ghost after cutoff: %+v",tc.name,got)}
 }
}
