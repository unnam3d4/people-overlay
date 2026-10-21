package main

import (
	"image"
	_ "image/jpeg"
	"math"
	"os"
	"testing"
	"time"
)

func photographicTrackingFrame(t *testing.T, positions ...int) ([]byte, int, int) {
	t.Helper()
	f, err := os.Open("testdata/bus-reference.jpg")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	photo, _, err := image.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	const w, h = 400, 250
	pixels := make([]byte, w*h*4)
	for i := 0; i < len(pixels); i += 4 {
		pixels[i], pixels[i+1], pixels[i+2], pixels[i+3] = 80, 85, 90, 255
	}
	for _, left := range positions {
		for y := 0; y < 180; y++ {
			for x := 0; x < 70; x++ {
				r, g, b, _ := photo.At(50+x*196/70, 398+y*506/180).RGBA()
				p := ((40+y)*w + left + x) * 4
				pixels[p], pixels[p+1], pixels[p+2] = byte(b>>8), byte(g>>8), byte(r>>8)
			}
		}
	}
	return pixels, w, h
}

func TestDelayedPhotographicDetectionCatchesUpBeforeMotionTracking(t *testing.T) {
	start := time.Unix(90000, 0)
	capture, w, h := photographicTrackingFrame(t, 100)
	current, _, _ := photographicTrackingFrame(t, 110)
	later, _, _ := photographicTrackingFrame(t, 116)
	box := Rect{X: 100, Y: 40, W: 70, H: 180, Score: .9}
	var tr personTracker
	tr.enableConfirmation()
	tr.step(current, w, h, start.Add(300*time.Millisecond))
	tr.observePeopleFromCapture([]Rect{box}, nil, capture, scanRegion{0, 0, w, h}, start, start.Add(300*time.Millisecond))
	views := tr.views(start.Add(300 * time.Millisecond))
	if len(views) != 1 || math.Abs(views[0].Box.X-110) > 2 {
		t.Fatalf("delayed detection did not catch up to actual photo: %+v", views)
	}
	id := views[0].ID
	tr.step(later, w, h, start.Add(533*time.Millisecond))
	views = tr.views(start.Add(533 * time.Millisecond))
	if len(views) != 1 || views[0].ID != id || math.Abs(views[0].Box.X-116) > 2 {
		t.Fatalf("captured anchor vetoed valid subsequent motion: %+v", views)
	}
	if tr.people[0].LastAI != start {
		t.Fatal("catch-up falsified the AI evidence timestamp")
	}
}

func TestCapturedPhotoSearchRejectsAbsenceAndAmbiguity(t *testing.T) {
	capture, w, h := photographicTrackingFrame(t, 100)
	box := Rect{X: 100, Y: 40, W: 70, H: 180, Score: .9}
	anchor, ok := sampleCaptureAnchor(capture, scanRegion{0, 0, w, h}, box)
	if !ok {
		t.Fatal("invalid anchor")
	}
	for _, positions := range [][]int{nil, {60, 140}} {
		pixels, _, _ := photographicTrackingFrame(t, positions...)
		gray, gw, gh := grayscaleQuarter(pixels, w, h)
		if dx, dy, ok := capturedAnchorTranslation(anchor, gray, gw, gh, box, scanRegion{0, 0, w, h}); ok {
			t.Fatalf("absent or ambiguous person accepted: %v, %v/%v", positions, dx, dy)
		}
	}
}
