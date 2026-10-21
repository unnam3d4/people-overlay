package main

import (
	"testing"
	"time"
)

func observeCaptureForTest(tr *personTracker, found []Rect, captured []TrackView, pixels []byte, region scanRegion, at, now time.Time) {
	tr.observePeopleFromCapture(found, captured, pixels, region, at, now)
}
func noteSnapshotForTest(tr *personTracker, region scanRegion, now, at time.Time, captured []TrackView) {
	tr.noteRegionScanSnapshot(region, now, at, 0, captured)
}

func TestSlowPositiveCaptureRetainsStillVisibleHuman(t *testing.T) {
	var tr personTracker
	start := time.Unix(67000, 0)
	human, w, h := flatCCTVFrame(true)
	arrived := start.Add(3 * time.Second)
	tr.step(human, w, h, arrived)
	observeCaptureForTest(&tr, []Rect{{X: 30, Y: 17, W: 40, H: 61, Score: .9}}, nil, human, scanRegion{0, 0, w, h}, start, arrived)
	if got := tr.views(arrived); len(got) != 1 {
		t.Fatalf("slow inference cannot discover a person still visible on screen: %+v", got)
	}
}

func TestOldPositiveCaptureDoesNotRestartPersonLifetime(t *testing.T) {
	var tr personTracker
	start := time.Unix(63000, 0)
	frame, w, h := flatCCTVFrame(true)
	observeCaptureForTest(&tr, []Rect{{X: 30, Y: 17, W: 40, H: 61, Score: .9}}, nil, frame, scanRegion{0, 0, w, h}, start, start.Add(3*time.Second))
	if got := tr.views(start.Add(3 * time.Second)); len(got) != 0 {
		t.Fatalf("old inference restarted disappeared person's lifetime: %+v", got)
	}
}

func TestDelayedCaptureKeepsHumanAnchorInsteadOfEmptyBackground(t *testing.T) {
	var tr personTracker
	start := time.Unix(64000, 0)
	tr.maxAIAge = 20 * time.Second
	human, w, h := flatCCTVFrame(true)
	blank, _, _ := flatCCTVFrame(false)
	tr.step(blank, w, h, start.Add(500*time.Millisecond))
	observeCaptureForTest(&tr, []Rect{{X: 30, Y: 17, W: 40, H: 61, Score: .9}}, nil, human, scanRegion{0, 0, w, h}, start, start.Add(500*time.Millisecond))
	for i := 3; i <= 16; i++ {
		tr.step(blank, w, h, start.Add(time.Duration(i)*250*time.Millisecond))
	}
	if got := tr.views(start.Add(4 * time.Second)); len(got) != 0 {
		t.Fatalf("delayed inference anchored itself to empty background: %+v", got)
	}
}

func TestNegativeCaptureUsesPersonPositionAtCapture(t *testing.T) {
	var tr personTracker
	start := time.Unix(65000, 0)
	tr.observePeople([]Rect{{X: 180, Y: 20, W: 40, H: 60, Score: .9}}, start)
	captured := tr.views(start.Add(100 * time.Millisecond))
	tr.people[0].Box.X = 30
	region := scanRegion{0, 0, 120, 100}
	noteSnapshotForTest(&tr, region, start.Add(300*time.Millisecond), start.Add(100*time.Millisecond), captured)
	noteSnapshotForTest(&tr, region, start.Add(500*time.Millisecond), start.Add(200*time.Millisecond), captured)
	if got := tr.views(start.Add(500 * time.Millisecond)); len(got) != 1 {
		t.Fatalf("empty crop killed person who entered after capture: %+v", got)
	}
}

func TestCaptureAnchorUsesScreenPixelAlignment(t *testing.T) {
	var tr personTracker
	start := time.Unix(66000, 0)
	frame, w, h := syntheticPatch(0, 0)
	region := scanRegion{1, 3, 110, 90}
	crop := copyRegionBGRA(frame, w, h, region)
	observeCaptureForTest(&tr, []Rect{{X: 30, Y: 17, W: 40, H: 61, Score: .9}}, nil, crop, region, start, start.Add(100*time.Millisecond))
	for i := 1; i <= 16; i++ {
		tr.step(frame, w, h, start.Add(time.Duration(i)*250*time.Millisecond))
	}
	if got := tr.views(start.Add(4 * time.Second)); len(got) != 1 {
		t.Fatalf("unaligned camera crop rejected an unchanged real person: %+v", got)
	}
}

func TestMovingBackgroundCannotRefreshDepartedPerson(t *testing.T) {
	var tr personTracker
	start := time.Unix(60000, 0)
	tr.maxAIAge = 20 * time.Second
	tr.observePeople([]Rect{{X: 30, Y: 17, W: 40, H: 61, Score: .9}}, start)
	human, w, h := flatCCTVFrame(true)
	tr.step(human, w, h, start)
	// The person leaves. A different textured object/background moves in
	// the vacated area; consecutive-frame motion must not validate identity.
	for i := 1; i <= 16; i++ {
		frame, _, _ := syntheticPatch((i%2)*4, 0)
		tr.step(frame, w, h, start.Add(time.Duration(i)*250*time.Millisecond))
	}
	if got := tr.views(start.Add(4 * time.Second)); len(got) != 0 {
		t.Fatalf("unrelated moving texture kept departed person alive: %+v", got)
	}
}

func TestEstablishedPeopleKeepBothIDsDuringExactOverlap(t *testing.T) {
	start := time.Unix(61000, 0)
	var tr personTracker
	tr.observePeople([]Rect{{X: 100, Y: 100, W: 50, H: 120, Score: .9}, {X: 135, Y: 100, W: 50, H: 120, Score: .8}}, start)
	first, second := tr.people[0].ID, tr.people[1].ID
	// Optical tracking brings two already distinct people into occlusion.
	tr.people[0].Box.X = 118
	tr.people[1].Box.X = 119
	tr.observePeople([]Rect{{X: 118, Y: 100, W: 50, H: 120, Score: .9}}, start.Add(200*time.Millisecond))
	got := tr.views(start.Add(200 * time.Millisecond))
	if len(got) != 2 || got[0].ID != first || got[1].ID != second {
		t.Fatalf("temporary overlap destroyed established identity: %+v", got)
	}
}

func TestNegativeScanCaptureCountsOnlyOnce(t *testing.T) {
	start := time.Unix(62000, 0)
	var tr personTracker
	tr.observePeople([]Rect{{X: 30, Y: 20, W: 40, H: 60, Score: .9}}, start)
	region := scanRegion{x: 0, y: 0, w: 120, h: 100}
	capture := start.Add(100 * time.Millisecond)
	tr.noteRegionScanCaptured(region, start.Add(200*time.Millisecond), capture, 0)
	tr.noteRegionScanCaptured(region, start.Add(300*time.Millisecond), capture, 0)
	if got := tr.views(start.Add(300 * time.Millisecond)); len(got) != 1 {
		t.Fatalf("repeated result counted as independent negative scan: %+v", got)
	}
	tr.noteRegionScanCaptured(region, start.Add(500*time.Millisecond), start.Add(400*time.Millisecond), 0)
	if got := tr.views(start.Add(500 * time.Millisecond)); len(got) != 0 {
		t.Fatalf("two independent negative scans did not retire track: %+v", got)
	}
}
