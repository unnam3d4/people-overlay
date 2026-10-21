package main

import (
	"math"
	"sort"
	"time"
)

// ID means a track in this session only, never the person's identity.
type TrackView struct {
	Box Rect
	ID  int
}

type trackState uint8

const (
	trackTentative trackState = iota
	trackConfirmed
	trackLost
)

type personTrack struct {
	Box                  Rect
	ID                   int
	LastAI               time.Time
	LastVisual           time.Time
	LastMotion           time.Time
	LastStep             time.Time
	VelocityX, VelocityY float64
	Tile                 int
	AIMisses             int
	LastNegativeCapture  time.Time
	Anchor               [49]byte
	AnchorValid          bool
	AnchorCaptured       bool
	State                trackState
	FirstAI              time.Time
	CreatedAt            time.Time
	TentativeUntil       time.Time
	LastPositiveCapture  time.Time
	AIHits               int
}

type personTracker struct {
	people               []personTrack
	nextID               int
	prevGray             []byte
	grayW, grayH         int
	grayAt               time.Time
	screenW, screenH     int
	columns              int
	maxAIAge             time.Duration
	confirmationRequired bool
}

func (t *personTracker) enableConfirmation() { t.confirmationRequired = true }

func (t *personTracker) recordConfirmation(p *personTrack, score float32, capturedAt, now time.Time) {
	if p.FirstAI.IsZero() {
		p.FirstAI = capturedAt
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
		// Give a slow CPU time to return a second independent observation.
		// Only candidate retention uses delivery time; AI evidence keeps the
		// original capture timestamp and pixels cannot confirm a candidate.
		wait := 2*now.Sub(capturedAt) + time.Second
		if wait < 3*time.Second {
			wait = 3 * time.Second
		}
		p.TentativeUntil = now.Add(wait)
	}
	if p.LastPositiveCapture.IsZero() || capturedAt.After(p.LastPositiveCapture) {
		p.LastPositiveCapture = capturedAt
		p.AIHits++
	}
	if !t.confirmationRequired || score >= .65 || p.AIHits >= 2 {
		p.State = trackConfirmed
	}
}

func (t *personTracker) trackVisible(p personTrack) bool {
	return !t.confirmationRequired || p.State == trackConfirmed
}

const trackingDownscale = 2
const maxTrackedPeople = 48

func (t *personTracker) layout(w, h, cols int) {
	t.screenW, t.screenH, t.columns = w, h, cols
}
func (t *personTracker) tileOf(r Rect) int {
	if t.screenW < 1 || t.screenH < 1 || t.columns < 2 {
		return 0
	}
	x := int((r.X + r.W*.5) * float64(t.columns) / float64(t.screenW))
	y := int((r.Y + r.H*.5) * float64(t.columns) / float64(t.screenH))
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x >= t.columns {
		x = t.columns - 1
	}
	if y >= t.columns {
		y = t.columns - 1
	}
	return y*t.columns + x + 1
}
func (t *personTracker) reset() {
	t.people = nil
	t.prevGray = nil
	t.grayW, t.grayH = 0, 0
	t.grayAt = time.Time{}
}

func minTrack(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func grayscaleQuarter(pixels []byte, w, h int) ([]byte, int, int) {
	if w < 1 || h < 1 || len(pixels) < w*h*4 {
		return nil, 0, 0
	}
	dw, dh := (w+trackingDownscale-1)/trackingDownscale, (h+trackingDownscale-1)/trackingDownscale
	g := make([]byte, dw*dh)
	for y := 0; y < dh; y++ {
		sy := minTrack(h-1, y*trackingDownscale+1)
		for x := 0; x < dw; x++ {
			sx := minTrack(w-1, x*trackingDownscale+1)
			p := (sy*w + sx) * 4
			g[y*dw+x] = uint8((int(pixels[p])*11 + int(pixels[p+1])*59 + int(pixels[p+2])*30) / 100)
		}
	}
	return g, dw, dh
}
func absTrack(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// A cheap localized block matcher. 2px spatial resolution lets it follow
// 14-27px-wide silhouettes that the old 4px matcher could not handle.
// It never predicts a new person. If the template becomes different or
// untextured it rejects motion rather than allowing drift.
func motionShift(previous, current []byte, w, h int, box Rect) (float64, float64, bool) {
	if w < 1 || h < 1 || len(previous) != w*h || len(current) != w*h {
		return 0, 0, false
	}
	if box.W < 14 || box.H < 20 {
		return 0, 0, false
	}
	cx := (box.X + box.W*.5) / trackingDownscale
	cy := (box.Y + box.H*.5) / trackingDownscale
	hw := math.Max(3.0, box.W*.28/trackingDownscale)
	hh := math.Max(4.0, box.H*.30/trackingDownscale)
	if cx-hw < 7 || cy-hh < 7 || cx+hw >= float64(w-7) || cy+hh >= float64(h-7) {
		return 0, 0, false
	}
	var xs, ys [63]int
	var values [63]byte
	k, lo, hi := 0, 255, 0
	for iy := 0; iy < 9; iy++ {
		y := int(math.Round(cy - hh + float64(iy)*2*hh/8))
		for ix := 0; ix < 7; ix++ {
			x := int(math.Round(cx - hw + float64(ix)*2*hw/6))
			if x < 6 || x >= w-6 || y < 6 || y >= h-6 {
				return 0, 0, false
			}
			xs[k], ys[k] = x, y
			v := previous[y*w+x]
			values[k] = v
			if int(v) < lo {
				lo = int(v)
			}
			if int(v) > hi {
				hi = int(v)
			}
			k++
		}
	}
	if hi-lo < 18 {
		return 0, 0, false
	}
	best, bestDX, bestDY := 1e9, 0, 0
	same := 1e9
	second := 1e9
	for dy := -5; dy <= 5; dy++ {
		for dx := -5; dx <= 5; dx++ {
			diff := 0
			for i := 0; i < 63; i++ {
				a, b := int(values[i]), int(current[(ys[i]+dy)*w+xs[i]+dx])
				if a > b {
					diff += a - b
				} else {
					diff += b - a
				}
			}
			avg := float64(diff) / 63
			if dx == 0 && dy == 0 {
				same = avg
			}
			score := avg + float64(absTrack(dx)+absTrack(dy))*.10
			if score < best {
				second = best
				best, bestDX, bestDY = score, dx, dy
			} else if score < second {
				second = score
			}
		}
	}
	if best > 25 {
		return 0, 0, false
	}
	if bestDX == 0 && bestDY == 0 {
		return 0, 0, true
	}
	// A repetitive background generates near-equal matches and otherwise
	// makes the corners hop between tiles. Prefer not moving over guessing.
	if same-best < 3.5 || second-best < .24 {
		return 0, 0, false
	}
	return float64(bestDX * trackingDownscale), float64(bestDY * trackingDownscale), true
}

// A background that became motionless after the human left must not
// perpetually refresh visual evidence. Compare the static patch against
// the last trustworthy person texture, not only the immediately prior frame.
func sampleAnchor(gray []byte, w, h int, box Rect) ([49]byte, bool) {
	var out [49]byte
	if len(gray) < w*h || w <= 0 || h <= 0 {
		return out, false
	}
	cx := (box.X + box.W*.5) / trackingDownscale
	cy := (box.Y + box.H*.5) / trackingDownscale
	hw := box.W * .34 / trackingDownscale
	hh := box.H * .35 / trackingDownscale
	if cx-hw < 0 || cy-hh < 0 || cx+hw >= float64(w)-.5 || cy+hh >= float64(h)-.5 {
		return out, false
	}
	k := 0
	for y := 0; y < 7; y++ {
		for x := 0; x < 7; x++ {
			px := int(math.Round(cx - hw + 2*hw*float64(x)/6))
			py := int(math.Round(cy - hh + 2*hh*float64(y)/6))
			out[k] = gray[py*w+px]
			k++
		}
	}
	return out, true
}

// Use the full screen's downsampling lattice, even when the camera crop
// starts on an odd pixel. Downsampling the crop first changes that phase
// and can make an unchanged textured person look like a different patch.
func sampleCaptureAnchor(pixels []byte, region scanRegion, box Rect) ([49]byte, bool) {
	var out [49]byte
	if region.w <= 0 || region.h <= 0 || len(pixels) < region.w*region.h*4 {
		return out, false
	}
	cx := (box.X + box.W*.5) / trackingDownscale
	cy := (box.Y + box.H*.5) / trackingDownscale
	hw := box.W * .34 / trackingDownscale
	hh := box.H * .35 / trackingDownscale
	k := 0
	for y := 0; y < 7; y++ {
		for x := 0; x < 7; x++ {
			px := int(math.Round(cx-hw+2*hw*float64(x)/6))*trackingDownscale + 1 - region.x
			py := int(math.Round(cy-hh+2*hh*float64(y)/6))*trackingDownscale + 1 - region.y
			if px < 0 || py < 0 || px >= region.w || py >= region.h {
				return out, false
			}
			off := (py*region.w + px) * 4
			out[k] = byte((int(pixels[off])*11 + int(pixels[off+1])*59 + int(pixels[off+2])*30) / 100)
			k++
		}
	}
	return out, true
}
func anchorStillPresent(old, next [49]byte) bool {
	total, large := 0, 0
	for i := range old {
		d := absTrack(int(old[i]) - int(next[i]))
		total += d
		if d > 25 {
			large++
		}
	}
	return total <= 49*16 && large <= 13
}

// A newly delivered neural box describes the capture, not the latest screen.
// Recover that missing displacement before consecutive-frame motion starts.
// Match only the captured person texture, and reject ambiguous distant matches.
func capturedAnchorTranslation(anchor [49]byte, gray []byte, w, h int, box Rect, region scanRegion) (float64, float64, bool) {
	if patch, ok := sampleAnchor(gray, w, h, box); ok && anchorStillPresent(anchor, patch) {
		return 0, 0, true
	}
	limitX := int(math.Max(16, math.Min(96, box.W*.8)))
	limitY := int(math.Max(12, math.Min(48, box.H*.25)))
	limitX += limitX % 2
	limitY += limitY % 2
	type candidate struct {
		dx, dy int
		error  float64
	}
	var matches []candidate
	best := candidate{error: math.Inf(1)}
	for dy := -limitY; dy <= limitY; dy += 2 {
		for dx := -limitX; dx <= limitX; dx += 2 {
			shifted := box
			shifted.X += float64(dx)
			shifted.Y += float64(dy)
			cx, cy := shifted.X+shifted.W*.5, shifted.Y+shifted.H*.5
			if cx < float64(region.x) || cy < float64(region.y) || cx >= float64(region.x+region.w) || cy >= float64(region.y+region.h) {
				continue
			}
			patch, ok := sampleAnchor(gray, w, h, shifted)
			if !ok || !anchorStillPresent(anchor, patch) {
				continue
			}
			total := 0
			for i := range anchor {
				total += absTrack(int(anchor[i]) - int(patch[i]))
			}
			c := candidate{dx: dx, dy: dy, error: float64(total) / 49}
			matches = append(matches, c)
			if c.error < best.error {
				best = c
			}
		}
	}
	if math.IsInf(best.error, 1) {
		return 0, 0, false
	}
	for _, c := range matches {
		if (absTrack(c.dx-best.dx) > 4 || absTrack(c.dy-best.dy) > 4) && c.error-best.error < 2 {
			return 0, 0, false
		}
	}
	return float64(best.dx), float64(best.dy), true
}

// When the frame-to-frame block matcher says "still", check a very small
// neighbourhood against texture originally verified by AI. A 2px shift can
// otherwise be reported as zero motion even though the silhouette slowly
// walks away from its corners. Do not turn a near-equal background match into
// motion: require a meaningful gain over keeping the original location.
func anchorFineMotion(anchor [49]byte, gray []byte, w, h int, box Rect) (float64, float64, bool) {
    patch0, valid := sampleAnchor(gray,w,h,box)
    if !valid { return 0,0,false }
    anchorError:=func(next [49]byte) float64 {
        var total int
        for i:=range anchor { total+=absTrack(int(anchor[i])-int(next[i])) }
        return float64(total)/49
    }
    zeroScore:=anchorError(patch0)
    bestScore:=math.Inf(1)
    var bestDX,bestDY int
    type alternative struct {dx,dy int; score float64}
    alternatives:=make([]alternative,0,48)
    // -6..6 screen pixels in 2px steps: 49 patches x 49 pixels.
    // Bounded cost is much smaller than a second neural inference.
    for dy:=-6;dy<=6;dy+=2 {
        for dx:=-6;dx<=6;dx+=2 {
            moved:=box
            moved.X+=float64(dx)
            moved.Y+=float64(dy)
            next,ok:=sampleAnchor(gray,w,h,moved)
            if !ok||!anchorStillPresent(anchor,next){continue}
            score:=anchorError(next)
            alternatives=append(alternatives,alternative{dx,dy,score})
            if score<bestScore {
                bestScore,bestDX,bestDY=score,dx,dy
            }
        }
    }
    if math.IsInf(bestScore,1) {return 0,0,false}
    if bestDX==0&&bestDY==0 {return 0,0,true}
    // Avoid fake movement on flat, repeating or merely noisy backgrounds.
    if !anchorStillPresent(anchor,patch0) {
        // If the old position is gone, a candidate must be both unambiguous
        // and close to the AI-confirmed target.
        if bestScore>12{return 0,0,false}
    } else if zeroScore-bestScore<3.0 {
        return 0,0,true
    }
    for _,c:=range alternatives {
        if absTrack(c.dx-bestDX)>4||absTrack(c.dy-bestDY)>4 {
            if c.score-bestScore<2.0 {return 0,0,false}
        }
    }
    return float64(bestDX),float64(bestDY),true
}

// When clothing or the image is smooth, template matching has too little
// texture to infer motion. A stable patch still gives positive visual evidence
// of a stationary person (and will not be mistaken for a vanished silhouette).
func stationaryAppearance(previous, current []byte, w, h int, box Rect) bool {
	if w < 1 || h < 1 || len(previous) != w*h || len(current) != w*h {
		return false
	}
	cx := (box.X + box.W*.5) / trackingDownscale
	cy := (box.Y + box.H*.5) / trackingDownscale
	hw := box.W * .34 / trackingDownscale
	hh := box.H * .35 / trackingDownscale
	if cx-hw < 0 || cy-hh < 0 || cx+hw >= float64(w)-.5 || cy+hh >= float64(h)-.5 {
		return false
	}
	total, large := 0, 0
	for y := 0; y < 7; y++ {
		for x := 0; x < 7; x++ {
			px := int(math.Round(cx - hw + 2*hw*float64(x)/6))
			py := int(math.Round(cy - hh + 2*hh*float64(y)/6))
			diff := absTrack(int(previous[py*w+px]) - int(current[py*w+px]))
			total += diff
			if diff > 24 {
				large++
			}
		}
	}
	return total <= 49*9 && large <= 5
}

func (t *personTracker) step(pixels []byte, w, h int, now time.Time) {
	gray, gw, gh := grayscaleQuarter(pixels, w, h)
	if gray == nil {
		return
	}
	t.grayAt = now
	if t.grayW != gw || t.grayH != gh || len(t.prevGray) != len(gray) {
		t.prevGray = gray
		t.grayW, t.grayH = gw, gh
		return
	}
	for i := range t.people {
		p := &t.people[i]
		if !p.AnchorValid && !p.AnchorCaptured {
			p.Anchor, p.AnchorValid = sampleAnchor(t.prevGray, gw, gh, p.Box)
		}
		dx, dy, good := motionShift(t.prevGray, gray, gw, gh, p.Box)
        // If the center-template matcher freezes or loses a filmed person,
        // use consensus sparse *person-region* patch flow. This estimates
        // motion from several independent textured parts of the body,
        // never from one ambiguous background patch. Unlike continuous
        // Lucas-Kanade, this is bounded local SAD matching in pure Go.
        flowVerified:=false
        if p.AnchorValid && (!good || (dx==0 && dy==0)) {
            if fx,fy,ok:=sparsePersonFlow(t.prevGray,gray,gw,gh,p.Box);ok &&
              (fx!=0||fy!=0) {
                dx,dy,good=fx,fy,true
                flowVerified=true
            }
        }
        // An apparently stationary block is not necessarily a stationary
        // person: resolve sub-block movement using AI-verified texture.
        if p.AnchorValid && good && dx==0 && dy==0 {
            if fx,fy,ok:=anchorFineMotion(p.Anchor,gray,gw,gh,p.Box);ok && (fx!=0||fy!=0) {
                dx,dy=fx,fy
            }
        }
        // Sparse 18-24px CCTV silhouettes can be too narrow for the 63-point
        // consecutive-frame matcher, especially after an odd-pixel move or
        // repeated video frame. On failure, reacquire only the *originally
        // AI-confirmed* texture within a bounded local displacement. The
        // matcher explicitly rejects ambiguous distant patches: background
        // motion alone must never renew LastVisual.
        if !good && p.AnchorValid {
            // Retry the compact local search first. This handles slow
            // 2-6px movement when the block matcher rejects nearly equal
            // correlations, without treating motion in the background as a
            // new anchor. The wider search is only a last resort.
            if x,y,ok:=anchorFineMotion(p.Anchor,gray,gw,gh,p.Box);ok {
                dx,dy,good=x,y,true
            } else if x,y,ok:=capturedAnchorTranslation(p.Anchor,gray,gw,gh,p.Box,scanRegion{0,0,w,h});ok {
                dx,dy,good=x,y,true
            }
        }
		// A stable background often returns a perfect zero-motion match. It is
		// not evidence of the *person* unless the AI-anchored texture is present.
		if good && (p.AnchorValid || p.AnchorCaptured) {
			candidate := p.Box
			candidate.X += dx
			candidate.Y += dy
			nowPatch, ok := sampleAnchor(gray, gw, gh, candidate)
            anchored:=p.AnchorValid && ok && anchorStillPresent(p.Anchor,nowPatch)
            // Sparse flow from multiple body points can temporarily bridge
            // clothing/lighting changes during a slow neural revisit. The
            // exception is strictly age-bounded; it cannot make a moving
            // wall count as a person for an entire 16-camera sweep.
            recentAI:=!p.LastAI.IsZero() && !now.Before(p.LastAI) &&
                now.Sub(p.LastAI)<=1800*time.Millisecond
			if !anchored && !(flowVerified && recentAI) {
				good = false
			}
		}
		dt := now.Sub(p.LastStep).Seconds()
		if dt <= 0 || dt > .4 {
			dt = .115
		}
		p.LastStep = now
		if !good {
			// Low-texture CCTV clothing can remain perfectly still. Keep the
			// confirmed person's corners while their actual pixels remain stable.
			if stationaryAppearance(t.prevGray, gray, gw, gh, p.Box) {
				current, valid := sampleAnchor(gray, gw, gh, p.Box)
				if (!p.AnchorValid && !p.AnchorCaptured) || (p.AnchorValid && valid && anchorStillPresent(p.Anchor, current)) {
					p.LastVisual = now
					p.VelocityX = 0
					p.VelocityY = 0
					continue
				}
				// The original silhouette is absent although the *background*
				// has become motionless. Keep LastVisual stale for prompt removal.
			}
			// Bridge at most 0.35s of weak or occluded video using recent velocity.
			// Never renew LastVisual here: lost tracks still expire promptly.
			if !p.LastMotion.IsZero() && now.Sub(p.LastMotion) < 350*time.Millisecond &&
				now.Sub(p.LastVisual) < 350*time.Millisecond {
				dx = math.Max(-8, math.Min(8, p.VelocityX*dt*.5))
				dy = math.Max(-8, math.Min(8, p.VelocityY*dt*.5))
				p.VelocityX *= .78
				p.VelocityY *= .78
			} else {
				p.VelocityX = 0
				p.VelocityY = 0
				continue
			}
		} else {
			p.LastVisual = now
			if dx != 0 || dy != 0 {
				velX := math.Max(-160, math.Min(160, dx/dt))
				velY := math.Max(-160, math.Min(160, dy/dt))
				p.VelocityX = .46*p.VelocityX + .54*velX
				p.VelocityY = .46*p.VelocityY + .54*velY
				p.LastMotion = now
			} else {
				p.VelocityX *= .5
				p.VelocityY *= .5
			}
		}
		next := p.Box
		next.X = math.Max(0, math.Min(float64(w)-next.W, next.X+dx))
		next.Y = math.Max(0, math.Min(float64(h)-next.H, next.Y+dy))
		// Only allow movement that matches actual pixels. An approximate
		// 4x4 screen grid is not an authoritative camera boundary: RVi toolbar
		// and window borders can shift the real video tiles.
		if dx != 0 || dy != 0 {
			p.Box = next
			p.Tile = t.tileOf(next)
		}
		if good && p.AnchorValid && dx != 0 || good && p.AnchorValid && dy != 0 {
			if sampled, ok := sampleAnchor(gray, gw, gh, p.Box); ok && anchorStillPresent(p.Anchor, sampled) {
				// Update clothing template only after a verified movement, never
				// on newly static *background* where the person used to stand.
				p.Anchor = sampled
			}
		}
	}
	t.prevGray = gray
	// Never merge IDs merely because two real people overlap on screen.
	t.prune(now)
}

// The same person can have boxes with different sizes in overlapping crops.
// The metric combines ordinary IoU and small-box containment.
func coveringFraction(a, b Rect) float64 {
	x1, y1 := math.Max(a.X, b.X), math.Max(a.Y, b.Y)
	x2, y2 := math.Min(a.X+a.W, b.X+b.W), math.Min(a.Y+a.H, b.Y+b.H)
	if x2 <= x1 || y2 <= y1 {
		return 0
	}
	return (x2 - x1) * (y2 - y1) / math.Max(1, math.Min(a.W*a.H, b.W*b.H))
}
func centerSeparation(a, b Rect) float64 {
	dx := (a.X + a.W*.5 - b.X - b.W*.5) / math.Max(14, math.Max(a.W, b.W))
	dy := (a.Y + a.H*.5 - b.Y - b.H*.5) / math.Max(14, math.Max(a.H, b.H))
	return math.Hypot(dx, dy)
}
func samePersonGeometry(a, b Rect) bool {
	if a.W <= 0 || b.W <= 0 || a.H <= 0 || b.H <= 0 {
		return false
	}
	center := centerSeparation(a, b)
	iou := overlap(a, b)
	if iou >= .52 {
		return true
	}
	if center <= .25 && coveringFraction(a, b) >= .74 {
		return true
	}
	return false
}

func (t *personTracker) observePeople(detections []Rect, at time.Time) {
	if len(detections) == 0 {
		return
	}
	t.assignDetections(uniquePersonDetections(detections), at)
	t.prune(at)
}

func uniquePersonDetections(detections []Rect) []Rect {
	detections = append([]Rect(nil), detections...)
	sort.SliceStable(detections, func(i, j int) bool { return detections[i].Score > detections[j].Score })
	// A single YOLO frame can contain duplicate near-identical proposals.
	// Keep two genuinely overlapping people unless almost the same silhouette.
	unique := make([]Rect, 0, len(detections))
	for _, d := range detections {
		if d.W < 6 || d.H < 9 {
			continue
		}
		duplicate := false
		for i, u := range unique {
			if !sameROISubject(d, u) {
				continue
			}
			// Prefer the full human over torso/head proposals. Different crops
			// can make the partial detection appear more confident than the body.
			if nestedROISubject(d, u) && d.W*d.H > u.W*u.H && d.Score >= u.Score*.58 {
				unique[i] = d
			} else if sameSilhouette(d, u) && d.Score > u.Score {
				unique[i] = d
			}
			duplicate = true
			break
		}
		if !duplicate {
			unique = append(unique, d)
		}
	}
	return unique
}

// Preserve the time and pixels that produced an asynchronous AI result.
// Inference completion is not fresh evidence that the person is still there.
func (t *personTracker) observePeopleFromCapture(found []Rect, captured []TrackView, pixels []byte, region scanRegion, capturedAt, now time.Time) {
	if capturedAt.IsZero() || capturedAt.After(now) {
		return
	}
	unique := uniquePersonDetections(found)
	current := t.schedulerViews(now)
	previous := make(map[int]personTrack, len(t.people))
	for _, p := range t.people {
		previous[p.ID] = p
	}
	corrected := compensateLateDetections(unique, captured, current)
	ids := t.assignDetectionsAt(corrected, now, capturedAt)
	for j, id := range ids {
		if id == 0 {
			continue
		}
		for i := range t.people {
			p := &t.people[i]
			if p.ID != id {
				continue
			}
			old, existed := previous[id]
			p.LastAI = capturedAt
			p.LastVisual = capturedAt
			if existed && old.LastVisual.After(p.LastVisual) {
				p.LastVisual = old.LastVisual
			}
			// Sample the filtered box in the actual AI image, undoing its measured
			// displacement since capture. Never anchor a delayed result to today's
			// possibly empty background.
			box := p.Box
			box.X -= corrected[j].X - unique[j].X
			box.Y -= corrected[j].Y - unique[j].Y
			p.Anchor, p.AnchorValid = sampleCaptureAnchor(pixels, region, box)
			p.AnchorCaptured = true
			if !p.AnchorValid && existed && old.AnchorValid {
				p.Anchor, p.AnchorValid = old.Anchor, true
			}
			// A slow CPU may take seconds to discover a stationary person. Keep
			// the true capture time, but allow the fresh screen to verify that
			// person's captured appearance instead of immediately expiring it.
			if p.AnchorValid && !t.grayAt.Before(capturedAt) && !t.grayAt.After(now) && now.Sub(t.grayAt) <= 250*time.Millisecond {
				dx, dy, ok := capturedAnchorTranslation(p.Anchor, t.prevGray, t.grayW, t.grayH, p.Box, region)
				if ok {
					p.Box.X += dx
					p.Box.Y += dy
					p.Tile = t.tileOf(p.Box)
					p.LastVisual = t.grayAt
				}
			}
			break
		}
	}
	t.prune(now)
}

// Each camera is checked asynchronously. One missed YOLO detection can
// occur because of occlusion or poor video quality; two independent negative
// scans of the *same fully visible region* are enough to retire a ghost box.
// Crops touching the edge of the person's box are intentionally ignored.
func (t *personTracker) noteRegionScan(region scanRegion, now time.Time) {
	t.noteRegionScanCaptured(region, now, now, 0)
}

// An in-flight inference describes the screen *at capture time*.
// Do not count a miss against a person who appeared afterwards, or
// against a person belonging to a different CCTV camera tile.
func (t *personTracker) noteRegionScanCaptured(region scanRegion, now, capturedAt time.Time, tileID int) {
	captured := make([]TrackView, 0, len(t.people))
	for _, p := range t.people {
		captured = append(captured, TrackView{Box: p.Box, ID: p.ID})
	}
	t.noteRegionScanSnapshot(region, now, capturedAt, tileID, captured)
}

func (t *personTracker) noteRegionScanSnapshot(region scanRegion, now, capturedAt time.Time, tileID int, captured []TrackView) {
	left, top := float64(region.x), float64(region.y)
	right, bottom := float64(region.x+region.w), float64(region.y+region.h)
	boxes := make(map[int]Rect, len(captured))
	for _, v := range captured {
		boxes[v.ID] = v.Box
	}
	for i := range t.people {
		p := &t.people[i]
		box, ok := boxes[p.ID]
		if !ok || !p.LastAI.Before(now) || box.W <= 0 || box.H <= 0 {
			continue
		}
		if !p.LastAI.Before(capturedAt) {
			continue
		}
		if !p.LastNegativeCapture.IsZero() && !capturedAt.After(p.LastNegativeCapture) {
			continue
		}
		tile := t.tileOf(box)
		if tileID > 0 && tile > 0 && tile != tileID {
			continue
		}
		w := math.Min(box.X+box.W, right) - math.Max(box.X, left)
		h := math.Min(box.Y+box.H, bottom) - math.Max(box.Y, top)
		if w <= 0 || h <= 0 || w*h < .90*box.W*box.H {
			continue
		}
		p.AIMisses++
		p.LastNegativeCapture = capturedAt
	}
	t.prune(now)
}

func (t *personTracker) prune(now time.Time) {
	keep := t.people[:0]
	for _, p := range t.people {
		aiAge := now.Sub(p.LastAI)
		visualAge := now.Sub(p.LastVisual)
		if aiAge < 0 || visualAge < 0 {
			continue
		}
		if p.AIMisses >= 2 {
			continue
		}
        // 16-camera mosaics share a single expensive CPU neural worker.
        // Measured revisit gaps for an already found human are 1.2-2.0s;
        // the universal 1.2/1.7s screen timeout killed valid people first,
        // then the next NN observation recreated them with fresh IDs.
        // Keep a narrowly bounded 2.6s maximum while AI is in flight;
        // independent negative scans still remove a ghost immediately.
        // Fullscreen/2x2/3x3 retain existing faster removal.
        visibleGrace:=1200*time.Millisecond
        aiGrace:=1700*time.Millisecond
        lostGrace:=1200*time.Millisecond
        if t.columns==4 {
            visibleGrace=2400*time.Millisecond
            aiGrace=2600*time.Millisecond
            lostGrace=2400*time.Millisecond
        }
		if t.confirmationRequired {
			if p.State == trackTentative {
				if now.After(p.TentativeUntil) {
					continue
				}
				// Tentative tracks are hidden. Their bounded confirmation
				// budget must survive the first frame's age while the next
				// slow inference is in flight.
				keep = append(keep, p)
				continue
			} else if aiAge > lostGrace && visualAge > 700*time.Millisecond {
				p.State = trackLost
			} else {
				p.State = trackConfirmed
			}
		}
		// Unverified boxes must disappear quickly rather than freeze for 9s.
		if visualAge > visibleGrace && aiAge > aiGrace {
			continue
		}
		// Even if the texture matcher claims a static match, force neural
		// revalidation eventually so background texture is not called a person.
		ttl := t.maxAIAge
		if ttl <= 0 {
			ttl = 8 * time.Second
		}
		// No static screen texture can extend a missing AI confirmation forever.
		// Active targets are deliberately revisited every few seconds by scheduler.
		if aiAge > ttl {
			continue
		}
		keep = append(keep, p)
	}
	t.people = keep
}
func (t *personTracker) views(now time.Time) []TrackView {
	t.prune(now)
	views := make([]TrackView, 0, len(t.people))
	for _, p := range t.people {
		if t.trackVisible(p) {
			views = append(views, TrackView{Box: p.Box, ID: p.ID})
		}
	}
	return views
}

// Hidden candidates and briefly lost identities still need neural revisits.
func (t *personTracker) schedulerViews(now time.Time) []TrackView {
	t.prune(now)
	views := make([]TrackView, 0, len(t.people))
	for _, p := range t.people {
		views = append(views, TrackView{Box: p.Box, ID: p.ID})
	}
	return views
}

func (t *personTracker) tentativeViews(now time.Time) []TrackView {
	t.prune(now)
	views := make([]TrackView, 0)
	if !t.confirmationRequired {
		return views
	}
	for _, p := range t.people {
		// Hidden weak candidates require their second independent AI
        // observation. Briefly lost confirmed people deserve the same
        // urgent crop before expiring and coming back as duplicate IDs.
		if p.State == trackTentative || p.State == trackLost {
			views = append(views, TrackView{Box: p.Box, ID: p.ID})
		}
	}
	return views
}

func (t *personTracker) statusCounts(now time.Time) (tentative, confirmed, lost int) {
	t.prune(now)
	for _, p := range t.people {
		if !t.confirmationRequired {
			confirmed++
			continue
		}
		switch p.State {
		case trackTentative:
			tentative++
		case trackConfirmed:
			confirmed++
		case trackLost:
			lost++
		}
	}
	return
}
func sameTrackViews(a, b []TrackView) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID {
			return false
		}
		u, v := a[i].Box, b[i].Box
		if math.Abs(u.X-v.X) > 1.2 || math.Abs(u.Y-v.Y) > 1.2 || math.Abs(u.W-v.W) > 1.2 || math.Abs(u.H-v.H) > 1.2 {
			return false
		}
	}
	return true
}
