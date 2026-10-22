package main

import "testing"

func v3SyntheticCapture(w,h int,f func(x,y int)(byte,byte,byte)) []byte {
    pixels:=make([]byte,w*h*4)
    for y:=0;y<h;y++{
        for x:=0;x<w;x++{
            r,g,b:=f(x,y)
            i:=(y*w+x)*4
            pixels[i]=b;pixels[i+1]=g;pixels[i+2]=r;pixels[i+3]=255
        }
    }
    return pixels
}

func TestV3CaptureDiagnosticBlackMayBeGPUOrNight(t *testing.T) {
    img:=v3SyntheticCapture(800,600,func(x,y int)(byte,byte,byte){return 0,0,0})
    stats:=v3CapturePixelStats(img,800,600)
    if stats.Samples!=768 || stats.BlackPercent!=100 || !stats.PossiblyBlank || !stats.NearUniform {
        t.Fatalf("black capture must raise a caution, not be called VMS ready: %+v",stats)
    }
}
func TestV3CaptureDiagnosticNormalMovingPicture(t *testing.T) {
    img:=v3SyntheticCapture(960,540,func(x,y int)(byte,byte,byte){
        return byte(x*255/960),byte(y*255/540),byte((x+y)%256)
    })
    stats:=v3CapturePixelStats(img,960,540)
    if stats.Samples!=768 || stats.PossiblyBlank || stats.DynamicRange<50 {
        t.Fatalf("ordinary multicolor frame must be seen: %+v",stats)
    }
}
func TestV3CaptureDiagnosticStaticLightDoesNotMeanBroken(t *testing.T) {
    img:=v3SyntheticCapture(300,200,func(x,y int)(byte,byte,byte){return 190,190,190})
    stats:=v3CapturePixelStats(img,300,200)
    if stats.PossiblyBlank || !stats.NearUniform || stats.BlackPercent!=0 {
        t.Fatalf("static gray camera must not be marked blank: %+v",stats)
    }
}
func TestV3CaptureDiagnosticCannotReadIncompletePixels(t *testing.T) {
    if got:=v3CapturePixelStats(make([]byte,50),1920,1080);got.Samples!=0 {
        t.Fatalf("corrupt frame cannot be analyzed: %+v",got)
    }
    if got:=v3CapturePixelStats(nil,0,1080);got.Samples!=0 {
        t.Fatalf("invalid size cannot be analyzed: %+v",got)
    }
}
