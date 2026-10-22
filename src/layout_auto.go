package main

// Conservative frame-boundary detection for RVi layouts. Pixels are BGRA,
// exactly as captured by Windows BitBlt. A new layout is only accepted by
// the Windows engine after repeated observations; users can override it.
// This does NOT use faces, network access, RVi internals or machine learning.
import "math"

func v3GrayAt(p []byte,w,h,x,y int) float64 {
    if x<0 || x>=w || y<0 || y>=h {return 0}
    at:=(y*w+x)*4
    return float64(int(p[at])*11+int(p[at+1])*59+int(p[at+2])*30)/100
}

// Measure abrupt cross-pane discontinuities, rather than merely looking for
// black lines: some camera clients have one-pixel or no visible grid borders.
// Interior control lines are used to reject ordinary scene edges.
func v3EdgeMean(p []byte,w,h,pos int,vertical bool) float64 {
    total:=0.
    count:=0
    if vertical {
        if pos<2 || pos>=w-2{return 0}
        step:=h/80
        if step<2{step=2}
        for y:=h/18;y<h-h/18;y+=step {
            total+=math.Abs(v3GrayAt(p,w,h,pos-1,y)-v3GrayAt(p,w,h,pos+1,y))
            count++
        }
    }else{
        if pos<2 || pos>=h-2{return 0}
        step:=w/80
        if step<2{step=2}
        for x:=w/18;x<w-w/18;x+=step {
            total+=math.Abs(v3GrayAt(p,w,h,x,pos-1)-v3GrayAt(p,w,h,x,pos+1))
            count++
        }
    }
    if count==0{return 0}
    return total/float64(count)
}

func v3SeamScore(p []byte,w,h,grid int,vertical bool) float64 {
    length:=h
    if vertical{length=w}
    if grid<2||length/grid<40{return 0}
    matched:=0
    for k:=1;k<grid;k++ {
        seam:=k*length/grid
        maxEdge:=0.
        // Split boundaries may be offset by window borders or rounding.
        for dx:=-3;dx<=3;dx++ {
            v:=v3EdgeMean(p,w,h,seam+dx,vertical)
            if v>maxEdge{maxEdge=v}
        }
        // Reference line through the middle of an ordinary CCTV pane.
        ctl:=(k*length/grid)+length/(2*grid)
        if ctl>=length-3{ctl=(k-1)*length/grid+length/(2*grid)}
        control:=v3EdgeMean(p,w,h,ctl,vertical)
        if maxEdge>=17 && maxEdge>=control*1.75+4 {
            matched++
        }
    }
    return float64(matched)/float64(grid-1)
}

// Auto returns BOTH grid size and the estimated video viewport. A windowed
// RVi/SmartPSS mosaic must not be scanned as four equal desktop quarters:
// that cuts human silhouettes in the wrong places behind its sidebar.
func v3RecognizeLayoutGeometry(p []byte,w,h int)(int32,scanRegion){
    full:=scanRegion{0,0,w,h}
    if w<320||h<240||len(p)<w*h*4{return 0,full}
    for _,grid:=range []int{4,3,2}{
        vertical:=v3SeamScore(p,w,h,grid,true)
        horizontal:=v3SeamScore(p,w,h,grid,false)
        if vertical>=.85 && horizontal>=.85{
            return int32(grid-1),full
        }
        left,top:=0,0
        pitchX,pitchY:=w/grid,h/grid
        if vertical<.85 {
            var sx int
            vertical,left,sx=v3WindowedSeamGeometry(p,w,h,grid,true)
            pitchX=sx
        }
        if horizontal<.85 {
            var sy int
            horizontal,top,sy=v3WindowedSeamGeometry(p,w,h,grid,false)
            pitchY=sy
        }
        if vertical<.85||horizontal<.85||pitchX<48||pitchY<48{continue}
        // Bounded by the captured display. No viewport crop may leak into
        // the next monitor or exceed the source DIB.
        width,height:=grid*pitchX,grid*pitchY
        if left+width>w {width=w-left}
        if top+height>h {height=h-top}
        if width<w/3 || height<h/3 || left<0 || top<0 {continue}
        return int32(grid-1),scanRegion{left,top,width,height}
    }
    return 0,full
}

func v3RecognizeLayout(p []byte,w,h int) int32 {
    profile,_:=v3RecognizeLayoutGeometry(p,w,h)
    return profile
}
