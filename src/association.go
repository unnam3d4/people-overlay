package main

import (
	"math"
	"time"
)

// A detector result is a box, not a person's identity. Pair the entire
// collection of existing tracks and current boxes in one global decision:
// processing high-confidence boxes one at a time makes IDs switch in crowds.
func predictedTrackBox(p personTrack, now time.Time) Rect {
	b := p.Box
	if p.LastMotion.IsZero() || p.LastStep.IsZero() || now.Before(p.LastStep) ||
		now.Sub(p.LastMotion) > 600*time.Millisecond {
		return b
	}
	dt := now.Sub(p.LastStep).Seconds()
	if dt < 0 {
		dt = 0
	}
	if dt > .5 {
		dt = .5
	}
	// Never teleport a track to a different feed based on stale velocity.
	limitX := math.Max(8, b.W*.48)
	limitY := math.Max(8, b.H*.28)
	b.X += math.Max(-limitX, math.Min(limitX, p.VelocityX*dt))
	b.Y += math.Max(-limitY, math.Min(limitY, p.VelocityY*dt))
	return b
}

// Strict geometric duplication, distinct from overlapping real people.
// Different sized crops of one person should normally be associated by ID;
// merging two simultaneously detected overlapping humans is not acceptable.
func sameSilhouette(a, b Rect) bool {
	if a.W <= 0 || b.W <= 0 || a.H <= 0 || b.H <= 0 {
		return false
	}
	sizeW := math.Min(a.W, b.W) / math.Max(a.W, b.W)
	sizeH := math.Min(a.H, b.H) / math.Max(a.H, b.H)
	if sizeW < .73 || sizeH < .73 {
		return false
	}
	if overlap(a, b) < .78 {
		return false
	}
	return centerSeparation(a, b) < .15
}

func trackMatchScore(p personTrack, d Rect, cell int, at time.Time) float64 {
	if d.W <= 0 || d.H <= 0 || p.Box.W <= 0 || p.Box.H <= 0 {
		return 0
	}
	// Camera mosaics are independent video streams. Do not transfer a track
	// to another cell, even if boxes happen to overlap across tile borders.
	if p.Tile > 0 && cell > 0 && p.Tile != cell && overlap(d, p.Box) < .30 {
		return 0
	}
	if nestedROISubject(d, p.Box) {
		return 88
	}
	wRatio := math.Min(d.W, p.Box.W) / math.Max(d.W, p.Box.W)
	hRatio := math.Min(d.H, p.Box.H) / math.Max(d.H, p.Box.H)
	if wRatio*hRatio < .29 {
		return 0
	}
	predicted := predictedTrackBox(p, at)
	iouNow, iouPred := overlap(d, p.Box), overlap(d, predicted)
	dist := centerSeparation(d, predicted)
    // On slow CPU one crop from another camera may execute before we
    // re-detect this pedestrian. A person can move farther than 0.79 body
    // widths between real neural detections: the old fixed gate discarded
    // correct boxes and created a new ID at every such revisit.
    gap := 0.0
    if !p.LastAI.IsZero() && at.After(p.LastAI) {
        gap = math.Min(2.4, at.Sub(p.LastAI).Seconds())
    }
    // Keep the original tight gate for fresh evidence or an unreliable
    // shape. Longer matches require the SAME video tile and similar person
    // size. Unlike unrestricted nearest-neighbour matching this cannot
    // transfer IDs between non-overlapping cameras.
    maxDistance := .79
    stale := gap > .55 && wRatio*hRatio >= .50
    if stale {
        maxDistance += math.Min(1.25, (gap-.55)*.80)
    }
    if dist > maxDistance { return 0 }
    if iouNow < .08 && iouPred < .08 && dist > .42 && !stale {
        return 0
    }
	// With tiled cameras do not transfer a track between separate video feeds
	// just because adjacent images happen to resemble one another.
	if p.Tile != 0 && cell != 0 && p.Tile != cell && math.Max(iouNow, iouPred) < .46 {
		return 0
	}
	// Explicitly favour trajectory continuity, then current overlap and shape.
	// This makes crossed detection order irrelevant to identity assignment.
    // An older but geometrically consistent track can still win a global
    // assignment when the person walks 1+ body widths between detector calls.
    // Never reward negative distance beyond one width, as the old formula did.
    proximity := 1-math.Min(1,dist/maxDistance)
	score := 70*iouPred + 25*iouNow + 18*proximity +
		8*wRatio*hRatio + 3*float64(d.Score)
    if stale && iouNow < .08 && iouPred < .08 {
        // A remote geometric recovery is weaker than recent visual overlap.
        // Preserve ties in favour of nearer / more recent identities.
        score -= 2 + math.Min(3,gap)
    }
	if p.Tile == cell {
		score += 2
	}
	return score
}

// A single human can be detected as a full body, torso and head when
// neighboring camera crops overlap. Containment alone is insufficient:
// unrelated people in crowds can overlap. Require strong horizontal
// alignment and the same top edge of the silhouette.
func nestedROISubject(a, b Rect) bool {
	if a.W <= 0 || b.W <= 0 || a.H <= 0 || b.H <= 0 {
		return false
	}
	big, small := a, b
	if a.W*a.H < b.W*b.H {
		big, small = b, a
	}
	ratio := small.W * small.H / (big.W * big.H)
	if ratio > .76 || ratio < .055 {
		return false
	}
	if coveringFraction(big, small) < .85 {
		return false
	}
	dx := math.Abs((small.X + small.W*.5) - (big.X + big.W*.5))
	if dx > big.W*.23 {
		return false
	}
	if small.Y-big.Y > big.H*.48 || small.Y < big.Y-big.H*.06 {
		return false
	}
	if small.Y+small.H*.5 > big.Y+big.H*.83 {
		return false
	}
	// The crop should look like a partial detection of the same upright
	// silhouette, not another person standing elsewhere inside a loose box.
	if small.H > big.H*.94 {
		return false
	}
	return true
}
func sameROISubject(a, b Rect) bool {
	return sameSilhouette(a, b) || nestedROISubject(a, b)
}

// O(n^3) minimum-cost assignment (rectangular Hungarian algorithm).
// Extra dummy columns let unmatched tracks remain unmatched, instead of
// stealing another person's box. At most 48 tracks are retained.
func bestGlobalMatches(scores [][]float64) []int {
	n := len(scores)
	if n == 0 {
		return nil
	}
	d := len(scores[0])
	result := make([]int, n)
	for i := range result {
		result[i] = -1
	}
	if d == 0 {
		return result
	}
	m := d + n
	const noMatch = 0.0
	cost := func(i, j int) float64 {
		if j >= d {
			return noMatch
		}
		if scores[i][j] <= 0 {
			return 1e6
		}
		return noMatch - scores[i][j]
	}
	u, v := make([]float64, n+1), make([]float64, m+1)
	p, way := make([]int, m+1), make([]int, m+1)
	for i := 1; i <= n; i++ {
		p[0] = i
		j0 := 0
		minv := make([]float64, m+1)
		used := make([]bool, m+1)
		for j := 1; j <= m; j++ {
			minv[j] = math.Inf(1)
		}
		for {
			used[j0] = true
			i0 := p[j0]
			delta := math.Inf(1)
			j1 := 0
			for j := 1; j <= m; j++ {
				if used[j] {
					continue
				}
				cur := cost(i0-1, j-1) - u[i0] - v[j]
				if cur < minv[j] {
					minv[j] = cur
					way[j] = j0
				}
				if minv[j] < delta {
					delta = minv[j]
					j1 = j
				}
			}
			for j := 0; j <= m; j++ {
				if used[j] {
					u[p[j]] += delta
					v[j] -= delta
				} else {
					minv[j] -= delta
				}
			}
			j0 = j1
			if p[j0] == 0 {
				break
			}
		}
		for {
			j1 := way[j0]
			p[j0] = p[j1]
			j0 = j1
			if j0 == 0 {
				break
			}
		}
	}
	for j := 1; j <= m; j++ {
		if p[j] > 0 && j <= d && scores[p[j]-1][j-1] > 0 {
			result[p[j]-1] = j - 1
		}
	}
	return result
}

func (t *personTracker) assignDetections(found []Rect, at time.Time) []int {
	return t.assignDetectionsAt(found, at, at)
}

func (t *personTracker) assignDetectionsAt(found []Rect, at, capturedAt time.Time) []int {
	ids := make([]int, len(found))
	if len(found) == 0 {
		return ids
	}
	scores := make([][]float64, len(t.people))
	for i, p := range t.people {
		scores[i] = make([]float64, len(found))
		for j, d := range found {
			scores[i][j] = trackMatchScore(p, d, t.tileOf(d), at)
		}
	}
	matches := bestGlobalMatches(scores)
	used := make([]bool, len(found))
	for i, j := range matches {
		if j < 0 {
			continue
		}
		used[j] = true
		p := &t.people[i]
		ids[j] = p.ID
		d := found[j]
		// A lightly filtered AI correction removes box vibration at rest without
		// making a rapidly moving person drag an entire body-width behind.
		move := math.Hypot(d.X-p.Box.X, d.Y-p.Box.Y)
		alpha := .42
		if move > math.Max(5, p.Box.W*.14) {
			alpha = .76
		}
        // The old .76 easing leaves an already 0.5–1.5s stale human
        // another 6–20 pixels behind each correct new neural box. On real
        // PETS footage the person traverses whole body widths between
        // NN callbacks. Trust a high-confidence geometrically matched
        // re-observation instead of smoothing back toward the old wall.
        if move > math.Max(12,p.Box.W*.35) && d.Score>=.65 {
            alpha=1.0
        }
		if move < 1.25 {
			alpha = .24
		}
		if nestedROISubject(p.Box, d) {
			// Hold the larger YOLO box: a torso-only crop must not shrink it.
			if d.W*d.H > p.Box.W*p.Box.H {
				p.Box = d
			}
		} else {
			p.Box.X += (d.X - p.Box.X) * alpha
			p.Box.Y += (d.Y - p.Box.Y) * alpha
			p.Box.W += (d.W - p.Box.W) * .44
			p.Box.H += (d.H - p.Box.H) * .44
		}
		p.Box.Score = d.Score
		p.LastAI = at
		t.recordConfirmation(p, d.Score, capturedAt, at)
		p.AnchorValid = false // new AI box requires new texture reference
		p.LastVisual = at
		p.AIMisses = 0
		p.Tile = t.tileOf(p.Box)
	}
	for j, d := range found {
		if used[j] || len(t.people) >= maxTrackedPeople {
			continue
		}
		duplicate := false
		for _, p := range t.people {
			if p.Tile > 0 && t.tileOf(d) > 0 && p.Tile != t.tileOf(d) && overlap(d, p.Box) < .30 {
				continue
			}
			if sameROISubject(d, p.Box) {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		t.nextID++
		ids[j] = t.nextID
		t.people = append(t.people, personTrack{
			Box: d, ID: t.nextID, LastAI: at, LastVisual: at, Tile: t.tileOf(d),
		})
		t.recordConfirmation(&t.people[len(t.people)-1], d.Score, capturedAt, at)
	}
	// Duplicate proposals are suppressed before assignment and before creating
	// a new ID. Do not merge already established IDs during an occlusion:
	// coincident boxes cannot prove that two known people became one person.
	return ids
}
