package main

import (
 "math"
 "sort"
 "time"
)

// ID means a track in this session only, never the person's identity.
type TrackView struct { Box Rect; ID int }

type personTrack struct {
 Box Rect
 ID int
 LastAI time.Time
 LastVisual time.Time
 LastMotion time.Time
 LastStep time.Time
 VelocityX,VelocityY float64
 Tile int
 AIMisses int
}

type personTracker struct {
 people []personTrack
 nextID int
 prevGray []byte
 grayW,grayH int
 screenW,screenH int
 columns int
 maxAIAge time.Duration
}

const trackingDownscale=2
const maxTrackedPeople=48

func (t *personTracker)layout(w,h,cols int) {
 t.screenW,t.screenH,t.columns=w,h,cols
}
func (t *personTracker)tileOf(r Rect) int {
 if t.screenW<1||t.screenH<1||t.columns<2{return 0}
 x:=int((r.X+r.W*.5)*float64(t.columns)/float64(t.screenW))
 y:=int((r.Y+r.H*.5)*float64(t.columns)/float64(t.screenH))
 if x<0{x=0};if y<0{y=0}
 if x>=t.columns{x=t.columns-1};if y>=t.columns{y=t.columns-1}
 return y*t.columns+x+1
}
func (t *personTracker)reset() {
 t.people=nil
 t.prevGray=nil
 t.grayW,t.grayH=0,0
}

func minTrack(a,b int)int {if a<b{return a};return b}

func grayscaleQuarter(pixels []byte,w,h int)([]byte,int,int) {
 if w<1||h<1||len(pixels)<w*h*4{return nil,0,0}
 dw,dh:=(w+trackingDownscale-1)/trackingDownscale,(h+trackingDownscale-1)/trackingDownscale
 g:=make([]byte,dw*dh)
 for y:=0;y<dh;y++ {
  sy:=minTrack(h-1,y*trackingDownscale+1)
  for x:=0;x<dw;x++ {
   sx:=minTrack(w-1,x*trackingDownscale+1)
   p:=(sy*w+sx)*4
   g[y*dw+x]=uint8((int(pixels[p])*11+int(pixels[p+1])*59+int(pixels[p+2])*30)/100)
  }
 }
 return g,dw,dh
}
func absTrack(v int)int {if v<0{return -v};return v}

// A cheap localized block matcher. 2px spatial resolution lets it follow
// 14-27px-wide silhouettes that the old 4px matcher could not handle.
// It never predicts a new person. If the template becomes different or
// untextured it rejects motion rather than allowing drift.
func motionShift(previous,current []byte,w,h int,box Rect)(float64,float64,bool) {
 if w<1||h<1||len(previous)!=w*h||len(current)!=w*h{return 0,0,false}
 if box.W<14||box.H<20{return 0,0,false}
 cx:=(box.X+box.W*.5)/trackingDownscale
 cy:=(box.Y+box.H*.5)/trackingDownscale
 hw:=math.Max(3.0,box.W*.28/trackingDownscale)
 hh:=math.Max(4.0,box.H*.30/trackingDownscale)
 if cx-hw<7||cy-hh<7||cx+hw>=float64(w-7)||cy+hh>=float64(h-7){return 0,0,false}
 var xs,ys [63]int
 var values [63]byte
 k,lo,hi:=0,255,0
 for iy:=0;iy<9;iy++{
  y:=int(math.Round(cy-hh+float64(iy)*2*hh/8))
  for ix:=0;ix<7;ix++{
   x:=int(math.Round(cx-hw+float64(ix)*2*hw/6))
   if x<6||x>=w-6||y<6||y>=h-6{return 0,0,false}
   xs[k],ys[k]=x,y
   v:=previous[y*w+x]
   values[k]=v
   if int(v)<lo{lo=int(v)}
   if int(v)>hi{hi=int(v)}
   k++
  }
 }
 if hi-lo<18{return 0,0,false}
 best,bestDX,bestDY:=1e9,0,0
 same:=1e9
 second:=1e9
 for dy:=-5;dy<=5;dy++{
  for dx:=-5;dx<=5;dx++{
   diff:=0
   for i:=0;i<63;i++{
    a,b:=int(values[i]),int(current[(ys[i]+dy)*w+xs[i]+dx])
    if a>b{diff+=a-b}else{diff+=b-a}
   }
   avg:=float64(diff)/63
   if dx==0&&dy==0{same=avg}
   score:=avg+float64(absTrack(dx)+absTrack(dy))*.10
   if score<best {
    second=best
    best,bestDX,bestDY=score,dx,dy
   }else if score<second {second=score}
  }
 }
 if best>25{return 0,0,false}
 if bestDX==0&&bestDY==0{return 0,0,true}
 // A repetitive background generates near-equal matches and otherwise
 // makes the corners hop between tiles. Prefer not moving over guessing.
 if same-best<3.5||second-best<.24{return 0,0,false}
 return float64(bestDX*trackingDownscale),float64(bestDY*trackingDownscale),true
}
// When clothing or the image is smooth, template matching has too little
// texture to infer motion. A stable patch still gives positive visual evidence
// of a stationary person (and will not be mistaken for a vanished silhouette).
func stationaryAppearance(previous,current []byte,w,h int,box Rect)bool{
 if w<1||h<1||len(previous)!=w*h||len(current)!=w*h{return false}
 cx:=(box.X+box.W*.5)/trackingDownscale
 cy:=(box.Y+box.H*.5)/trackingDownscale
 hw:=box.W*.34/trackingDownscale
 hh:=box.H*.35/trackingDownscale
 if cx-hw<0||cy-hh<0||cx+hw>=float64(w)||cy+hh>=float64(h){return false}
 total,large:=0,0
 for y:=0;y<7;y++{for x:=0;x<7;x++{
  px:=int(math.Round(cx-hw+2*hw*float64(x)/6))
  py:=int(math.Round(cy-hh+2*hh*float64(y)/6))
  diff:=absTrack(int(previous[py*w+px])-int(current[py*w+px]))
  total+=diff
  if diff>24{large++}
 }}
 return total<=49*9&&large<=5
}

func (t *personTracker)step(pixels []byte,w,h int,now time.Time) {
 if len(t.people)==0 {
  t.prevGray=nil
  return
 }
 gray,gw,gh:=grayscaleQuarter(pixels,w,h)
 if gray==nil{return}
 if t.grayW!=gw||t.grayH!=gh||len(t.prevGray)!=len(gray){
  t.prevGray=gray;t.grayW,t.grayH=gw,gh
  return
 }
 for i:=range t.people {
  p:=&t.people[i]
  dx,dy,good:=motionShift(t.prevGray,gray,gw,gh,p.Box)
  dt:=now.Sub(p.LastStep).Seconds()
  if dt<=0||dt>.4{dt=.115}
  p.LastStep=now
  if !good {
   // Low-texture CCTV clothing can remain perfectly still. Keep the
   // confirmed person's corners while their actual pixels remain stable.
   if stationaryAppearance(t.prevGray,gray,gw,gh,p.Box){
    p.LastVisual=now
    p.VelocityX=0;p.VelocityY=0
    continue
   }
   // Bridge at most 0.35s of weak or occluded video using recent velocity.
   // Never renew LastVisual here: lost tracks still expire promptly.
   if !p.LastMotion.IsZero()&&now.Sub(p.LastMotion)<350*time.Millisecond&&
     now.Sub(p.LastVisual)<350*time.Millisecond{
    dx=math.Max(-8,math.Min(8,p.VelocityX*dt*.5))
    dy=math.Max(-8,math.Min(8,p.VelocityY*dt*.5))
    p.VelocityX*=.78;p.VelocityY*=.78
   }else{p.VelocityX=0;p.VelocityY=0;continue}
  }else{
   p.LastVisual=now
   if dx!=0||dy!=0{
    velX:=math.Max(-160,math.Min(160,dx/dt))
    velY:=math.Max(-160,math.Min(160,dy/dt))
    p.VelocityX=.46*p.VelocityX+.54*velX
    p.VelocityY=.46*p.VelocityY+.54*velY
    p.LastMotion=now
   }else{p.VelocityX*=.5;p.VelocityY*=.5}
  }
  next:=p.Box
  next.X=math.Max(0,math.Min(float64(w)-next.W,next.X+dx))
  next.Y=math.Max(0,math.Min(float64(h)-next.H,next.Y+dy))
  // ROI boundaries may not align with the actual RVi video grid.
  p.Tile=t.tileOf(next)
  if dx!=0||dy!=0{p.Box=next}
 }
 t.prevGray=gray
 // Never merge IDs merely because two real people overlap on screen.
 t.prune(now)
}

// The same person can have boxes with different sizes in overlapping crops.
// The metric combines ordinary IoU and small-box containment.
func coveringFraction(a,b Rect)float64{
 x1,y1:=math.Max(a.X,b.X),math.Max(a.Y,b.Y)
 x2,y2:=math.Min(a.X+a.W,b.X+b.W),math.Min(a.Y+a.H,b.Y+b.H)
 if x2<=x1||y2<=y1{return 0}
 return (x2-x1)*(y2-y1)/math.Max(1,math.Min(a.W*a.H,b.W*b.H))
}
func centerSeparation(a,b Rect)float64{
 dx:=(a.X+a.W*.5-b.X-b.W*.5)/math.Max(14,math.Max(a.W,b.W))
 dy:=(a.Y+a.H*.5-b.Y-b.H*.5)/math.Max(14,math.Max(a.H,b.H))
 return math.Hypot(dx,dy)
}
func samePersonGeometry(a,b Rect)bool{
 if a.W<=0||b.W<=0||a.H<=0||b.H<=0{return false}
 center:=centerSeparation(a,b)
 iou:=overlap(a,b)
 if iou>=.52{return true}
 if center<=.25&&coveringFraction(a,b)>=.74{return true}
 return false
}

func (t *personTracker)observePeople(detections []Rect,at time.Time){
 if len(detections)==0{return}
 sort.SliceStable(detections,func(i,j int)bool{return detections[i].Score>detections[j].Score})
 // A single YOLO frame can contain duplicate near-identical proposals.
 // Keep two genuinely overlapping people unless almost the same silhouette.
 unique:=make([]Rect,0,len(detections))
 for _,d:=range detections{
  if d.W<6||d.H<9{continue}
  duplicate:=false
  for _,u:=range unique{if sameSilhouette(d,u){duplicate=true;break}}
  if !duplicate{unique=append(unique,d)}
 }
 t.assignDetections(unique,at)
 t.prune(at)
}

// Each camera is checked asynchronously. One missed YOLO detection can
// occur because of occlusion or poor video quality; two independent negative
// scans of the *same fully visible region* are enough to retire a ghost box.
// Crops touching the edge of the person's box are intentionally ignored.
func (t *personTracker)noteRegionScan(region scanRegion,now time.Time){
 left,top:=float64(region.x),float64(region.y)
 right,bottom:=float64(region.x+region.w),float64(region.y+region.h)
 for i:=range t.people{
  p:=&t.people[i]
  if !p.LastAI.Before(now)||p.Box.W<=0||p.Box.H<=0{continue}
  w:=math.Min(p.Box.X+p.Box.W,right)-math.Max(p.Box.X,left)
  h:=math.Min(p.Box.Y+p.Box.H,bottom)-math.Max(p.Box.Y,top)
  if w<=0||h<=0||w*h<.90*p.Box.W*p.Box.H{continue}
  p.AIMisses++
 }
 t.prune(now)
}

func (t *personTracker)prune(now time.Time){
 keep:=t.people[:0]
 for _,p:=range t.people{
  aiAge:=now.Sub(p.LastAI)
  visualAge:=now.Sub(p.LastVisual)
  if aiAge<0||visualAge<0{continue}
  if p.AIMisses>=2{continue}
  // Unverified boxes must disappear quickly rather than freeze for 9s.
  if visualAge>1200*time.Millisecond&&aiAge>1700*time.Millisecond{continue}
  // Even if the texture matcher claims a static match, force neural
  // revalidation eventually so background texture is not called a person.
  ttl:=t.maxAIAge
  if ttl<=0{ttl=8*time.Second}
  if aiAge>ttl{continue}
  keep=append(keep,p)
 }
 t.people=keep
}
func (t *personTracker)views(now time.Time)[]TrackView{
 t.prune(now)
 views:=make([]TrackView,0,len(t.people))
 for _,p:=range t.people{views=append(views,TrackView{Box:p.Box,ID:p.ID})}
 return views
}
func sameTrackViews(a,b []TrackView)bool{
 if len(a)!=len(b){return false}
 for i:=range a{
  if a[i].ID!=b[i].ID{return false}
  u,v:=a[i].Box,b[i].Box
  if math.Abs(u.X-v.X)>1.2||math.Abs(u.Y-v.Y)>1.2||math.Abs(u.W-v.W)>1.2||math.Abs(u.H-v.H)>1.2{return false}
 }
 return true
}
