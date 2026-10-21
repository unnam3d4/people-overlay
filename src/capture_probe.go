package main

// sampleCaptureDifference gives a cheap diagnostic of what the screen-grabber
// *actually* sees, not what is displayed through DirectDraw/D3D overlays.
// It samples a 36x24 grid, far cheaper than comparing two 1080p snapshots.
func sampleCaptureDifference(a,b []byte,w,h int)int{
 if w<=0||h<=0||len(a)<w*h*4||len(b)<w*h*4{return 0}
 const nx,ny=36,24
 changed,total:=0,0
 for y:=0;y<ny;y++{
  sy:=(2*y+1)*h/(2*ny)
  if sy>=h{sy=h-1}
  for x:=0;x<nx;x++{
   sx:=(2*x+1)*w/(2*nx)
   if sx>=w{sx=w-1}
   index:=(sy*w+sx)*4
   d:=0
   for c:=0;c<3;c++{
    v:=int(a[index+c])-int(b[index+c])
    if v<0{v=-v}
    d+=v
   }
   if d>=38{changed++}
   total++
  }
 }
 if total==0{return 0}
 return changed*100/total
}
