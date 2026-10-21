//go:build windows

package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image/jpeg"
	"time"
)

// Source: ultralytics/ultralytics ultralytics/assets/bus.jpg (AGPL-3.0).
// See testdata/SOURCE.md. Three full pedestrians are manually annotated;
// the additional person at the left edge is intentionally not required.
//
//go:embed testdata/bus-reference.jpg
var selftestJPEG []byte

func imageSelftest(d *Detector) (map[string]interface{}, error) {
	img, e := jpeg.Decode(bytes.NewReader(selftestJPEG))
	if e != nil {
		return nil, e
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	pixels := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			i := (y*w + x) * 4
			pixels[i], pixels[i+1], pixels[i+2], pixels[i+3] = byte(bl>>8), byte(g>>8), byte(r>>8), 255
		}
	}
	found, timing, e := benchmarkPredict(d, pixels, w, h, scanRegion{0, 0, w, h}, .32, false)
	if e != nil {
		return nil, e
	}
	truth := []Rect{{X: 50, Y: 398, W: 196, H: 506}, {X: 220, Y: 403, W: 129, H: 459}, {X: 667, Y: 397, W: 143, H: 480}}
	matched := 0
	for _, person := range truth {
		for _, box := range found {
			if overlap(person, box) >= .5 {
				matched++
				break
			}
		}
	}
	if matched != len(truth) {
		return nil, fmt.Errorf("real-image selftest: matched %d/%d annotated people; decoded %d", matched, len(truth), len(found))
	}
	input := make([]float32, 3*640*640)
	for i := range input {
		input[i] = 114.0 / 255.0
	}
	start := time.Now()
	out, n, e := d.Predict(input)
	if e != nil {
		return nil, e
	}
	empty := decodePeople(out, n, 640, 640, 1, 0, 0, .32)
	if len(empty) != 0 {
		return nil, fmt.Errorf("blank negative control produced %d people", len(empty))
	}
	return map[string]interface{}{"passed": true, "matched_people": matched, "detections": found, "blank_people": len(empty), "real_image_times": timing, "blank_infer_ms": float64(time.Since(start)) / float64(time.Millisecond)}, nil
}
