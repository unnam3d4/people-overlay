package main

import (
    "testing"
    "time"
)

func TestStaleMovingPersonDoesNotShowTwoSetsOfCorners(t *testing.T) {
    now:=time.Unix(800000,0)
    var tr personTracker
    tr.enableConfirmation()
    tr.layout(1920,1080,4)
    old:=personTrack{
        ID:10,Tile:6,Box:Rect{X:560,Y:265,W:44,H:110,Score:.91},
        State:trackConfirmed,AIHits:4,LastAI:now.Add(-1450*time.Millisecond),
        LastVisual:now, // old background still (incorrectly) matches
    }
    replacement:=personTrack{
        ID:11,Tile:6,Box:Rect{X:581,Y:268,W:45,H:112,Score:.90},
        State:trackConfirmed,AIHits:1,LastAI:now.Add(-200*time.Millisecond),
        LastVisual:now,
    }
    tr.people=[]personTrack{old,replacement}
    views:=tr.views(now)
    if len(views)!=1 || views[0].ID!=11 {
        t.Fatalf("same moving person drawn twice or older stale marker won: %+v",views)
    }
    if len(tr.people)!=2 {t.Fatal("visual suppression must preserve underlying tracking identities")}
}

func TestTwoPeopleCrossingRemainTwoEvenWhenBoxesOverlap(t *testing.T){
    now:=time.Unix(800100,0)
    var tr personTracker
    tr.enableConfirmation()
    tr.layout(1920,1080,4)
    tr.people=[]personTrack{
        {ID:20,Tile:3,Box:Rect{X:1000,Y:100,W:54,H:112},State:trackConfirmed,AIHits:4,LastAI:now.Add(-1100*time.Millisecond),LastVisual:now},
        {ID:21,Tile:3,Box:Rect{X:1027,Y:102,W:54,H:112},State:trackConfirmed,AIHits:3,LastAI:now.Add(-150*time.Millisecond),LastVisual:now},
    }
    v:=tr.views(now)
    if len(v)!=2 {t.Fatalf("two overlapping but independently verified humans were merged: %+v",v)}
}

func TestOverlappingCameraStreamsNotDeduplicatedTogether(t *testing.T){
    now:=time.Unix(800200,0)
    var tr personTracker
    tr.enableConfirmation()
    tr.layout(1920,1080,4)
    tr.people=[]personTrack{
        {ID:3,Tile:6,Box:Rect{X:455,Y:256,W:42,H:94},State:trackConfirmed,AIHits:3,LastAI:now.Add(-1300*time.Millisecond),LastVisual:now},
        {ID:4,Tile:7,Box:Rect{X:456,Y:255,W:42,H:94},State:trackConfirmed,AIHits:1,LastAI:now.Add(-100*time.Millisecond),LastVisual:now},
    }
    if got:=tr.views(now);len(got)!=2 {t.Fatalf("detections from different camera tiles collapsed: %+v",got)}
}

func TestNewOldTrackNotMergedWithoutSufficientGeometricEvidence(t *testing.T){
    now:=time.Unix(800300,0)
    old:=personTrack{Tile:5,Box:Rect{X:100,Y:100,W:42,H:98},LastAI:now.Add(-1300*time.Millisecond),AIHits:3}
    fresh:=personTrack{Tile:5,Box:Rect{X:162,Y:100,W:42,H:98},LastAI:now.Add(-100*time.Millisecond),AIHits:1}
    if displayDuplicateTracks(old,fresh,now) {
        t.Fatal("one person walking past another must not be collapsed")
    }
}

func TestThreeTimeStaggeredFramesOfOnePersonShowOneMarker(t *testing.T){
    now:=time.Unix(800400,0)
    var tr personTracker
    tr.enableConfirmation()
    tr.layout(1920,1080,4)
    tr.people=[]personTrack{
        {ID:31,Tile:7,Box:Rect{X:1000,Y:150,W:44,H:110},State:trackConfirmed,AIHits:3,LastAI:now.Add(-1700*time.Millisecond),LastVisual:now},
        {ID:32,Tile:7,Box:Rect{X:1020,Y:151,W:44,H:110},State:trackConfirmed,AIHits:1,LastAI:now.Add(-700*time.Millisecond),LastVisual:now},
        {ID:33,Tile:7,Box:Rect{X:1031,Y:151,W:44,H:110},State:trackConfirmed,AIHits:1,LastAI:now.Add(-100*time.Millisecond),LastVisual:now},
    }
    got:=tr.views(now)
    if len(got)!=1||got[0].ID!=33 {
        t.Fatalf("three overlapping time-staggered IDs rendered on one pedestrian: %+v",got)
    }
}
