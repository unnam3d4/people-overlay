package main

import (
 "math"
 "sort"
 "time"
)

// TrackView is an independently tracked person. ID is session-local, not a
// biometric identity and not consistent across program restarts or cameras.
type TrackView struct {
 Box Rect
 ID int
}

type personTrack struct {
 Box Rect
 ID int
 LastAI time.Time
 LastMotion time.Time
 Tile int
}

type personTracker struct {
 people []personTrack
 nextID int
 prevGray []byte
 grayW,grayH int
 screenW,screenH int
 columns int
}

const trackingDownscale=4
const maxTrackedPeople=48

func (t *personTracker)layout(w,h int,cols int){
 t.screenW,t.screenH,t.columns=w,h,cols
}
func (t *personTracker)tileOf(r Rect)int{
 if t.screenW<1||t.screenH<1||t.columns<2{return 0}
 x:=int((r.X+r.W*.5)*float64(t.columns)/float64(t.screenW))
 y:=int((r.Y+r.H*.5)*float64(t.columns)/float64(t.screenH))
 if x<0{x=0};if y<0{y=0}
 if x>=t.columns{x=t.columns-1};if y>=t.columns{y=t.columns-1}
 return y*t.columns+x+1
}
func (t *personTracker)reset(){
 t.people=nil
 t.prevGray=nil
 t.grayW,t.grayH=0,0
 // nextID deliberately remains increasing during the session.
}

func grayscaleQuarter(pixels []byte,w,h int)([]byte,int,int){
 if w<1||h<1||len(pixels)<w*h*4{return nil,0,0}
 dw,dh:=(w+3)/trackingDownscale,(h+3)/trackingDownscale
 g:=make([]byte,dw*dh)
 for y:=0;y<dh;y++{
  sy:=minTrack(h-1,y*trackingDownscale+2)
  for x:=0;x<dw;x++{
   sx:=minTrack(w-1,x*trackingDownscale+2)
   p:=(sy*w+sx)*4
   g[y*dw+x]=uint8((int(pixels[p])*11+int(pixels[p+1])*59+int(pixels[p+2])*30)/100)
  }
 }
 return g,dw,dh
}

func minTrack(a,b int)int{if a<b{return a};return b}
func maxTrack(a,b int)int{if a>b{return a};return b}

// motionShift finds translation of a person's central body patch between two
// screen captures using 7x9 grayscale samples and local SAD block matching.
// It does not create new people: only actual neural detections can start tracks.
func motionShift(previous,current []byte,w,h int,box Rect)(float64,float64,bool){
 if len(previous)!=w*h||len(current)!=w*h||w<1||h<1{return 0,0,false}
 // Tiny objects and cropped people cannot be tracked reliably with 1/4
 // resolution; they wait for the next AI detection instead of drifting.
 if box.W<28||box.H<32{return 0,0,false}
 cx:=(box.X+box.W*.5)/trackingDownscale
 cy:=(box.Y+box.H*.5)/trackingDownscale
 hw:=box.W*.30/trackingDownscale
 hh:=box.H*.32/trackingDownscale
 if cx-hw<6.5||cy-hh<6.5||cx+hw>=float64(w-7)||cy+hh>=float64(h-7){return 0,0,false}
 var xs,ys [63]int
 var values [63]byte
 k,lo,hi:=0,255,0
 for iy:=0;iy<9;iy++{
  y:=int(math.Round(cy-hh+float64(iy)*2*hh/8))
  for ix:=0;ix<7;ix++{
   x:=int(math.Round(cx-hw+float64(ix)*2*hw/6))
   if x<7||x>=w-7||y<7||y>=h-7{return 0,0,false}
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
 zero:=1e9
 // 13x13 quarter-resolution search: up to +/-24 screen pixels per
 // lightweight frame. No CPU-heavy optical-flow network required.
 for dy:=-6;dy<=6;dy++{
  for dx:=-6;dx<=6;dx++{
   diff:=0
   for i:=0;i<63;i++{
    a:=int(values[i])
    b:=int(current[(ys[i]+dy)*w+xs[i]+dx])
    if a>b{diff+=a-b}else{diff+=b-a}
   }
   avg:=float64(diff)/63.0
   if dx==0&&dy==0{zero=avg}
   score:=avg+float64(absTrack(dx)+absTrack(dy))*.18
   if score<best{best,bestDX,bestDY=score,dx,dy}
  }
 }
 if best>32{return 0,0,false}
 if bestDX==0&&bestDY==0{return 0,0,true}
 // Do not "walk" a box into a similar nearby video tile/background when
 // there is no significant improvement over staying still.
 if zero-best<2.0{return 0,0,true}
 return float64(bestDX*trackingDownscale),float64(bestDY*trackingDownscale),true
}

func absTrack(v int)int{if v<0{return -v};return v}
func(t *personTracker)step(pixels []byte,w,h int,now time.Time){
 gray,gw,gh:=grayscaleQuarter(pixels,w,h)
 if gray==nil{return}
 if t.grayW!=gw||t.grayH!=gh||len(t.prevGray)!=len(gray){
  t.prevGray=gray;t.grayW,t.grayH=gw,gh
  return
 }
 for i:=range t.people{
  p:=&t.people[i]
  dx,dy,good:=motionShift(t.prevGray,gray,gw,gh,p.Box)
  if !good{continue}
  if math.Abs(dx)+math.Abs(dy)>=4 {
   next:=p.Box
   next.X=math.Max(0,math.Min(float64(w)-p.Box.W,p.Box.X+dx))
   next.Y=math.Max(0,math.Min(float64(h)-p.Box.H,p.Box.Y+dy))
   // In CCTV 4x4 mode, a track must not hop into another camera tile.
   if p.Tile==0||t.tileOf(next)==p.Tile{
    p.Box=next
    p.LastMotion=now
   }
  }
 }
 t.prevGray=gray
 t.prune(now)
}

// A fresh person detection corrects the lighter tracker, but IDs survive
// ordinary translations, partial overlaps, and brief low-confidence frames.
func (t *personTracker)observePeople(detections []Rect,at time.Time){
 if len(detections)==0{return}
 sort.Slice(detections,func(i,j int)bool{return detections[i].Score>detections[j].Score})
 used:=make([]bool,len(t.people))
 for _,d:=range detections{
  if d.W<=0||d.H<=0{continue}
  tile:=t.tileOf(d)
  // A second overlapping tiled crop must not invent a new person ID.
  duplicate:=false
  for _,p:=range t.people{
   if p.Tile==tile&&overlap(d,p.Box)>.68{duplicate=true;break}
  }
  // Do not skip overlap matching here: duplicate detection is still useful
  // for updating the existing track, just never spawn another track.
  best,bestScore:=-1,0.0
  for j,p:=range t.people{
   if used[j]||p.Tile!=tile{continue}
   iou:=overlap(d,p.Box)
   centerDX:=math.Abs((d.X+d.W*.5)-(p.Box.X+p.Box.W*.5))/math.Max(20,math.Max(d.W,p.Box.W))
   centerDY:=math.Abs((d.Y+d.H*.5)-(p.Box.Y+p.Box.H*.5))/math.Max(20,math.Max(d.H,p.Box.H))
   distance:=math.Hypot(centerDX,centerDY)
   if iou<.13&&distance>.42{continue}
   ratio:=math.Min(d.W,p.Box.W)/math.Max(d.W,p.Box.W)*
     math.Min(d.H,p.Box.H)/math.Max(d.H,p.Box.H)
   if ratio<.29{continue}
   score:=2*iou+(.65-distance)+ratio*.2
   if score>bestScore{bestScore,best=score,j}
  }
  if best>=0{
   p:=&t.people[best]
   // Detector anchors prevent template matching from drifting over time.
   p.Box.X=p.Box.X*.14+d.X*.86
   p.Box.Y=p.Box.Y*.14+d.Y*.86
   p.Box.W=p.Box.W*.20+d.W*.80
   p.Box.H=p.Box.H*.20+d.H*.80
   p.Box.Score=d.Score
   p.LastAI=at
   used[best]=true
  }else if !duplicate&&len(t.people)<maxTrackedPeople{
   t.nextID++
   t.people=append(t.people,personTrack{Box:d,ID:t.nextID,LastAI:at,Tile:tile})
   used=append(used,true)
  }
 }
 t.prune(at)
}
func (t *personTracker)prune(now time.Time){
 keep:=t.people[:0]
 for _,p:=range t.people{
  age:=now.Sub(p.LastAI)
  // A moving subject may be retained slightly longer when a 4x4 sweep is
  // slow. No stale phantom rectangles lasting 20+ seconds.
  maxAge:=5*time.Second
  if !p.LastMotion.IsZero()&&now.Sub(p.LastMotion)<1250*time.Millisecond{maxAge=9*time.Second}
  if age>=0&&age<=maxAge{keep=append(keep,p)}
 }
 t.people=keep
}
func(t *personTracker)views(now time.Time)[]TrackView{
 t.prune(now)
 views:=make([]TrackView,0,len(t.people))
 // IDs are stable; duplicate regions must never create duplicate labels.
 // Keep the most recently AI-confirmed overlapping track.
 ages:=make([]time.Time,0,len(t.people))
 for _,p:=range t.people{
  duplicate:=-1
  for i,v:=range views{
   if overlap(p.Box,v.Box)>.5{duplicate=i;break}
  }
  if duplicate>=0{
   if p.LastAI.After(ages[duplicate]){
    views[duplicate]=TrackView{Box:p.Box,ID:p.ID}
    ages[duplicate]=p.LastAI
   }
   continue
  }
  views=append(views,TrackView{Box:p.Box,ID:p.ID})
  ages=append(ages,p.LastAI)
 }
 return views
}

// Avoid redrawing the entire 1080p monitor when no marker moved.
func sameTrackViews(a,b []TrackView)bool{
 if len(a)!=len(b){return false}
 for i:=range a{
  if a[i].ID!=b[i].ID{return false}
  u,v:=a[i].Box,b[i].Box
  if math.Abs(u.X-v.X)>1.2||math.Abs(u.Y-v.Y)>1.2||math.Abs(u.W-v.W)>1.2||math.Abs(u.H-v.H)>1.2{return false}
 }
 return true
}
