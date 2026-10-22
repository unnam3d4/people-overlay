package main

// A detector crop in a CCTV mosaic deliberately overlaps its neighbours
// to avoid chopping bodies at the edge. A person seen in that overlapping
// margin must be assigned to THEIR actual camera, never the crop being run.
// This is image geometry only: it never guesses identity or faces.
func v3GridColumns(profile int32) int {
    switch profile {
    case 1: return 2
    case 2: return 3
    case 3: return 4
    default: return 0
    }
}

func v3OwnerTile(box Rect, w,h,columns int) int {
    if columns<2 || w<=0 || h<=0 || box.W<=0 || box.H<=0 {return 0}
    cx:=int((box.X+box.W*.5)*float64(columns)/float64(w))
    cy:=int((box.Y+box.H*.5)*float64(columns)/float64(h))
    if cx<0{cx=0}
    if cy<0{cy=0}
    if cx>=columns{cx=columns-1}
    if cy>=columns{cy=columns-1}
    return cy*columns+cx+1
}

// The region index is 1..N² in row-major order. Full-frame AI pass (0)
// may legitimately see people in all feeds, and must never be filtered.
func v3KeepCameraDetections(boxes []Rect, regionIdx int, w,h int,profile int32) []Rect {
    columns:=v3GridColumns(profile)
    if columns==0 || regionIdx==0 {return boxes}
    filtered:=make([]Rect,0,len(boxes))
    for _,box:=range boxes {
        if v3OwnerTile(box,w,h,columns)==regionIdx {
            filtered=append(filtered,box)
        }
    }
    return filtered
}
