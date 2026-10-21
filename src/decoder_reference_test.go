package main

import (
	"math"
	"testing"
)

func decoderAnchor() []float32 {
	out := make([]float32, 84)
	out[0], out[1], out[2], out[3], out[4] = 320, 320, 100, 200, .8
	return out
}

func TestDecoderRejectsHigherScoringNonPersonClass(t *testing.T) {
	out := decoderAnchor()
	out[6] = .95 // car, with a plausible but lower person score
	if got := decodePeople(out, 1, 640, 640, 1, 0, 0, .4); len(got) != 0 {
		t.Fatalf("car accepted as person: %+v", got)
	}
	out[6] = out[4] // Ultralytics argmax selects first class on a tie.
	if got := decodePeople(out, 1, 640, 640, 1, 0, 0, .4); len(got) != 1 {
		t.Fatalf("person class tie rejected: %+v", got)
	}
}

func TestDecoderRejectsNonFiniteTensorValues(t *testing.T) {
	for _, channel := range []int{0, 1, 2, 3, 4, 5, 83} {
		for _, bad := range []float32{float32(math.NaN()), float32(math.Inf(1)), float32(math.Inf(-1))} {
			out := decoderAnchor()
			out[channel] = bad
			if got := decodePeople(out, 1, 640, 640, 1, 0, 0, .4); len(got) != 0 {
				t.Errorf("channel %d value %v accepted: %+v", channel, bad, got)
			}
		}
	}
}

func TestDecoderStrictConfidenceAndValidGeometry(t *testing.T) {
	out := decoderAnchor()
	out[4] = .4
	if got := decodePeople(out, 1, 640, 640, 1, 0, 0, .4); len(got) != 0 {
		t.Fatal("threshold equality must be excluded like Ultralytics")
	}
	out = decoderAnchor()
	for _, scale := range []float64{math.NaN(), math.Inf(1), 0, -1} {
		if got := decodePeople(out, 1, 640, 640, scale, 0, 0, .4); len(got) != 0 {
			t.Errorf("invalid scale accepted: %v", scale)
		}
	}
}

func TestDecoderSuppressesBeforeClippingToImage(t *testing.T) {
	out := make([]float32, 84*2)
	out[0], out[1] = -60, 40
	out[2], out[3] = 200, 200
	out[4], out[5] = 200, 80
	out[6], out[7] = 100, 100
	out[8], out[9] = .9, .8
	if got := decodePeople(out, 2, 640, 640, 1, 0, 0, .4); len(got) != 2 {
		t.Fatalf("clipped IoU wrongly suppressed distinct proposals: %+v", got)
	}
}

func TestDecoderDoesNotDiscardPersonBehindDuplicateCandidates(t *testing.T) {
	const n = 301
	out := make([]float32, 84*n)
	for i := 0; i < n; i++ {
		out[i] = 100
		out[n+i] = 100
		out[2*n+i] = 50
		out[3*n+i] = 100
		out[4*n+i] = .9
	}
	out[n-1], out[4*n+n-1] = 500, .8
	if got := decodePeople(out, n, 640, 640, 1, 0, 0, .4); len(got) != 2 {
		t.Fatalf("second person lost behind duplicate anchors: %+v", got)
	}
}
