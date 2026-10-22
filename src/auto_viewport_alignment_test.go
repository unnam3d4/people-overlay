package main

import "testing"

func TestAutoCameraViewportUsesActualVideoPaneNotMonitorFractions(t *testing.T){
 const w,h=960,720
 for _,c:=range []struct{cols,left,top,right,bottom int;mode int32}{
  {2,134,68,950,710,1},
  {3,145,72,950,708,2},
  {4,150,75,950,715,3},
 }{
  img:=v3OffsetLayoutFixture(w,h,c.cols,c.left,c.top,c.right,c.bottom)
  mode,viewport:=v3RecognizeLayoutGeometry(img,w,h)
  if mode!=c.mode {t.Fatalf("auto found profile %d want %d",mode,c.mode)}
  if absAuto(viewport.x-c.left)>18||absAuto(viewport.y-c.top)>18{
   t.Fatalf("%d grid: inferred viewport shifted: %+v wanted top left %d,%d",c.cols,viewport,c.left,c.top)
  }
  regs:=regionsForViewport(w,h,mode,viewport)
  if len(regs)!=1+c.cols*c.cols{
   t.Fatalf("%d grid: %d regions",c.cols,len(regs))
  }
  // First grid tile must start near the real viewer's left panel,
  // and not at desktop x=0.
  if regs[1].x<c.left-20 {
   t.Fatalf("%d-grid inference crop starts in navigation sidebar: %+v",c.cols,regs[1])
  }
  // Detector ownership and tracking identity agree about the same
  // physical camera, not the origin of the entire Windows desktop.
  const personW,personH=24,45
  sample:=Rect{X:float64(c.left+viewport.w/c.cols/2-personW/2),Y:float64(c.top+viewport.h/c.cols/2-personH/2),W:personW,H:personH,Score:.9}
  if tile:=v3OwnerTileViewport(sample,viewport,c.cols);tile!=1{
   t.Fatalf("%d-grid real first camera assigned tile %d",c.cols,tile)
  }
  var tr personTracker
  tr.layout(w,h,c.cols)
  tr.setViewport(viewport)
  if got:=tr.tileOf(sample);got!=1{t.Fatalf("%d-grid tracking owner was %d",c.cols,got)}
  found:=v3KeepCameraDetectionsViewport([]Rect{sample},1,c.cols,viewport)
  if len(found)!=1{t.Fatal("real first camera human discarded in auto")}
 }
}

func absAuto(x int)int{if x<0{return -x};return x}

func TestViewportDoesNotMatchPeopleInSidebarOrNeighbouringCamera(t *testing.T){
 viewport:=scanRegion{160,70,720,600}
 one:=Rect{X:177,Y:100,W:28,H:65}
 sidebar:=Rect{X:28,Y:100,W:28,H:65}
 next:=Rect{X:370,Y:100,W:28,H:65}
 if got:=v3OwnerTileViewport(sidebar,viewport,4);got!=0{
  t.Fatalf("sidebar erroneously assigned CCTV camera %d",got)
 }
 filtered:=v3KeepCameraDetectionsViewport([]Rect{one,sidebar,next},1,4,viewport)
 if len(filtered)!=1||filtered[0].X!=one.X{
  t.Fatalf("auto allowed detections outside original video pane: %+v",filtered)
 }
}

func TestManualFullscreenAndGridROIsUnchanged(t *testing.T){
 for _,mode:=range []int32{0,1,2,3}{
  a:=regionsForProfile(1920,1080,mode)
  b:=regionsForViewport(1920,1080,mode,scanRegion{0,0,1920,1080})
  if len(a)!=len(b){t.Fatalf("mode %d changed region count",mode)}
  for i:=range a{if a[i]!=b[i]{t.Fatalf("manual mode %d region %d altered",mode,i)}}
 }
}
