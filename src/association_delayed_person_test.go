package main

import ("testing";"time")

// Captured real PETS pedestrian traverses ~40-60 screen pixels before the
// next CPU inference returns. Current .79-body-width matching barrier made
// dozens of new IDs from eight filmed people despite correct YOLO detections.
func TestPersonAssociationAcrossMeasuredNeuralDelay(t *testing.T) {
 now:=time.Unix(400000,0)
 prior:=personTrack{
   Box:Rect{X:105,Y:160,W:32,H:78,Score:.85},
   ID:9,Tile:2,LastAI:now.Add(-1350*time.Millisecond),
   LastVisual:now.Add(-580*time.Millisecond),
 }
 moved:=Rect{X:148,Y:159,W:33,H:79,Score:.89}
 if score:=trackMatchScore(prior,moved,2,now);score<=0{
  t.Fatalf("same-body person moved 43px over 1.35s, association lost, score %.3f",score)
 }
 // A different camera is not the same person.
 if score:=trackMatchScore(prior,moved,3,now);score>0{
  t.Fatalf("human transferred between different CCTV feeds: %.3f",score)
 }
}

func TestDelayedGlobalAssignmentPrefersConsistentPedestrianTrajectories(t *testing.T) {
 now:=time.Unix(400100,0)
 tracks:=[]personTrack{
  {ID:1,Tile:0,LastAI:now.Add(-1200*time.Millisecond),Box:Rect{X:100,Y:160,W:32,H:78,Score:.86}},
  {ID:2,Tile:0,LastAI:now.Add(-1200*time.Millisecond),Box:Rect{X:185,Y:160,W:36,H:82,Score:.85}},
 }
 observations:=[]Rect{
  {X:145,Y:160,W:32,H:78,Score:.86},
  {X:225,Y:160,W:36,H:82,Score:.85},
 }
 var tracker personTracker
 tracker.people=tracks
 ids:=tracker.assignDetectionsAt(observations,now,now.Add(-300*time.Millisecond))
 if len(ids)!=2||ids[0]!=1||ids[1]!=2{
  t.Fatalf("same moving pedestrians acquired new or swapped IDs after AI gap: %v",ids)
 }
}
