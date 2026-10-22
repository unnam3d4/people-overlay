package main

import (
    "testing"
    "time"
)

func TestV3GridColumnsAllLayouts(t *testing.T) {
    for _,tc:=range []struct{profile int32;cols int}{
        {0,0},{1,2},{2,3},{3,4},{-1,0},
    } {
        if got:=v3GridColumns(tc.profile);got!=tc.cols {
            t.Fatalf("profile=%d got %d, want %d",tc.profile,got,tc.cols)
        }
    }
}

func TestV3CameraOwnerStopsNeighbourCropBleed(t *testing.T) {
    for _,tc:=range []struct{profile int32;cols int}{
        {1,2},{2,3},{3,4},
    }{
        cols:=tc.cols
        width,height:=960,720
        for row:=0;row<cols;row++{
            for col:=0;col<cols;col++{
                tile:=row*cols+col+1
                a:=Rect{X:float64(col*width/cols+width/(3*cols)),
                    Y:float64(row*height/cols+height/(3*cols)),W:18,H:28,Score:.9}
                if owner:=v3OwnerTile(a,width,height,cols);owner!=tile {
                    t.Fatalf("profile %d, tile %d attributed to %d",tc.profile,tile,owner)
                }
                // All other cameras are outside this actual camera even
                // though a YOLO crop padded over its border may include it.
                other:=Rect{X:a.X+float64(width/cols),Y:a.Y,W:18,H:28}
                if col==cols-1 {other.X=a.X-float64(width/cols)}
                kept:=v3KeepCameraDetections([]Rect{a,other},tile,width,height,tc.profile)
                if len(kept)!=1 || kept[0]!=a {
                    t.Fatalf("profile %d tile %d falsely attributed adjacent person: %+v",tc.profile,tile,kept)
                }
                if full:=v3KeepCameraDetections([]Rect{a,other},0,width,height,tc.profile);len(full)!=2 {
                    t.Fatalf("full-screen scan must not filter distinct people, profile %d",tc.profile)
                }
            }
        }
    }
}

func TestV3CrossCameraBoxesNeverMergeIDs(t *testing.T) {
    now:=time.Unix(1700000000,0)
    tkr:=personTracker{}
    tkr.layout(960,720,2)
    boxA:=Rect{X:410,Y:100,W:100,H:120,Score:.9}
    boxB:=Rect{X:466,Y:100,W:100,H:120,Score:.9}
    if tkr.tileOf(boxA)==tkr.tileOf(boxB) {
        t.Fatal("test must straddle independent camera boundary")
    }
    a:=tkr.assignDetections([]Rect{boxA},now)
    b:=tkr.assignDetections([]Rect{boxB},now.Add(time.Second))
    if len(a)!=1||len(b)!=1||a[0]==b[0]||a[0]<=0||b[0]<=0 {
        t.Fatalf("two camera sources should never reuse one person ID: first=%v next=%v",a,b)
    }
}
