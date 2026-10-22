package main

import (
	"math"
	"time"
)

// scanScheduler chooses which CCTV tile deserves the next expensive AI
// inference. The low-cost motion pass runs on snapshots of the screen.
type scanScheduler struct {
	lastScan        []time.Time
	fingerprint     [][]uint8
	activity        []float64
	recentlyFound   []time.Time
    // A one-shot arrival signal, never a perpetual motion priority.
    motionArrival []bool
    // Non-overlapping core signatures locate the owner of new motion.
    // Detection crops may overlap across neighbouring RVi camera tiles.
    coreFingerprint [][]uint8
    coreActivity []float64
	previousProfile int32
	width, height   int
	lastFull        time.Time
	firstDecision   time.Time
    decisionCount uint64
    coverageDecision uint64 // independent quota for never-scanned CCTV tiles
}

func (s *scanScheduler) reset(regions int, w, h int, profile int32) {
	s.lastScan = make([]time.Time, regions)
	s.fingerprint = make([][]uint8, regions)
	s.activity = make([]float64, regions)
	s.recentlyFound = make([]time.Time, regions)
    s.motionArrival = make([]bool, regions)
    s.coreFingerprint = make([][]uint8, regions)
    s.coreActivity = make([]float64, regions)
	s.previousProfile = profile
	s.width = w
	s.height = h
	// On CCTV mosaic a full-screen AI pass makes people tiny and wastes
	// several seconds on older processors. Start directly with camera crops.
	s.lastFull = time.Time{}
	s.firstDecision = time.Time{}
    s.decisionCount = 0
    s.coverageDecision = 0
	if profile == 3 {
		s.lastFull = time.Now()
	}
}

// A 32x24 signature covers small CCTV figures that disappear entirely in
// 16x12 sampling, while still requiring very little CPU.
func tinySignature(src []byte, w, h int, r scanRegion) []uint8 {
	const cols, rows = 32, 24
	sig := make([]uint8, cols*rows)
	if len(src) < w*h*4 || w <= 0 || h <= 0 || r.w < 1 || r.h < 1 {
		return sig
	}
	for y := 0; y < rows; y++ {
		py := r.y + (2*y+1)*r.h/(2*rows)
		if py >= h {
			py = h - 1
		}
		for x := 0; x < cols; x++ {
			px := r.x + (2*x+1)*r.w/(2*cols)
			if px >= w {
				px = w - 1
			}
			p := (py*w + px) * 4
			sig[y*cols+x] = uint8((int(src[p])*11 + int(src[p+1])*59 + int(src[p+2])*30) / 100)
		}
	}
	return sig
}
func movementScore(before, after []uint8) float64 {
	if len(before) != len(after) || len(after) == 0 {
		return 0
	}
	changed := 0
	for i, v := range after {
		d := int(v) - int(before[i])
		if d < -18 || d > 18 {
			changed++
		}
	}
	fraction := float64(changed) / float64(len(after))
	// Small moving silhouettes may occupy only a few samples.
	if fraction < .003 {
		return 0
	}
	return math.Min(1, fraction)
}
func (s *scanScheduler) observe(frame []byte, w, h int, regions []scanRegion) bool {
	if len(regions) != len(s.lastScan) || w != s.width || h != s.height {
		return false
	}
	hardCuts, compared := 0, 0
	for i := 1; i < len(regions); i++ {
		cur := tinySignature(frame, w, h, regions[i])
		if s.fingerprint[i] != nil {
			measured := movementScore(s.fingerprint[i], cur)
			compared++
			if measured >= .72 {
				hardCuts++
			}
			// Motion persists briefly through occasional compression/decoding stalls.
			s.activity[i] = math.Max(measured, s.activity[i]*.48)
		}
		s.fingerprint[i] = cur
        if s.previousProfile == 3 && len(regions)==17 && i < 17 && i < len(s.coreFingerprint) {
            col,row:=(i-1)%4,(i-1)/4
            x1,x2:=col*w/4,(col+1)*w/4
            y1,y2:=row*h/4,(row+1)*h/4
            core:=tinySignature(frame,w,h,scanRegion{x1,y1,x2-x1,y2-y1})
            if s.coreFingerprint[i]!=nil {
                amount:=movementScore(s.coreFingerprint[i],core)
                if amount>.014 && s.coreActivity[i]<.008 {
                    s.motionArrival[i]=true
                }
                s.coreActivity[i]=math.Max(amount,s.coreActivity[i]*.48)
            }
            s.coreFingerprint[i]=core
        }
	}
	// Require a synchronized change of at least 2/3 camera tiles. A walking
	// person in one feed or compression noise must never reset all tracking.
	return compared >= 4 && hardCuts*3 >= compared*2
}
func (s *scanScheduler) markScanned(index int, now time.Time, detected bool) {
	if index < 0 || index >= len(s.lastScan) {
		return
	}
	s.lastScan[index] = now
    if index < len(s.motionArrival) { s.motionArrival[index] = false }
	if index == 0 {
		s.lastFull = now
	}
	if detected {
		s.recentlyFound[index] = now
	}
}
func (s *scanScheduler) next(now time.Time, profile int32) int {
	if len(s.lastScan) < 2 {
		return 0
	}
	fullInterval := 55 * time.Second
	if profile == 1 {
		fullInterval = 16 * time.Second
	}
	if profile == 2 {
		fullInterval = 32 * time.Second
	}
	if s.lastFull.IsZero() || now.Sub(s.lastFull) >= fullInterval {
		return 0
	}

	best, bestScore := 1, -1.0
	for i := 1; i < len(s.lastScan); i++ {
		age := 120.0
		if !s.lastScan[i].IsZero() {
			age = now.Sub(s.lastScan[i]).Seconds()
		}
		if age < 0 {
			age = 0
		}
		// Age grows unbounded until this tile is checked, ensuring non-moving
		// persons are still inspected even with constant motion elsewhere.
		score := math.Min(age, 120)*3.0 + s.activity[i]*34
		if !s.recentlyFound[i].IsZero() && now.Sub(s.recentlyFound[i]) < 12*time.Second {
			score += 15
		}
		if score > bestScore {
			bestScore, best = score, i
		}
	}
	return best
}

// firstVisitRegion returns the most promising camera never checked since
// the current mosaic/layout started. Movement matters, but a quiet tile
// must still get a first look: a motion-only detector misses standing people.
func (s *scanScheduler) firstVisitRegion(regions []scanRegion) int {
    best:=-1
    bestScore:=-1.0
    for i:=1;i<len(regions) && i<len(s.lastScan);i++ {
        if !s.lastScan[i].IsZero() {continue}
        score:=s.activity[i]*100
        if i<len(s.coreActivity) {score+=s.coreActivity[i]*120}
        if i<len(s.motionArrival) && s.motionArrival[i] {score+=50}
        // Stable tie-break by physical camera index to guarantee coverage
        // even for a long series of perfectly stationary images.
        if best<0 || score>bestScore {best,bestScore=i,score}
    }
    return best
}

// At startup, reserve two out of each three full-camera inference slots
// for a feed not yet visited. Preserve one slot in three for revisiting
// confirmed/tentative people. Once all cameras were checked, the original
// adaptive scoring and immediate weak-candidate confirmation resume.
func (s *scanScheduler) ensureFirstCoverage(profile int32, regions []scanRegion, proposed int) int {
    if profile!=3 || len(regions)<3 {return proposed}
    candidate:=s.firstVisitRegion(regions)
    if candidate<0 {return proposed}
    s.coverageDecision++
    // When all unseen screens are quiet, preserve most slots for feeds
    // with already verified humans. Quiet backgrounds still receive
    // periodic exploration; do not spend 2/3 CPU on empty camera tiles.
    motion:=s.activity[candidate]
    if candidate<len(s.coreActivity) && s.coreActivity[candidate]>motion {motion=s.coreActivity[candidate]}
    if motion<.012 {
        if s.coverageDecision%4==0 {return candidate}
        return proposed
    }
    if s.coverageDecision%3!=0 {return candidate}
    return proposed
}

// scanOwner assigns a person to its *unpadded* nominal screen tile.
// ROI inference deliberately receives an extra 12% overlap to avoid cutting
// partial bodies. That extra video belongs to a different tile: if its person
// is treated as a hit in the neighbouring camera, the adaptive scheduler can
// spend the entire CPU budget repeatedly scanning an empty padded camera.
// This changes ONLY scheduling, never suppresses output detections or assumes
// the physical RVi video pane is perfectly aligned with our nominal grid.
func (s *scanScheduler) scanOwner(box Rect) int {
    if s.previousProfile != 3 || s.width <= 0 || s.height <= 0 || box.W <= 0 || box.H <= 0 {
        return 0
    }
    cx, cy := box.X + box.W*.5, box.Y + box.H*.5
    if cx < 0 || cy < 0 || cx >= float64(s.width) || cy >= float64(s.height) {
        return 0
    }
    col := int(cx * 4 / float64(s.width))
    row := int(cy * 4 / float64(s.height))
    if col < 0 || col > 3 || row < 0 || row > 3 {return 0}
    // The optional two-vertically-stacked-camera research layout contains
    // eight crops: grouped rows 0+1 and 2+3, four columns each.
    // Keep box ownership aligned with those crops without changing
    // normal 16-camera behavior.
    if len(s.lastScan)==9 {return (row/2)*4+col+1}
    return row * 4 + col + 1
}

// markScannedPeople only marks a CAMERA positive when at least one NN box
// belongs to its own unpadded core. We still send ALL detections to the
// existing tracker, including humans crossing nominal pixel boundaries.
func (s *scanScheduler) markScannedPeople(index int, at time.Time, people []Rect) {
    positive:=false
    for _,person:=range people {
        if index==0 || s.previousProfile!=3 || s.scanOwner(person)==index {
            positive=true
            break
        }
    }
    s.markScanned(index,at,positive)
}

// Prevent ghosts from lingering across a full 16-camera scan cycle.
// When somebody has already been seen, revisit that person's camera about
// every four seconds; this makes disappearance detection responsive without
// rescanning a quiet camera on every frame. Other tiles remain eligible.
func (s *scanScheduler) priorityTracked(views []TrackView, regions []scanRegion, now time.Time) int {
	best, bestAge := -1, 4.0
	for _, v := range views {
		for i := 1; i < len(regions); i++ {
			if i >= len(s.lastScan) {
				break
			}
			r := regions[i]
			bx1 := math.Max(v.Box.X, float64(r.x))
			by1 := math.Max(v.Box.Y, float64(r.y))
			bx2 := math.Min(v.Box.X+v.Box.W, float64(r.x+r.w))
			by2 := math.Min(v.Box.Y+v.Box.H, float64(r.y+r.h))
			// Two negative extents may otherwise multiply into a positive area.
			if bx2 <= bx1 || by2 <= by1 {
				continue
			}
			if (bx2-bx1)*(by2-by1) < .82*v.Box.W*v.Box.H {
				continue
			}
			age := 120.0
			if !s.lastScan[i].IsZero() {
				age = now.Sub(s.lastScan[i]).Seconds()
			}
			if age > bestAge {
				best, bestAge = i, age
			}
		}
	}
	return best
}

// A 16-camera mosaic cannot send sixteen 640x640 neural inferences at
// once on one CPU worker. Allocate scarce inference slots to movement,
// then to overdue known humans, without starving unchanged cameras.
// Unlike priorityTracked, this includes every camera in a single fair
// decision; an old ghost cannot permanently monopolize the detector.
func (s *scanScheduler) nextAdaptive(now time.Time, profile int32, views []TrackView, regs []scanRegion) int {
	if len(regs) < 2 {
		return 0
	}
	if profile != 3 {
		return s.next(now, profile)
	}
	if len(s.lastScan) != len(regs) {
		return 0
	}
	if s.firstDecision.IsZero() {
		s.firstDecision = now
	}
	best, bestScore := 1, math.Inf(-1)
    trackedBest, trackedScore, trackedAge := -1, math.Inf(-1), 0.0
    exploreBest, exploreScore, exploreAge := -1, math.Inf(-1), 0.0
    knownCameras := 0
	for i := 1; i < len(regs); i++ {
		// Unvisited tiles must age too. A constant age allowed a moving feed
		// to starve every other camera forever, including stationary people.
		age := 2.0 + math.Max(0, now.Sub(s.firstDecision).Seconds())
		if !s.lastScan[i].IsZero() {
			age = now.Sub(s.lastScan[i]).Seconds()
		}
		if age < 0 {
			age = 0
		}
		// The oldest camera gets scanned eventually even if a different camera
		// has perpetual movement (e.g. trees, reflections, compression noise).
		score := math.Min(30, age) * 6
		score += math.Min(1, s.activity[i]) * 180
        if i<len(s.motionArrival) && s.motionArrival[i] {
            score += 400
        }
		if s.activity[i] > .012 {
			score += 22
		}
		// Once a person has been seen, keep that tile in a bounded neural
		// revalidation loop. The former +30 bonus after 2.8 seconds let a
		// queue of quiet never-visited cameras monopolize the only AI worker,
		// dropping actively moving people for 5+ seconds in the v20 replay.
		if !s.recentlyFound[i].IsZero() && now.Sub(s.recentlyFound[i]) < 7*time.Second && age >= 1 {
			score += 20
		}
		knownHuman := false
		for _, v := range views {
            if s.scanOwner(v.Box)!=i {
                continue
            }
			if !scanContainsBox(regs[i], v.Box, .82) {
				continue
			}
			knownHuman = true
            if age >= 1 {
				score += 75
			}
			if age >= 2.5 {
				score += 20
			}
			break
		}
		if s.lastScan[i].IsZero() {
			score += 8
		}
		// Introduce quiet cameras smoothly as they age instead of a sudden
		// all-quiet scan burst. Long-overdue cameras still outrank active feeds.
		if age > 5 {
			score += math.Min(8, age-5) * 13
		}
		if age > 13 {
			score += 200
		}
        // Hard bounded starvation despite persistent flashing/new motion.
        if age > 18 {score += 600}
		if score > bestScore {
			best, bestScore = i, score
		}
        if knownHuman {
            knownCameras++
            if score > trackedScore { trackedBest, trackedScore, trackedAge = i, score, age }
        } else if score > exploreScore {
            exploreBest, exploreScore, exploreAge = i, score, age
        }
	}
    // A single expensive inference worker must split its time between
    // verification of visible humans and discovery of new arrivals. Scores
    // alone can create multi-second all-empty-camera bursts after 13 seconds.
    // Explicit quotas avoid that while making the ratio adapt to crowd size.
    if trackedBest > 0 && exploreBest > 0 {
        s.decisionCount++
        chooseTracked := false
        switch {
        case knownCameras <= 1:
            chooseTracked = s.decisionCount%4 == 0
        case knownCameras <= 2:
            chooseTracked = s.decisionCount%3 == 0
        case knownCameras <= 7:
            chooseTracked = s.decisionCount%2 == 1
        case knownCameras <= 11:
            chooseTracked = s.decisionCount%4 != 0
        default:
            chooseTracked = s.decisionCount%8 != 0
        }
        // One or two known people should not disappear merely because
        // other cameras contain moving trees. But never indefinitely deny a
        // camera that has been quiet for over 13 seconds.
        if knownCameras <= 6 && trackedAge >= 4 && exploreAge < 13 {
            chooseTracked = true
        }
        if exploreBest > 0 && exploreBest < len(s.motionArrival) && s.motionArrival[exploreBest] {
            return exploreBest
        }
        if chooseTracked { return trackedBest }
        return exploreBest
    }
	return best
}
func scanContainsBox(r scanRegion, b Rect, threshold float64) bool {
	if b.W <= 0 || b.H <= 0 {
		return false
	}
	x1 := math.Max(b.X, float64(r.x))
	y1 := math.Max(b.Y, float64(r.y))
	x2 := math.Min(b.X+b.W, float64(r.x+r.w))
	y2 := math.Min(b.Y+b.H, float64(r.y+r.h))
	if x2 <= x1 || y2 <= y1 {
		return false
	}
	return (x2-x1)*(y2-y1) >= threshold*b.W*b.H
}

// A weak first detection is hidden until an independent neural confirmation.
// Retry it promptly, while preserving the existing hard camera-fairness rule.
func(s *scanScheduler)confirmationRecheck(now time.Time,candidates []TrackView,regs []scanRegion,fallback int)int{
 if len(candidates)==0||len(regs)<2{return fallback}
 ageOf:=func(index int)float64{
  if index<0||index>=len(s.lastScan){return 0}
  if !s.lastScan[index].IsZero(){return math.Max(0,now.Sub(s.lastScan[index]).Seconds())}
  if s.firstDecision.IsZero(){return 0}
  return 2+math.Max(0,now.Sub(s.firstDecision).Seconds())
 }
 if ageOf(fallback)>13{return fallback}
 best,oldest:=fallback,.5
 for _,v:=range candidates{for i:=1;i<len(regs);i++{
  // Core ownership applies only to the 4x4 adaptive grid. Other profiles
  // retain their existing padded-ROI candidate confirmation behavior.
  if s.previousProfile==3 && s.scanOwner(v.Box)!=i {continue}
  if !scanContainsBox(regs[i],v.Box,.9){continue}
  age:=ageOf(i)
  if age>oldest{best,oldest=i,age}
 }}
 return best
}

func throttleFor(profile int32, busy time.Duration) time.Duration {
	if busy <= 0 {
		busy = 100 * time.Millisecond
	}
	var factor float64
	var minimum, maximum time.Duration
	switch profile {
	case 1:
		factor, minimum, maximum = 1.7, 240*time.Millisecond, 3*time.Second
	case 3:
		factor, minimum, maximum = .07, 50*time.Millisecond, 400*time.Millisecond
	default:
		factor, minimum, maximum = .55, 140*time.Millisecond, 1500*time.Millisecond
	}
	delay := time.Duration(float64(busy) * factor)
	if delay < minimum {
		delay = minimum
	}
	if delay > maximum {
		delay = maximum
	}
	return delay
}

// Neural revalidation must tolerate one scan cycle through the 16-camera
// mosaic, even on slower CPUs. Targeted negative AI scans and visual loss
// remove obsolete tracks early; the upper limit prevents permanent ghosts.
func recheckTTL(regions int, profile int32, busy time.Duration) time.Duration {
	if regions < 1 {
		regions = 1
	}
	if busy <= 0 {
		busy = 350 * time.Millisecond
	}
	cycle := time.Duration(regions) * (busy + throttleFor(profile, busy))
	ttl := cycle + cycle/3
	if ttl < 9*time.Second {
		ttl = 9 * time.Second
	}
	if ttl > 16*time.Second {
		ttl = 16 * time.Second
	}
	return ttl
}
