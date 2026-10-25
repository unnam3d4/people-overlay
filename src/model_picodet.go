//go:build picodet

package main

// PicoDet-S 320 proof-of-concept uses the official pre-converted NCNN model.
// 416px needs its own correctly converted assets and matching build; never
// pretend the 320px weights are interchangeable with a 416px input.
const modelInputSize = 320
const minSelftestPersons = 1

// Native PicoDet adapter outputs (x1,y1,x2,y2,score,classID) in 320px
// letterboxed coordinates. This is NOT YOLO's [84,N] layout.
func decodeActiveModel(out []float32, n, fullW, fullH int, scale, padX, padY float64, threshold float32) []Rect {
	if n < 0 || n > 100 || len(out) < 6*n || fullW <= 0 || fullH <= 0 ||
		scale <= 0 || !finiteDetection(scale) || !finiteDetection(padX) ||
		!finiteDetection(padY) || !finiteDetection(float64(threshold)) {
		return nil
	}
	candidates := make([]Rect, 0, n)
	for i := 0; i < n; i++ {
		base := 6 * i
		score := out[base+4]
		// COCO class zero represents person; other categories are discarded.
		if out[base+5] != 0 || !finiteDetection(float64(score)) ||
			score < threshold || score > 1 {
			continue
		}
		x1, y1 := float64(out[base]), float64(out[base+1])
		x2, y2 := float64(out[base+2]), float64(out[base+3])
		if !finiteDetection(x1) || !finiteDetection(y1) ||
			!finiteDetection(x2) || !finiteDetection(y2) || x2 <= x1 || y2 <= y1 {
			continue
		}
		x1 = maxF(0, minF((x1-padX)/scale, float64(fullW)))
		y1 = maxF(0, minF((y1-padY)/scale, float64(fullH)))
		x2 = maxF(0, minF((x2-padX)/scale, float64(fullW)))
		y2 = maxF(0, minF((y2-padY)/scale, float64(fullH)))
		if x2-x1 < 6 || y2-y1 < 9 { continue }
		candidates = append(candidates, Rect{X:x1, Y:y1, W:x2-x1, H:y2-y1, Score:score})
	}
	return suppressRects(candidates, .45, 80)
}
