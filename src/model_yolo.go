//go:build !picodet

package main

// All existing production and regression builds preserve 640px YOLO.
const modelInputSize = 640
const minSelftestPersons = 3

func decodeActiveModel(out []float32, n, w, h int, scale, padX, padY float64, threshold float32) []Rect {
	return decodePeople(out, n, w, h, scale, padX, padY, threshold)
}
