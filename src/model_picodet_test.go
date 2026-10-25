//go:build picodet

package main

import "testing"

func TestPicoDetDecodePersonAndUnletterbox(t *testing.T) {
	// PicoDet stretches 640x320 source to a 320x320 model input.
	raw := []float32{
		50, 100, 110, 220, .90, 0,
		52, 102, 108, 218, .80, 0,
		150, 100, 180, 160, .99, 2,
		0, 0, 0, 0, .99, 0,
	}
	got := decodeActiveModel(raw, 4, 640, 320, 1, 0, 0, .32)
	if len(got) != 1 { t.Fatalf("expected only person after NMS and class filter, got %+v",got) }
	if got[0].X!=100 || got[0].Y!=100 || got[0].W!=120 || got[0].H!=120 {
		t.Fatalf("incorrect original pixels: %+v",got[0])
	}
}
func TestPicoDetDecodeRejectsCorruptAndNonPerson(t *testing.T) {
	if got:=decodeActiveModel([]float32{0,0,40,40,.9,1},1,320,320,1,0,0,.2);len(got)!=0 {t.Fatalf("nonperson detected %+v",got)}
	if got:=decodeActiveModel([]float32{0,0,40,40,.9,0},2,320,320,1,0,0,.2);len(got)!=0 {t.Fatalf("short output accepted %+v",got)}
	if modelInputSize!=320 {t.Fatalf("unexpected PicoDet input %d",modelInputSize)}
}

func TestPicoDetStretchedPreprocessingUsesWholeImage(t *testing.T){
	// 640x320 red input -> 320x320 uniform RGB float with no letterbox.
	pixels:=make([]byte,640*320*4)
	for i:=0;i<len(pixels);i+=4 {pixels[i+2]=255;pixels[i+3]=255}
	planar,scale,px,py:=prepareActiveModelFrame(pixels,640,320,scanRegion{0,0,640,320},false)
	if len(planar)!=3*320*320||scale!=1||px!=0||py!=0{t.Fatal("bad PicoDet input dimensions")}
	for _,y:=range []int{0,160,319}{
		p:=y*320+160
		if planar[p]<.99||planar[320*320+p]>.001||planar[2*320*320+p]>.001{
			t.Fatalf("PicoDet has unexpected letterbox/colour at y=%d",y)
		}
	}
}
