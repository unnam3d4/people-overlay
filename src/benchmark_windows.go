//go:build windows

package main

// Offline pixel replay. Inference is executed synchronously but its measured
// worker time schedules delivery on the video clock. There is one pending
// result and no frame queue. Disk decoding is excluded from the worker time.
import (
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"time"
)

type benchmarkPerson struct {
	ID  string `json:"id"`
	Box Rect   `json:"box"`
}
type benchmarkLandmark struct {
    ID string `json:"id"`
    X float64 `json:"x"`
    Y float64 `json:"y"`
}
type benchmarkFrame struct {
	File   string            `json:"file"`
	People []benchmarkPerson `json:"people"`
    // Optional manually labelled head landmarks: not full-body bbox GT.
    Landmarks []benchmarkLandmark `json:"landmarks,omitempty"`
	Label  string            `json:"label,omitempty"`
}
type benchmarkManifest struct {
	Name        string            `json:"name"`
	Source      map[string]string `json:"source"`
	FPS         int               `json:"fps"`
	Profile     int32             `json:"profile"`
	Sensitivity int32             `json:"sensitivity"`
	Threads     int               `json:"threads"`
	Contrast    bool              `json:"contrast"`
	ResourceDir string            `json:"resource_dir,omitempty"`
	// Zero uses the actual production tracking/capture cadence. Positive values
	// explicitly override it for a tracker stress test, and are reported.
	CaptureCadenceMS float64          `json:"capture_cadence_ms,omitempty"`
 // Benchmark-only artificial lower bound for neural latency, in video clock
 // time. Does not sleep and does not alter reported actual NCNN CPU times.
 InferenceDelayFloorMS float64 `json:"inference_delay_floor_ms,omitempty"`
	Frames           []benchmarkFrame `json:"frames"`
}
type benchmarkTimes struct {
	LoadMS      float64 `json:"load_ms"`
	SchedulerMS float64 `json:"scheduler_ms"`
	TrackingMS  float64 `json:"tracking_ms"`
	ApplyMS     float64 `json:"apply_ms"`
	CopyMS      float64 `json:"copy_ms"`
	PrepareMS   float64 `json:"prepare_ms"`
	InferMS     float64 `json:"infer_ms"`
	DecodeMS    float64 `json:"decode_ms"`
}
type benchmarkObservation struct {
	Frame       int               `json:"frame"`
	TimestampMS float64           `json:"timestamp_ms"`
	File        string            `json:"file"`
	Label       string            `json:"label,omitempty"`
	People      []benchmarkPerson `json:"people"`
    Landmarks []benchmarkLandmark `json:"landmarks,omitempty"`
	Tracks      []TrackView       `json:"tracks"`
	Captured    bool              `json:"captured"`
	SceneReset  bool              `json:"scene_reset,omitempty"`
	Times       benchmarkTimes    `json:"times"`
}
type benchmarkInference struct {
	CaptureFrame int            `json:"capture_frame"`
	CapturedMS   float64        `json:"captured_ms"`
	ReadyMS      float64        `json:"ready_ms"`
	AppliedMS    *float64       `json:"applied_ms"`
	Rejected     bool           `json:"rejected"`
	RegionIndex  int            `json:"region_index"`
	Region       Rect           `json:"region"`
	Detections   []Rect         `json:"detections"`
	Times        benchmarkTimes `json:"times"`
}
type benchmarkReport struct {
	Schema       int                    `json:"schema"`
	Manifest     benchmarkManifest      `json:"manifest"`
	Protocol     string                 `json:"protocol"`
	CaptureAware bool                   `json:"capture_aware"`
	Width        int                    `json:"width"`
	Height       int                    `json:"height"`
	WallMS       float64                `json:"wall_ms"`
	Frames       []benchmarkObservation `json:"frames"`
	Inferences   []benchmarkInference   `json:"inferences"`
}

func benchmarkMS(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }
func benchmarkWrite(path string, v interface{}) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		return e
	}
	return os.WriteFile(path, append(b, '\n'), 0644)
}
func benchmarkLoad(path string) ([]byte, int, int, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, 0, 0, e
	}
	defer f.Close()
	img, e := png.Decode(f)
	if e != nil {
		return nil, 0, 0, e
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w < 1 || h < 1 || w > 10000 || h > 10000 {
		return nil, 0, 0, fmt.Errorf("invalid PNG dimensions")
	}
	pixels := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			p := (y*w + x) * 4
			pixels[p], pixels[p+1], pixels[p+2], pixels[p+3] = byte(b>>8), byte(g>>8), byte(r>>8), 255
		}
	}
	return pixels, w, h, nil
}
func benchmarkPredict(d *Detector, pixels []byte, w, h int, zone scanRegion, threshold float32, contrast bool) ([]Rect, benchmarkTimes, error) {
	var tm benchmarkTimes
	begun := time.Now()
	input, scale, px, py := prepareRegionFrameEnhanced(pixels, w, h, scanRegion{0, 0, w, h}, contrast)
	tm.PrepareMS = benchmarkMS(time.Since(begun))
	if input == nil {
		return nil, tm, fmt.Errorf("invalid benchmark frame")
	}
	begun = time.Now()
	raw, n, e := d.Predict(input)
	tm.InferMS = benchmarkMS(time.Since(begun))
	if e != nil {
		return nil, tm, e
	}
	begun = time.Now()
	found := decodePeople(raw, n, w, h, scale, px, py, threshold)
	for i := range found {
		found[i].X += float64(zone.x)
		found[i].Y += float64(zone.y)
	}
	tm.DecodeMS = benchmarkMS(time.Since(begun))
	return found, tm, nil
}

type benchmarkCaptureObserver interface {
	observePeopleFromCapture([]Rect, []TrackView, []byte, scanRegion, time.Time, time.Time)
}
type benchmarkNegativeObserver interface {
	noteRegionScanSnapshot(scanRegion, time.Time, time.Time, int, []TrackView)
}

// runBenchmarkCLI expects os.Args[1:] including --benchmark or --detect-image.
func runBenchmarkCLI(args []string) error {
	if len(args) != 3 {
		return fmt.Errorf("usage: --benchmark manifest.json output.json | --detect-image input.png output.json")
	}
	if args[0] == "--detect-image" {
		root, e := locateResources()
		if e != nil {
			return e
		}
		d, e := NewDetector(root)
		if e != nil {
			return e
		}
		defer d.Close()
		if control, ok := interface{}(d).(interface{ SetThreads(int) }); ok {
			control.SetThreads(1)
		}
		pixels, w, h, e := benchmarkLoad(args[1])
		if e != nil {
			return e
		}
		found, tm, e := benchmarkPredict(d, pixels, w, h, scanRegion{0, 0, w, h}, .32, false)
		if e != nil {
			return e
		}
		return benchmarkWrite(args[2], map[string]interface{}{"width": w, "height": h, "detections": found, "times": tm, "network": 640})
	}
	if args[0] != "--benchmark" {
		return fmt.Errorf("unknown benchmark command %s", args[0])
	}
	b, e := os.ReadFile(args[1])
	if e != nil {
		return e
	}
	var manifest benchmarkManifest
	if e = json.Unmarshal(b, &manifest); e != nil {
		return e
	}
	if manifest.FPS != 15 && manifest.FPS != 30 {
		return fmt.Errorf("benchmark FPS must be 15 or 30")
	}
	if len(manifest.Frames) == 0 {
		return fmt.Errorf("benchmark has no PNG frames")
	}
	if manifest.Profile < 0 || manifest.Profile > 3 {
		return fmt.Errorf("invalid profile")
	}
	if manifest.Sensitivity == 0 {
		manifest.Sensitivity = 2
	}
	if manifest.Threads == 0 {
		manifest.Threads = 1
	}
	if manifest.Threads < 1 || manifest.Threads > 2 {
		return fmt.Errorf("threads must be 1 or 2")
	}
 if manifest.InferenceDelayFloorMS < 0 || manifest.InferenceDelayFloorMS > 5000 {
  return fmt.Errorf("inference_delay_floor_ms must be between 0 and 5000")
 }
	base, e := filepath.Abs(filepath.Dir(args[1]))
	if e != nil {
		return e
	}
	root := manifest.ResourceDir
	if root == "" {
		root, e = locateResources()
		if e != nil {
			return e
		}
	} else if !filepath.IsAbs(root) {
		root = filepath.Join(base, root)
	}
	d, e := NewDetector(root)
	if e != nil {
		return e
	}
	defer d.Close()
	if control, ok := interface{}(d).(interface{ SetThreads(int) }); ok {
		control.SetThreads(manifest.Threads)
	}
	var sch scanScheduler
	var tracker personTracker
	if configurable, ok := interface{}(&tracker).(interface{ enableConfirmation() }); ok {
		configurable.enableConfirmation()
	}
	viewsForScheduler := func(now time.Time) []TrackView {
		if candidates, ok := interface{}(&tracker).(interface{ schedulerViews(time.Time) []TrackView }); ok {
			return candidates.schedulerViews(now)
		}
		return tracker.views(now)
	}
	aware, hasAware := interface{}(&tracker).(benchmarkCaptureObserver)
	negative, hasNegative := interface{}(&tracker).(benchmarkNegativeObserver)
	report := benchmarkReport{Schema: 1, Manifest: manifest, CaptureAware: hasAware, Protocol: "real PNG pixels -> production 640 preprocessing -> Detector.Predict -> decodePeople; deterministic video timestamps; one pending worker; measured NCNN delivery or manifest lower-bound 300/700 ms delay when set; production capture cadence unless explicitly overridden; disk loading excluded; synchronous offline replay does not model concurrent CPU contention. InferenceDelayFloorMS adds video-clock latency, not real CPU work."}
	start := time.Now()
	origin := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	var regs []scanRegion
	var nextCapture, nextInference time.Time
	var pending bool
	var pendingIndex int
	var pendingPixels []byte
	var pendingViews []TrackView
	var pendingAt, ready time.Time
	var pendingEpoch, epoch int
	var published []TrackView
	lastBusy := 350 * time.Millisecond
	for i, frame := range manifest.Frames {
		now := origin.Add(time.Duration(float64(i) * float64(time.Second) / float64(manifest.FPS)))
		obs := benchmarkObservation{Frame: i, TimestampMS: benchmarkMS(now.Sub(origin)), File: frame.File, Label: frame.Label, People: frame.People, Landmarks: frame.Landmarks}
		if i == 0 || !now.Before(nextCapture) {
			obs.Captured = true
			begun := time.Now()
			path := frame.File
			if !filepath.IsAbs(path) {
				path = filepath.Join(base, path)
			}
			pixels, w, h, e := benchmarkLoad(path)
			obs.Times.LoadMS = benchmarkMS(time.Since(begun))
			if e != nil {
				return fmt.Errorf("frame %d: %w", i, e)
			}
			if i == 0 {
				report.Width, report.Height = w, h
				regs = regionsForProfile(w, h, manifest.Profile)
				sch.reset(len(regs), w, h, manifest.Profile)
				if manifest.Profile == 3 {
					sch.lastFull = now
				}
				cols := 0
				if manifest.Profile == 3 {
					cols = 4
				}
				tracker.layout(w, h, cols)
			} else if w != report.Width || h != report.Height {
				return fmt.Errorf("frame %d changes dimensions", i)
			}
			begun = time.Now()
			if sch.observe(pixels, w, h, regs) {
				epoch++
				tracker.reset()
				cols := 0
				if manifest.Profile == 3 {
					cols = 4
				}
				tracker.layout(w, h, cols)
				sch.reset(len(regs), w, h, manifest.Profile)
				if manifest.Profile == 3 {
					sch.lastFull = now
				}
				nextInference = time.Time{}
				obs.SceneReset = true
			}
			obs.Times.SchedulerMS = benchmarkMS(time.Since(begun))
			begun = time.Now()
			tracker.step(pixels, w, h, now)
			obs.Times.TrackingMS = benchmarkMS(time.Since(begun))
			if pending && !now.Before(ready) {
				pending = false
				result := &report.Inferences[pendingIndex]
				begun = time.Now()
				if pendingEpoch == epoch {
					applied := benchmarkMS(now.Sub(origin))
					result.AppliedMS = &applied
					zone := regs[result.RegionIndex]
					sch.markScannedPeople(result.RegionIndex, now, result.Detections)
					if hasAware {
						aware.observePeopleFromCapture(result.Detections, pendingViews, pendingPixels, zone, pendingAt, now)
					} else {
						tracker.observePeople(compensateLateDetections(result.Detections, pendingViews, tracker.views(now)), now)
					}
					if result.RegionIndex != 0 || len(regs) == 1 {
						tile := 0
						if manifest.Profile == 3 {
							tile = result.RegionIndex
						}
						if hasNegative {
							negative.noteRegionScanSnapshot(zone, now, pendingAt, tile, pendingViews)
						} else {
							tracker.noteRegionScanCaptured(zone, now, pendingAt, tile)
						}
					}
					lastBusy = time.Duration((result.Times.PrepareMS + result.Times.InferMS + result.Times.DecodeMS) * float64(time.Millisecond))
     if floor:=time.Duration(manifest.InferenceDelayFloorMS*float64(time.Millisecond));floor>lastBusy{lastBusy=floor}
					tracker.maxAIAge = recheckTTL(len(regs), manifest.Profile, lastBusy)
					nextInference = now.Add(throttleFor(manifest.Profile, lastBusy))
				} else {
					result.Rejected = true
				}
				obs.Times.ApplyMS = benchmarkMS(time.Since(begun))
				pendingPixels = nil
				pendingViews = nil
			}
			if !pending && !now.Before(nextInference) {
				begun = time.Now()
				index := sch.nextAdaptive(now, manifest.Profile, viewsForScheduler(now), regs)
				if candidates,ok:=interface{}(&tracker).(interface{tentativeViews(time.Time)[]TrackView});ok{
				 if retry,ok:=interface{}(&sch).(interface{confirmationRecheck(time.Time,[]TrackView,[]scanRegion,int)int});ok{index=retry.confirmationRecheck(now,candidates.tentativeViews(now),regs,index)}
				}
				obs.Times.SchedulerMS += benchmarkMS(time.Since(begun))
				zone := regs[index]
				begun = time.Now()
				crop := copyRegionBGRA(pixels, w, h, zone)
				copyMS := benchmarkMS(time.Since(begun))
				captured := viewsForScheduler(now)
				found, tm, e := benchmarkPredict(d, crop, zone.w, zone.h, zone, confidenceThreshold(manifest.Sensitivity), manifest.Contrast)
				if e != nil {
					return fmt.Errorf("frame %d inference: %w", i, e)
				}
				tm.CopyMS = copyMS
				delay := time.Duration((tm.PrepareMS + tm.InferMS + tm.DecodeMS) * float64(time.Millisecond))
    if floor:=time.Duration(manifest.InferenceDelayFloorMS*float64(time.Millisecond));floor>delay{delay=floor}
				ready = now.Add(delay)
				report.Inferences = append(report.Inferences, benchmarkInference{CaptureFrame: i, CapturedMS: obs.TimestampMS, ReadyMS: benchmarkMS(ready.Sub(origin)), RegionIndex: index, Region: Rect{X: float64(zone.x), Y: float64(zone.y), W: float64(zone.w), H: float64(zone.h)}, Detections: found, Times: tm})
				pendingIndex = len(report.Inferences) - 1
				pending = true
				pendingPixels = crop
				pendingViews = captured
				pendingAt = now
				pendingEpoch = epoch
				sch.markScanned(index, now, false)
				nextInference = now.Add(throttleFor(manifest.Profile, lastBusy))
			}
			published = append([]TrackView{}, tracker.views(now)...)
			cadence := 300 * time.Millisecond
			if len(tracker.people) > 0 {
				cadence = 210 * time.Millisecond
				if manifest.Profile == 3 {
					cadence = 115 * time.Millisecond
				}
				if manifest.Profile == 1 {
					cadence = 340 * time.Millisecond
				}
			} else if manifest.Profile == 1 {
				cadence = 440 * time.Millisecond
			}
			if current, ok := interface{}(&tracker).(interface{ captureCadence(int32) time.Duration }); ok {
				cadence = current.captureCadence(manifest.Profile)
			}
			if manifest.CaptureCadenceMS > 0 {
				cadence = time.Duration(manifest.CaptureCadenceMS * float64(time.Millisecond))
			}
			nextCapture = now.Add(cadence)
		}
		obs.Tracks = append([]TrackView{}, published...)
		report.Frames = append(report.Frames, obs)
	}
	report.WallMS = benchmarkMS(time.Since(start))
	return benchmarkWrite(args[2], report)
}
