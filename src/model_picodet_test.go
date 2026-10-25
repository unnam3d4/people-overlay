//go:build picodet

package main

import "testing"

func TestPicoDetDecodePersonAndUnletterbox(t *testing.T) {
	// Input 320px is letterboxed: original 640x320 scaled by 0.5;
	// image occupies y=80..240 with scale=0.5 and padY=80.
	raw := []float32{
		50, 100, 110, 220, .90, 0,
		52, 102, 108, 218, .80, 0,
		150, 100, 180, 160, .99, 2,
		0, 0, 0, 0, .99, 0,
	}
	got := decodeActiveModel(raw, 4, 640, 320, .5, 0, 80, .32)
	if len(got) != 1 { t.Fatalf("expected only person after NMS and class filter, got %+v",got) }
	if got[0].X!=100 || got[0].Y!=40 || got[0].W!=120 || got[0].H!=240 {
		t.Fatalf("incorrect original pixels: %+v",got[0])
	}
}
func TestPicoDetDecodeRejectsCorruptAndNonPerson(t *testing.T) {
	if got:=decodeActiveModel([]float32{0,0,40,40,.9,1},1,320,320,1,0,0,.2);len(got)!=0 {t.Fatalf("nonperson detected %+v",got)}
	if got:=decodeActiveModel([]float32{0,0,40,40,.9,0},2,320,320,1,0,0,.2);len(got)!=0 {t.Fatalf("short output accepted %+v",got)}
	if modelInputSize!=320 {t.Fatalf("unexpected PicoDet input %d",modelInputSize)}
}
