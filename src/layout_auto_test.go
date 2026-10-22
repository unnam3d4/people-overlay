package main

import "testing"

func v3TestSyntheticFrame(w,h,grid int) []byte {
    p:=make([]byte,w*h*4)
    for y:=0;y<h;y++ {
        for x:=0;x<w;x++ {
            v:=uint8((x*75/w+y*40/h+65)%190)
            if grid>0 {
                row:=y*grid/h
                col:=x*grid/w
                v=uint8(20+(row*63+col*57)%210)
                // Thin dark bars mimic one-pixel RVi grid cell borders.
                if x%(w/grid)==0 || y%(h/grid)==0 {v=0}
            }
            at:=(y*w+x)*4
            p[at]=v
            p[at+1]=v
            p[at+2]=v
            p[at+3]=255
        }
    }
    return p
}

func TestV3AutoLayoutFindsRViFullscreenGridAndSwitchback(t *testing.T) {
    const w,h=960,720
    for _,test:=range []struct{grid int; expected int32}{
        {grid:0,expected:0}, // single fullscreen CCTV scene
        {grid:2,expected:1},
        {grid:3,expected:2},
        {grid:4,expected:3},
    } {
        pixels:=v3TestSyntheticFrame(w,h,test.grid)
        result:=v3RecognizeLayout(pixels,w,h)
        if result!=test.expected {
            t.Errorf("grid=%d expected profile=%d got=%d",test.grid,test.expected,result)
        }
    }
    // A confusing CCTV feed with only a vertical door must not be classified
    // as a grid unless matching horizontal lines are present too.
    p:=v3TestSyntheticFrame(w,h,0)
    for y:=0;y<h;y++ {
        for _,x:=range []int{w/2-1,w/2,w/2+1} {
            at:=(y*w+x)*4
            p[at]=0;p[at+1]=0;p[at+2]=0
        }
    }
    if got:=v3RecognizeLayout(p,w,h);got!=0{
        t.Fatalf("single vertical line incorrectly means grid %d",got)
    }
}

func TestV3AutoLayoutRejectsInvalidPixels(t *testing.T) {
    if got:=v3RecognizeLayout(nil,960,720);got!=0{t.Fatal(got)}
    if got:=v3RecognizeLayout(make([]byte,100),960,720);got!=0{t.Fatal(got)}
    if got:=v3RecognizeLayout(make([]byte,80*80*4),80,80);got!=0{t.Fatal(got)}
}
