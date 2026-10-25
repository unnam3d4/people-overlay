//go:build picodet

package main

import "math"

// Official PicoDet-S320 non-postprocess NCNN weights: 320px fixed square
// *stretched* resize (keep_ratio=False in PaddleDetection TestReader).
// Do not feed YOLO's aspect-preserving letterbox to this model.
const modelInputSize = 320
const minSelftestPersons = 3

// PicoDet-S 320 uses the same number of AI calls and CPU threads as before,
// while covering unseen tiles sooner. A confirmed person should not be
// artificially excluded just because there are 49+ persons in one display.
const picoCoverageBurst = true
const modelMaxTrackedPeople = 64

func prepareActiveModelFrame(src []byte, w,h int, reg scanRegion, enhance bool)([]float32,float64,float64,float64){
	const n=modelInputSize
	if w<1||h<1||reg.w<1||reg.h<1||reg.x<0||reg.y<0||
	   reg.x+reg.w>w||reg.y+reg.h>h||len(src)<w*h*4{return nil,0,0,0}
	out:=make([]float32,3*n*n)
	low,high:=0.0,255.0
	if enhance{low,high=regionContrastRange(src,w,reg)}
	gain:=255.0/math.Max(65,high-low)
	for y:=0;y<n;y++{
		fy:=(float64(y)+.5)*float64(reg.h)/n-.5
		if fy<0{fy=0};if fy>float64(reg.h-1){fy=float64(reg.h-1)}
		sy:=int(fy);dy:=fy-float64(sy);sy2:=minScan(sy+1,reg.h-1)
		for x:=0;x<n;x++{
			fx:=(float64(x)+.5)*float64(reg.w)/n-.5
			if fx<0{fx=0};if fx>float64(reg.w-1){fx=float64(reg.w-1)}
			sx:=int(fx);dx:=fx-float64(sx);sx2:=minScan(sx+1,reg.w-1)
			p00:=((reg.y+sy)*w+reg.x+sx)*4
			p10:=((reg.y+sy)*w+reg.x+sx2)*4
			p01:=((reg.y+sy2)*w+reg.x+sx)*4
			p11:=((reg.y+sy2)*w+reg.x+sx2)*4
			for c:=0;c<3;c++{
				ch:=2-c
				a:=float64(src[p00+ch])*(1-dx)+float64(src[p10+ch])*dx
				b:=float64(src[p01+ch])*(1-dx)+float64(src[p11+ch])*dx
				v:=a*(1-dy)+b*dy
				if enhance{v=math.Max(0,math.Min(255,(v-low)*gain))}
				out[c*n*n+y*n+x]=float32(v/255)
			}
		}
	}
	// Mapping is handled by per-axis rescaling in decodeActiveModel.
	return out,1,0,0
}

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
		x1 = maxF(0, minF(x1*float64(fullW)/modelInputSize, float64(fullW)))
		y1 = maxF(0, minF(y1*float64(fullH)/modelInputSize, float64(fullH)))
		x2 = maxF(0, minF(x2*float64(fullW)/modelInputSize, float64(fullW)))
		y2 = maxF(0, minF(y2*float64(fullH)/modelInputSize, float64(fullH)))
		if x2-x1 < 6 || y2-y1 < 9 { continue }
		candidates = append(candidates, Rect{X:x1, Y:y1, W:x2-x1, H:y2-y1, Score:score})
	}
	return suppressRects(candidates, .45, 80)
}

// Allow a bounded extension only while the screen texture is re-verified.
const picoVisualRevalidation = true

const picoDetectorCapable = true
