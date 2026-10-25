package main

import (
 "math"
 "time"
)

type scanRegion struct{x,y,w,h int}
type regionCache struct{found []Rect; updated time.Time}

// Kept for compatibility with older tests, but the detector uses the configurable grid below.
func zones(w,h int)[5]scanRegion{
 x,y:=w/2,h/2
 return [5]scanRegion{{0,0,w,h},{0,0,x,y},{x,0,w-x,y},{0,y,x,h-y},{x,y,w-x,h-y}}
}

// A full-frame check plus tiled crops with 12% edge overlap. Overlap prevents
// subjects standing between video tiles from being sliced into two half-people.
// Profile 1=Eco (2x2), 2=Balanced (3x3), 3=CCTV detail (4x4).
func maxScan(a,b int)int{if a>b{return a};return b}
func minScan(a,b int)int{if a<b{return a};return b}
func regionsForProfile(w,h int,profile int32)[]scanRegion{
 if w<1||h<1{return nil}
 // A camera expanded to full screen must not be chopped into 16 AI
 // crops: that splits the human silhouette and defeats detection.
 if profile==0{return []scanRegion{{0,0,w,h}}}
 grid:=3
 if profile==1{grid=2}
 if profile==3{grid=4}
 regs:=make([]scanRegion,0,grid*grid+1)
 regs=append(regs,scanRegion{0,0,w,h})
 for row:=0;row<grid;row++{
  y1:=row*h/grid
  y2:=(row+1)*h/grid
  for col:=0;col<grid;col++{
   x1:=col*w/grid
   x2:=(col+1)*w/grid
   padX:=maxScan(8,(x2-x1)*12/100)
   padY:=maxScan(8,(y2-y1)*12/100)
   x1=maxScan(0,x1-padX);y1=maxScan(0,y1-padY)
   x2=minScan(w,x2+padX);y2=minScan(h,y2+padY)
   regs=append(regs,scanRegion{x1,y1,x2-x1,y2-y1})
  }
 }
 return regs
}

// Fullscreen precision experiment: keep the original whole-camera 640x640
// neural scan, and additionally inspect four overlapping zoomed quadrants.
// The extra passes can recover distant people that downscaling the entire
// original 720p frame makes too small. This does not add pixels absent from RVi.
func v3FullDetailRegions(w,h int) []scanRegion {
    if w<64||h<64{return nil}
    regions:=[]scanRegion{{0,0,w,h}}
    xpad,ypad:=w/15,h/15
    for row:=0;row<2;row++{
        for col:=0;col<2;col++ {
            x1,x2:=col*w/2,(col+1)*w/2
            y1,y2:=row*h/2,(row+1)*h/2
            if x1>xpad{x1-=xpad}else{x1=0}
            if y1>ypad{y1-=ypad}else{y1=0}
            if x2+xpad<w{x2+=xpad}else{x2=w}
            if y2+ypad<h{y2+=ypad}else{y2=h}
            regions=append(regions,scanRegion{x1,y1,x2-x1,y2-y1})
        }
    }
    return regions
}

// copyRegionBGRA snapshots only one selected camera crop, not the full
// desktop. The 640x640 conversion runs in the AI goroutine after this copy.
// Each row is copied before BitBlt may overwrite the shared source DIB.
func copyRegionBGRA(src []byte,fullW,fullH int,r scanRegion)[]byte{
 if fullW<1||fullH<1||r.w<1||r.h<1||r.x<0||r.y<0||
  r.x+r.w>fullW||r.y+r.h>fullH||len(src)<fullW*fullH*4{return nil}
 stride:=r.w*4
 crop:=make([]byte,stride*r.h)
 for y:=0;y<r.h;y++{
  start:=((r.y+y)*fullW+r.x)*4
  copy(crop[y*stride:(y+1)*stride],src[start:start+stride])
 }
 return crop
}

// Crop, rescale to 640x640, maintain aspect ratio, and letterbox.
// Bilinear sampling avoids aliasing distant 20-50 pixel CCTV silhouettes.
func prepareRegionFrame(src []byte,fullW,fullH int,reg scanRegion)([]float32,float64,float64,float64){
 return prepareRegionFrameEnhanced(src,fullW,fullH,reg,false)
}
func prepareRegionFrameEnhanced(src []byte,fullW,fullH int,reg scanRegion,enhance bool)([]float32,float64,float64,float64){
 n:=modelInputSize
 if fullW<1||fullH<1||reg.w<1||reg.h<1||reg.x<0||reg.y<0||
    reg.x+reg.w>fullW||reg.y+reg.h>fullH||len(src)<fullW*fullH*4{return nil,0,0,0}
 scale:=math.Min(float64(n)/float64(reg.w),float64(n)/float64(reg.h))
 dw,dh:=int(float64(reg.w)*scale+.5),int(float64(reg.h)*scale+.5)
 if dw>n{dw=n};if dh>n{dh=n}
 px,py:=(n-dw)/2,(n-dh)/2
 out:=make([]float32,3*n*n)
 for i:=range out{out[i]=114.0/255.0}
 // Contrast stretching only when explicitly requested and the region is
 // almost monochrome or dark. Do not distort normal color video by default.
 low,high:=0.0,255.0
 if enhance{low,high=regionContrastRange(src,fullW,reg)}
 gain:=255.0/math.Max(65.0,high-low)
 for y:=0;y<dh;y++{
  fy:=(float64(y)+0.5)/scale-0.5
  if fy<0{fy=0};if fy>float64(reg.h-1){fy=float64(reg.h-1)}
  sy:=int(fy);fy-=float64(sy)
  sy2:=minScan(sy+1,reg.h-1)
  for x:=0;x<dw;x++{
   fx:=(float64(x)+0.5)/scale-0.5
   if fx<0{fx=0};if fx>float64(reg.w-1){fx=float64(reg.w-1)}
   sx:=int(fx);fx-=float64(sx)
   sx2:=minScan(sx+1,reg.w-1)
   i00:=((reg.y+sy)*fullW+reg.x+sx)*4
   i10:=((reg.y+sy)*fullW+reg.x+sx2)*4
   i01:=((reg.y+sy2)*fullW+reg.x+sx)*4
   i11:=((reg.y+sy2)*fullW+reg.x+sx2)*4
   p:=(y+py)*n+x+px
   for c:=0;c<3;c++{
    channel:=2-c
    a:=float64(src[i00+channel])*(1-fx)+float64(src[i10+channel])*fx
    b:=float64(src[i01+channel])*(1-fx)+float64(src[i11+channel])*fx
    v:=a*(1-fy)+b*fy
    if enhance {v=math.Max(0,math.Min(255,(v-low)*gain))}
    out[c*n*n+p]=float32(v/255.0)
   }
  }
 }
 return out,scale,float64(px),float64(py)
}

func regionContrastRange(src []byte,w int,reg scanRegion)(float64,float64){
 var hist [256]int
 count:=0
 for y:=reg.y;y<reg.y+reg.h;y+=maxScan(1,reg.h/50){
  for x:=reg.x;x<reg.x+reg.w;x+=maxScan(1,reg.w/50){
   p:=(y*w+x)*4
   lum:=(int(src[p])*11+int(src[p+1])*59+int(src[p+2])*30)/100
   hist[lum]++;count++
  }
 }
 if count==0{return 0,255}
 lowLimit,highLimit:=count*5/100,count*95/100
 acc:=0; low,high:=0,255
 for i,v:=range hist{acc+=v;if acc>=lowLimit{low=i;break}}
 acc=0
 for i,v:=range hist{acc+=v;if acc>=highLimit{high=i;break}}
 // If normal exposure, leave input alone. Otherwise avoid extreme over-gain.
 if low<32&&high>215{return 0,255}
 return math.Max(0,float64(low)-12),math.Min(255,float64(high)+12)
}
func combineCacheList(c []regionCache,now time.Time,maxAge time.Duration)[]Rect{
 all:=make([]Rect,0,100)
 for _,item:=range c{
  if !item.updated.IsZero()&&now.Sub(item.updated)<=maxAge{all=append(all,item.found...)}
 }
 return suppressRects(all,.48,120)
}
func combineCaches(c [5]regionCache,now time.Time)[]Rect{
 return combineCacheList(c[:],now,12*time.Second)
}
