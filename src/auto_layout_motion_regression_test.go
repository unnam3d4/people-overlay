package main

import (
    "testing"
    "time"
)

func v3OffsetLayoutFixture(w,h,cols,left,top,right,bottom int)[]byte{
    p:=make([]byte,w*h*4)
    for y:=0;y<h;y++{
        for x:=0;x<w;x++{
            v:=uint8(28)
            if x>=left&&x<right&&y>=top&&y<bottom{
                c:=(x-left)*cols/(right-left)
                r:=(y-top)*cols/(bottom-top)
                v=uint8(38+((c*47+r*79)%135))
                // Simulate camera separators several pixels wide. No
                // assumption that a 4x4 pane starts at desktop (0,0).
                if ((x-left)*cols)%(right-left)<3*cols ||
                   ((y-top)*cols)%(bottom-top)<3*cols {v=8}
            }
            at:=(y*w+x)*4
            p[at],p[at+1],p[at+2],p[at+3]=v,v,v,255
        }
    }
    return p
}

func TestV3AutoLayoutWindowedGridAfterSidebar(t *testing.T){
    const w,h=960,720
    for _,c:=range []struct{cols,left,top,right,bottom int;expected int32}{
        {2,134,68,950,710,1},
        {3,145,72,950,708,2},
        {4,150,75,950,715,3},
    }{
        p:=v3OffsetLayoutFixture(w,h,c.cols,c.left,c.top,c.right,c.bottom)
        if got:=v3RecognizeLayout(p,w,h);got!=c.expected {
            t.Fatalf("camera grid %dx%d inside viewport (%d,%d)-(%d,%d) misdetected: got profile %d want %d",c.cols,c.cols,c.left,c.top,c.right,c.bottom,got,c.expected)
        }
    }
}

func TestV3AutoModeRequiresStableEvidenceAndDwell(t *testing.T){
    for _,tc:=range []struct{votes int;age time.Duration;from,to int32;want bool}{
        {2,6*time.Second,0,3,false},
        {3,2*time.Second,0,3,false},
        {3,4*time.Second,0,3,true},
        {4,4*time.Second,3,0,false},
        {5,4*time.Second,3,0,true},
        {5,5*time.Second,3,3,false},
    }{
        got:=v3AutoLayoutSwitch(tc.to,tc.from,tc.votes,tc.age)
        if got!=tc.want {
            t.Errorf("layout transition from %d to %d votes=%d age=%s got=%v want=%v",tc.from,tc.to,tc.votes,tc.age,got,tc.want)
        }
    }
}

func TestMovingPersonOpticalFlowFixDoesNotInventMotion(t *testing.T){
    // The existing independent sparse-person-flow tests already cover
    // motion of a filmed human, uniform duplicate frames and blank arrivals.
    // This new test checks production 'step' actually uses the estimator
    // when the coarse block search reports no reliable displacement.
    now:=time.Unix(700200,0)
    var tr personTracker
    tr.enableConfirmation()
    tr.maxAIAge=8*time.Second
    p,w,h:=smallPhotoOnTile(t,75,true)
    tr.step(p,w,h,now)
    box:=Rect{X:75,Y:44,W:18,H:59,Score:.9}
    tr.observePeopleFromCapture([]Rect{box},nil,p,scanRegion{0,0,w,h},now,now)
    initial:=tr.views(now)
    if len(initial)!=1{t.Fatalf("no initial person: %+v",initial)}
    moved,_,_:=smallPhotoOnTile(t,81,true)
    tr.step(moved,w,h,now.Add(150*time.Millisecond))
    final:=tr.views(now.Add(150*time.Millisecond))
    if len(final)!=1||final[0].Box.X<78 {
        t.Fatalf("corners stayed behind filmed pedestrian after 6px move: %+v",final)
    }
}
