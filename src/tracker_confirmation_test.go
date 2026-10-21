package main

import (
	"testing"
	"time"
)

func confirmationTrackerForTest() *personTracker {
	tr := &personTracker{}
	tr.enableConfirmation()
	return tr
}
func candidateViewsForTest(tr *personTracker, at time.Time) []TrackView {
	return tr.schedulerViews(at)
}

func TestSlowWeakCaptureCanWaitForSecondDelayedConfirmation(t *testing.T) {
	tr := confirmationTrackerForTest()
	start := time.Unix(74000, 0)
	frame, w, h := flatCCTVFrame(true)
	found := []Rect{{X: 30, Y: 17, W: 40, H: 61, Score: .4}}
	area := scanRegion{0, 0, w, h}
	delivered := start.Add(4 * time.Second)
	tr.step(frame, w, h, delivered)
	tr.observePeopleFromCapture(found, nil, frame, area, start, delivered)
	candidates := tr.schedulerViews(delivered)
	if len(candidates) != 1 {
		t.Fatalf("slow first result expired before it could be rechecked: %+v", candidates)
	}
	if got := tr.views(delivered); len(got) != 0 {
		t.Fatalf("single slow weak capture was confirmed: %+v", got)
	}
	secondCapture := delivered.Add(600 * time.Millisecond)
	secondDelivery := secondCapture.Add(4 * time.Second)
	tr.step(frame, w, h, secondDelivery.Add(-100*time.Millisecond))
	if got := tr.schedulerViews(secondDelivery.Add(-100 * time.Millisecond)); len(got) != 1 || got[0].ID != candidates[0].ID {
		t.Fatalf("candidate expired while its second inference was running: %+v", got)
	}
	tr.step(frame, w, h, secondDelivery)
	tr.observePeopleFromCapture(found, candidates, frame, area, secondCapture, secondDelivery)
	if got := tr.views(secondDelivery); len(got) != 1 || got[0].ID != candidates[0].ID {
		t.Fatalf("second slow independent capture did not confirm original candidate: %+v", got)
	}
}

func TestUrgentRecheckContainsOnlyTentativeTracks(t *testing.T) {
	tr := confirmationTrackerForTest()
	now := time.Unix(75000, 0)
	tr.observePeople([]Rect{{X: 30, Y: 17, W: 40, H: 61, Score: .4}, {X: 130, Y: 17, W: 40, H: 61, Score: .8}}, now)
	candidates := tr.tentativeViews(now)
	if len(candidates) != 1 || candidates[0].Box.X != 30 {
		t.Fatalf("urgent candidate recheck includes confirmed person: %+v", candidates)
	}
}

func TestSingleWeakDetectionHiddenAndExpiresDespiteStaticPixels(t *testing.T) {
	tr := confirmationTrackerForTest()
	start := time.Unix(70000, 0)
	frame, w, h := flatCCTVFrame(true)
	tr.observePeopleFromCapture([]Rect{{X: 30, Y: 17, W: 40, H: 61, Score: .4}}, nil, frame, scanRegion{0, 0, w, h}, start, start)
	if got := tr.views(start); len(got) != 0 {
		t.Fatalf("one weak detection immediately shown: %+v", got)
	}
	if got := candidateViewsForTest(tr, start); len(got) != 1 {
		t.Fatal("candidate unavailable for neural recheck")
	}
	if tentative, confirmed, lost := tr.statusCounts(start); tentative != 1 || confirmed != 0 || lost != 0 {
		t.Fatalf("wrong candidate counts: %d/%d/%d", tentative, confirmed, lost)
	}
	for i := 1; i <= 16; i++ {
		tr.step(frame, w, h, start.Add(time.Duration(i)*250*time.Millisecond))
	}
	if got := candidateViewsForTest(tr, start.Add(4*time.Second)); len(got) != 0 {
		t.Fatalf("static pixels promoted or retained weak candidate: %+v", got)
	}
}

func TestOnlyDistinctNeuralCaptureConfirmsWeakCandidate(t *testing.T) {
	tr := confirmationTrackerForTest()
	start := time.Unix(71000, 0)
	frame, w, h := flatCCTVFrame(true)
	found := []Rect{{X: 30, Y: 17, W: 40, H: 61, Score: .4}}
	area := scanRegion{0, 0, w, h}
	tr.observePeopleFromCapture(found, nil, frame, area, start, start)
	captured := candidateViewsForTest(tr, start)
	tr.observePeopleFromCapture(found, captured, frame, area, start, start.Add(100*time.Millisecond))
	if got := tr.views(start.Add(100 * time.Millisecond)); len(got) != 0 {
		t.Fatalf("duplicate result confirmed candidate: %+v", got)
	}
	second := start.Add(200 * time.Millisecond)
	tr.observePeopleFromCapture(found, captured, frame, area, second, second.Add(100*time.Millisecond))
	got := tr.views(second.Add(100 * time.Millisecond))
	if len(got) != 1 || got[0].ID != captured[0].ID {
		t.Fatalf("second independent capture did not confirm same ID: %+v", got)
	}
}

func TestStrongDetectionConfirmedImmediately(t *testing.T) {
	tr := confirmationTrackerForTest()
	now := time.Unix(72000, 0)
	tr.observePeople([]Rect{{X: 30, Y: 17, W: 40, H: 61, Score: .8}}, now)
	if got := tr.views(now); len(got) != 1 {
		t.Fatalf("strong detection unnecessarily delayed: %+v", got)
	}
	if tentative, confirmed, lost := tr.statusCounts(now); tentative != 0 || confirmed != 1 || lost != 0 {
		t.Fatalf("wrong confirmed counts: %d/%d/%d", tentative, confirmed, lost)
	}
}

func TestLostTrackHiddenRecoversIDThenExpires(t *testing.T) {
	tr := confirmationTrackerForTest()
	start := time.Unix(73000, 0)
	tr.observePeople([]Rect{{X: 30, Y: 17, W: 40, H: 61, Score: .8}}, start)
	original := tr.views(start)[0].ID
	later := start.Add(time.Second)
	if got := tr.views(later); len(got) != 0 {
		t.Fatalf("lost track remains visible: %+v", got)
	}
	if got := candidateViewsForTest(tr, later); len(got) != 1 || got[0].ID != original {
		t.Fatalf("lost identity not available for recovery: %+v", got)
	}
	if tentative, confirmed, lost := tr.statusCounts(later); tentative != 0 || confirmed != 0 || lost != 1 {
		t.Fatalf("wrong lost counts: %d/%d/%d", tentative, confirmed, lost)
	}
	frame, w, h := flatCCTVFrame(true)
	tr.step(frame, w, h, later)
	tr.step(frame, w, h, later.Add(100*time.Millisecond))
	if got := tr.views(later.Add(100 * time.Millisecond)); len(got) != 1 || got[0].ID != original {
		t.Fatalf("visual recovery changed ID: %+v", got)
	}
	if got := candidateViewsForTest(tr, start.Add(3*time.Second)); len(got) != 0 {
		t.Fatalf("lost track survived expiry: %+v", got)
	}
}
