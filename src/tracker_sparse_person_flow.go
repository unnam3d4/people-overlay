package main

// Lightweight consensus motion estimation from *multiple* high-contrast
// features within the AI box. This is sparse local patch flow (SSD/SAD), NOT
// OpenCV or full pyramidal Lucas-Kanade. It uses grayscale downscaled 2x,
// avoids dependencies and returns a shift only with multiple consistent
// independent patches. The tracker still requires AI silhouette checks.
// Per-person bounded work: up to 16 patches x 13x13 shifts x 9 bytes.
import "math"

type flowPt struct {x,y int; quality int}
type flowVec struct {x,y int; error int}

func sparsePersonFlow(prev,cur []byte,w,h int,b Rect)(float64,float64,bool){
 if len(prev)!=w*h||len(cur)!=w*h||w<30||h<30||b.W<15||b.H<24{return 0,0,false}
 sx,sy:=float64(trackingDownscale),float64(trackingDownscale)
 left:=int(math.Round((b.X+b.W*.15)/sx))
 right:=int(math.Round((b.X+b.W*.85)/sx))
 top:=int(math.Round((b.Y+b.H*.13)/sy))
 bottom:=int(math.Round((b.Y+b.H*.88)/sy))
 if left<9||top<9||right>=w-9||bottom>=h-9||right-left<5||bottom-top<10{return 0,0,false}
 pts:=make([]flowPt,0,16)
 for y:=top+1;y<bottom;y+=2 {
  for x:=left+1;x<right;x+=2 {
   a:=y*w+x
   ix:=absTrack(int(prev[a+1])-int(prev[a-1]))
   iy:=absTrack(int(prev[a+w])-int(prev[a-w]))
   quality:=ix+iy
   if quality<38 {continue}
   // Prefer high-contrast corners; don't place all points on one textured
   // wall or straight vertical door edge.
   if ix<10||iy<10{continue}
   pos:=len(pts)
   for i:=0;i<len(pts);i++{
    if absTrack(pts[i].x-x)<=3&&absTrack(pts[i].y-y)<=3{
      pos=i;break
    }
   }
   if pos<len(pts){
    if quality>pts[pos].quality{pts[pos]=flowPt{x,y,quality}}
   }else if len(pts)<16{
    pts=append(pts,flowPt{x,y,quality})
   }else{
    low:=0
    for i:=1;i<len(pts);i++{if pts[i].quality<pts[low].quality{low=i}}
    if quality>pts[low].quality{pts[low]=flowPt{x,y,quality}}
   }
  }
 }
 if len(pts)<4{return 0,0,false}
 vectors:=make([]flowVec,0,len(pts))
 for _,p:=range pts{
  best,bx,by:=999999,0,0
  for dy:=-6;dy<=6;dy++{
   for dx:=-6;dx<=6;dx++{
    score:=0
    for py:=-1;py<=1;py++{
     for px:=-1;px<=1;px++{
      o:=(p.y+py)*w+p.x+px
      c:=(p.y+dy+py)*w+p.x+dx+px
      score+=absTrack(int(prev[o])-int(cur[c]))
     }
    }
    // Small preference for stable, local motion; never create artificial
    // velocity when the previous image is identical to the current image.
    score+=absTrack(dx)+absTrack(dy)
    if score<best{best,bx,by=score,dx,dy}
   }
  }
  if best>9*21{continue}
  vectors=append(vectors,flowVec{bx,by,best})
 }
 if len(vectors)<4{return 0,0,false}
 // Robust mode of sparse displacement vectors. Outliers (shadows, waving
 // foliage, reflections, occluders) cannot dictate a new person location.
 bestInliers,bestDX,bestDY:=0,0,0
 for _,c:=range vectors{
  inliers:=0
  for _,v:=range vectors{
   if absTrack(v.x-c.x)<=1&&absTrack(v.y-c.y)<=1{inliers++}
  }
  if inliers>bestInliers{bestInliers,bestDX,bestDY=inliers,c.x,c.y}
 }
 if bestInliers<4||bestInliers*100<len(vectors)*58{return 0,0,false}
 xsum,ysum,count:=0,0,0
 for _,v:=range vectors{
  if absTrack(v.x-bestDX)<=1&&absTrack(v.y-bestDY)<=1{
   xsum+=v.x;ysum+=v.y;count++
  }
 }
 if count==0{return 0,0,false}
 return math.Round(float64(xsum)/float64(count))*sx,
        math.Round(float64(ysum)/float64(count))*sy,true
}
