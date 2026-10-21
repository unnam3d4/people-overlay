package main

import "math"

// Detection ran on an earlier frame. Shift its box by displacement measured
// by the lightweight optical tracker since that frame. Prevents AI update
// from jumping an otherwise smooth overlay backwards in time.
func compensateLateDetections(found []Rect, atCapture, current []TrackView) []Rect {
	if len(found) == 0 || len(atCapture) == 0 || len(current) == 0 {
		return found
	}
	nowByID := make(map[int]Rect, len(current))
	for _, v := range current {
		nowByID[v.ID] = v.Box
	}
	eligible := make([]TrackView, 0, len(atCapture))
	for _, past := range atCapture {
		if _, ok := nowByID[past.ID]; ok {
			eligible = append(eligible, past)
		}
	}
	out := append([]Rect(nil), found...)
	scores := make([][]float64, len(found))
	for i, d := range out {
		scores[i] = make([]float64, len(eligible))
		for j, past := range eligible {
			iou := overlap(d, past.Box)
			dx := ((d.X + d.W*.5) - (past.Box.X + past.Box.W*.5)) / math.Max(16, d.W)
			dy := ((d.Y + d.H*.5) - (past.Box.Y + past.Box.H*.5)) / math.Max(16, d.H)
			dist := math.Hypot(dx, dy)
			if iou < .17 && dist > .45 {
				continue
			}
			scores[i][j] = iou*2 + 1/(1+dist)
		}
	}
	// Assign the whole capture at once. Otherwise two neighboring detections
	// can both borrow one person's motion and move in the same direction.
	for i, j := range bestGlobalMatches(scores) {
		if j < 0 {
			continue
		}
		past := eligible[j]
		currentBox := nowByID[past.ID]
		// This is displacement already measured for the same surviving track,
		// not velocity extrapolation. A body-width clamp would jump backwards
		// whenever a slow inference spans more than half a body-width of motion.
		out[i].X += currentBox.X - past.Box.X
		out[i].Y += currentBox.Y - past.Box.Y
	}
	return out
}
