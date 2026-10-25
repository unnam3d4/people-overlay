package main
import ("testing";"time")
func TestRectangularGridAnyNumberCameras(t *testing.T){
 for _,tc:=range []struct{cols,rows int}{{1,1},{2,3},{4,4},{6,5},{10,5},{12,12}}{
    customGridColumns.Store(int32(tc.cols))
    customGridRows.Store(int32(tc.rows))
    r:=regionsForProfile(3600,2160,4)
    if len(r)!=tc.cols*tc.rows+1{t.Fatalf("%dx%d got %d regions",tc.cols,tc.rows,len(r))}
    for i:=1;i<len(r);i++{
        col,row:=(i-1)%tc.cols,(i-1)/tc.cols
        x,y:=float64((col*3600)/tc.cols+1000/tc.cols),float64((row*2160)/tc.rows+500/tc.rows)
        box:=Rect{X:x,Y:y,W:12,H:30,Score:.85}
        found:=v3KeepCameraDetections([]Rect{box},i,3600,2160,4)
        if len(found)!=1{t.Fatalf("%dx%d region %d lost its person",tc.cols,tc.rows,i)}
    }
 }
 customGridColumns.Store(4);customGridRows.Store(4)
}
func TestDetectorOnePositiveAndImmediateNegativeNoGhost(t *testing.T){
 var d detectorCache;d.reset(7,4)
 now:=time.Unix(400,0)
 r:=[]Rect{{X:22,Y:33,W:24,H:70,Score:.8}}
 if !d.replace(1,r,now)||len(d.views(now))!=1 {t.Fatal("first person not detected")}
 if d.replace(1,r,now.Add(time.Second)){t.Fatal("duplicate alarm")}
 if len(d.views(now.Add(3*time.Second)))!=0 {t.Fatal("expired marker")}
 if d.replace(1,nil,now.Add(3100*time.Millisecond))||len(d.views(now.Add(3100*time.Millisecond)))!=0{t.Fatal("negative kept a ghost")}
 if d.replace(1,r,now.Add(4*time.Second)){t.Fatal("90s alarm cooldown bypass")}
 d.reset(7,4)
 if len(d.views(now.Add(5*time.Second)))!=0{t.Fatal("reset failed")}
}
func TestDirectDetectorFairCoverageWithoutTracker(t *testing.T){
 customGridColumns.Store(10);customGridRows.Store(5)
 var s scanScheduler;s.reset(51,3600,2160,4)
 now:=time.Unix(900,0);visited:=map[int]bool{}
 for i:=0;i<50;i++{
    idx:=s.nextDetector(now)
    if idx<1||idx>50||visited[idx]{t.Fatalf("duplicate/starved camera %d at %d",idx,i)}
    visited[idx]=true;s.markScanned(idx,now,false)
    now=now.Add(150*time.Millisecond)
 }
 customGridColumns.Store(4);customGridRows.Store(4)
}
