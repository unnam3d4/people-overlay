package main

// Content-area aware grid recognition. Some VMS windows occupy only part of
// the screen: toolbar/title on top and a navigation tree at left. Exact
// desktop fractions (w/2,w/3,w/4) are NOT generally the video-pane seams.
// Work entirely on CPU in a few thousand pre-sampled grayscale edge positions.
// No app-specific hooks, OS dependencies, screenshots, or exported telemetry.
import "math"

func v3WindowedSeamScore(p []byte,w,h,grid int,vertical bool) float64 {
    length:=h
    if vertical {length=w}
    if grid<2||length/grid<48{return 0}
    // Precompute edge responses lazily. Hundreds of origin/pitch proposals
    // reuse the same responses, keeping this bounded on old i3/i5 CPUs.
    edges:=make([]float64,length)
    for i:=range edges{edges[i]=-1}
    strength:=func(x int)float64{
        if x<2||x>=length-2{return 0}
        if edges[x]<0{edges[x]=v3EdgeMean(p,w,h,x,vertical)}
        return edges[x]
    }
    stepOrigin:=length/110
    if stepOrigin<4 {stepOrigin=4}
    stepPitch:=length/240
    if stepPitch<3 {stepPitch=3}
    minPitch:=int(float64(length)/float64(grid)*.67)
    maxPitch:=length/grid+stepPitch
    maxOrigin:=length/4
    best:=0.
    for origin:=0;origin<=maxOrigin;origin+=stepOrigin {
        for pitch:=minPitch;pitch<=maxPitch;pitch+=stepPitch {
            if pitch<42||origin+grid*pitch>length+stepPitch {continue}
            matched:=0
            totalStrength:=0.
            for k:=1;k<grid;k++ {
                seam:=origin+k*pitch
                local:=0.
                for delta:=-7;delta<=7;delta+=2 {
                    if value:=strength(seam+delta);value>local{local=value}
                }
                // A camera-pane interior line should have much less energy
                // than the neighbouring border. The control is deliberately
                // inside this *candidate* pane, not at a fullscreen fraction.
                control:=strength(origin+k*pitch+pitch/2)
                if origin+k*pitch+pitch/2>=length-2{
                    control=strength(origin+k*pitch-pitch/2)
                }
                if local>=18 && local>=control*1.85+5 {
                    matched++
                    totalStrength+=math.Min(1,local/45)
                }
            }
            if matched==grid-1 {
                score:=float64(matched)/float64(grid-1)
                // Reject extremely weak seams that coincidentally align.
                if totalStrength/float64(grid-1)>.45 && score>best {
                    best=score
                }
            }
        }
    }
    return best
}

func v3AutoLayoutSwitch(candidate,active int32,votes int,sinceChange time.Duration)bool{
    if candidate==active{return false}
    // 4x4/3x3/2x2 must persist for three independent capture observations.
    // An undetected edge does NOT prove that a grid was closed: e.g. one
    // black camera or a maximized stream can remove all visible borders.
    required:=3
    if candidate==0 && active!=0 {required=5}
    if votes<required{return false}
    return sinceChange>=3500*time.Millisecond
}
