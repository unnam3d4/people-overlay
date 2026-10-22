//go:build windows

package main

import (
 "testing"
 "time"
)

func TestTrayIconMouseMessagesAlwaysOpenMenu(t *testing.T) {
 for _,event:=range []uint32{wmRButtonUp,wmContextMenu,wmLButtonUp,wmLButtonDouble}{
  if trayMouseOp(event)!=1{t.Fatalf("icon event %#x was ignored",event)}
 }
 for _,event:=range []uint32{0,0x0200,0x0201,0x0204}{
  if trayMouseOp(event)!=0{t.Fatalf("non-activation event %#x should not open popup",event)}
 }
}

func TestV3UnverifiedCornersHideButTrackRetained(t *testing.T){
 now:=time.Unix(60000,0)
 var tr personTracker
 tr.enableConfirmation()
 tr.layout(1920,1080,4)
 tr.people=[]personTrack{{ID:1,Box:Rect{X:75,Y:65,W:40,H:95},State:trackConfirmed,AIHits:2,LastAI:now,LastVisual:now}}
 if len(tr.views(now))!=1{t.Fatal("confirmed person not drawn")}
 if len(tr.views(now.Add(3100*time.Millisecond)))!=0{t.Fatal("stale 4x4 corners visible after 3 seconds with no AI")}
 if len(tr.people)!=1{t.Fatal("track should survive unseen internally for safe reacquisition")}
 // A fresh neural re-observation re-enables the existing ID.
 tr.people[0].LastAI=now.Add(3200*time.Millisecond)
 tr.people[0].LastVisual=now.Add(3200*time.Millisecond)
 if got:=tr.views(now.Add(3200*time.Millisecond));len(got)!=1||got[0].ID!=1{
  t.Fatalf("valid reacquisition should reuse old ID: %+v",got)
 }
}

func TestV3FirstNegativeNeuralScanHidesGhostImmediately(t *testing.T){
 now:=time.Unix(61000,0)
 var tr personTracker
 tr.enableConfirmation()
 tr.layout(1920,1080,4)
 tr.people=[]personTrack{{ID:7,Box:Rect{X:80,Y:50,W:38,H:87},State:trackConfirmed,AIHits:2,LastAI:now,LastVisual:now}}
 if len(tr.views(now))!=1{t.Fatal("missing initial marker")}
 // This negative region covers the original captured box completely.
 captured:=[]TrackView{{ID:7,Box:tr.people[0].Box}}
 tr.noteRegionScanSnapshot(scanRegion{x:0,y:0,w:480,h:270},now.Add(350*time.Millisecond),now.Add(350*time.Millisecond),1,captured)
 if len(tr.views(now.Add(350*time.Millisecond)))!=0{
  t.Fatal("first reliable AI negative must hide old marker")
 }
 if len(tr.people)!=1{t.Fatal("one negative alone must not destroy track ID")}
 // The second independent scan retires the old person permanently.
 tr.noteRegionScanSnapshot(scanRegion{x:0,y:0,w:480,h:270},now.Add(600*time.Millisecond),now.Add(600*time.Millisecond),1,captured)
 if len(tr.people)!=0{t.Fatal("two negatives must remove ghost track")}
}

func TestV3DisplayDuplicateSilhouettesSuppressStaleID(t *testing.T){
 now:=time.Unix(62000,0)
 var tr personTracker
 tr.enableConfirmation()
 tr.layout(1920,1080,4)
 tr.people=[]personTrack{
  {ID:9,Box:Rect{X:82,Y:55,W:45,H:108},State:trackConfirmed,AIHits:2,LastAI:now,LastVisual:now},
  {ID:10,Box:Rect{X:83,Y:56,W:45,H:108},State:trackConfirmed,AIHits:2,LastAI:now.Add(200*time.Millisecond),LastVisual:now.Add(200*time.Millisecond)},
 }
 got:=tr.views(now.Add(250*time.Millisecond))
 if len(got)!=1 || got[0].ID!=10 {t.Fatalf("same silhouette drawn twice instead of latest AI: %+v",got)}
 // But truly distinct overlapping people remain separate.
 tr.people[1].Box.X=116
 got=tr.views(now.Add(250*time.Millisecond))
 if len(got)!=2 {t.Fatalf("two separate people were incorrectly suppressed: %+v",got)}
}
