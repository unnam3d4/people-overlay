package main

// Snapshot diagnostics are intentionally a warning, NOT a claim of VMS
// compatibility. A black picture can be normal for an IR/night scene, a
// disconnected stream, hardware overlay, or a copyright-protected surface.
// A still picture can be a legitimately quiet camera. Only a known moving
// reference in the captured frame can prove live capture.
// Sample 32x24 points instead of scanning a potentially 4K desktop.
type v3CaptureStats struct {
    Samples int
    MeanBrightness int
    BlackPercent int
    WhitePercent int
    DynamicRange int
    NearUniform bool
    PossiblyBlank bool
}

func v3CapturePixelStats(pixels []byte,w,h int) v3CaptureStats {
    var result v3CaptureStats
    if w<1 || h<1 || w>16384 || h>16384 || len(pixels)<w*h*4 {return result}
    const cols,rows=32,24
    minY,maxY:=256,-1
    sum,black,white:=0,0,0
    for row:=0;row<rows;row++ {
        y:=(2*row+1)*h/(2*rows)
        if y>=h {y=h-1}
        for col:=0;col<cols;col++ {
            x:=(2*col+1)*w/(2*cols)
            if x>=w {x=w-1}
            offset:=(y*w+x)*4
            b,g,r:=int(pixels[offset]),int(pixels[offset+1]),int(pixels[offset+2])
            yvalue:=(11*b+59*g+30*r)/100
            if yvalue<minY {minY=yvalue}
            if yvalue>maxY {maxY=yvalue}
            sum+=yvalue
            if yvalue<=14 {black++}
            if yvalue>=245 {white++}
            result.Samples++
        }
    }
    if result.Samples==0{return result}
    result.MeanBrightness=sum/result.Samples
    result.BlackPercent=black*100/result.Samples
    result.WhitePercent=white*100/result.Samples
    result.DynamicRange=maxY-minY
    result.NearUniform=result.DynamicRange<=8
    // Black video on screen can result from a GPU-only rendering path,
    // but this is NEVER definitive evidence of a capture failure.
    result.PossiblyBlank=result.BlackPercent>=96 ||
        (result.NearUniform && result.MeanBrightness<20)
    return result
}
