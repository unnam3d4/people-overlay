package main

import("testing";"time")
func TestZones(t *testing.T){
 for _,dim:=range [][2]int{{1920,1080},{1366,768},{1921,1081}}{
  z:=zones(dim[0],dim[1])
  if z[0].w!=dim[0]||z[0].h!=dim[1]{t.Fatal("full zone")}
  total:=0
  for i:=1;i<5;i++{total+=z[i].w*z[i].h}
  if total!=dim[0]*dim[1]{t.Fatal("coverage")}
 }
}
func TestCacheExpiry(t *testing.T){
 var c [5]regionCache
 c[0]=regionCache{[]Rect{{0,0,12,20,.8}},time.Now().Add(-15*time.Second)}
 if len(combineCaches(c,time.Now()))!=0{t.Fatal("stale frame")}
}
