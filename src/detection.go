package main

import (
	"math"
	"sort"
)

type Rect struct {
	X, Y, W, H float64
	Score      float32
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
func overlap(a, b Rect) float64 {
	x1, y1 := maxF(a.X, b.X), maxF(a.Y, b.Y)
	x2, y2 := minF(a.X+a.W, b.X+b.W), minF(a.Y+a.H, b.Y+b.H)
	w, h := x2-x1, y2-y1
	if w <= 0 || h <= 0 {
		return 0
	}
	i := w * h
	return i / (a.W*a.H + b.W*b.H - i)
}
func finiteDetection(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// YOLOv8 / YOLO26 one-to-many output [1,84,N], xywh and COCO scores.
// Match Ultralytics' best-class-only person filter and suppress in model
// coordinates before unletterboxing/clipping. The 6x9 display minimum and
// 80-person limit are intentional application policies.
func decodePeople(out []float32, n, fullW, fullH int, scale, padX, padY float64, threshold float32) []Rect {
	if n <= 0 || n > 8400 || len(out) < 84*n || fullW <= 0 || fullH <= 0 || scale <= 0 ||
		!finiteDetection(scale) || !finiteDetection(padX) || !finiteDetection(padY) || !finiteDetection(float64(threshold)) {
		return nil
	}
	cand := make([]Rect, 0, 100)
	for i := 0; i < n; i++ {
		sc := out[4*n+i]
		if !finiteDetection(float64(sc)) || sc <= threshold || sc > 1 {
			continue
		}
		person := true
		for c := 5; c < 84; c++ {
			other := out[c*n+i]
			if !finiteDetection(float64(other)) || other > sc {
				person = false
				break
			}
		}
		if !person {
			continue
		}
		cx, cy := float64(out[i]), float64(out[n+i])
		w, h := float64(out[2*n+i]), float64(out[3*n+i])
		if !finiteDetection(cx) || !finiteDetection(cy) || !finiteDetection(w) || !finiteDetection(h) || w <= 0 || h <= 0 {
			continue
		}
		cand = append(cand, Rect{X: cx - w/2, Y: cy - h/2, W: w, H: h, Score: sc})
	}
	kept := suppressRects(cand, .45, 80)
	result := make([]Rect, 0, len(kept))
	for _, r := range kept {
		x1 := maxF(0, minF((r.X-padX)/scale, float64(fullW)))
		y1 := maxF(0, minF((r.Y-padY)/scale, float64(fullH)))
		x2 := maxF(0, minF((r.X+r.W-padX)/scale, float64(fullW)))
		y2 := maxF(0, minF((r.Y+r.H-padY)/scale, float64(fullH)))
		if x2-x1 < 6 || y2-y1 < 9 {
			continue
		}
		result = append(result, Rect{X: x1, Y: y1, W: x2 - x1, H: y2 - y1, Score: r.Score})
	}
	return result
}
func suppressRects(in []Rect, iou float64, limit int) []Rect {
	sort.Slice(in, func(i, j int) bool { return in[i].Score > in[j].Score })
	if len(in) > 30000 {
		in = in[:30000]
	}
	out := make([]Rect, 0, 64)
	for _, r := range in {
		ok := true
		for _, p := range out {
			if overlap(r, p) > iou {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, r)
		}
		if len(out) >= limit {
			break
		}
	}
	return out
}
