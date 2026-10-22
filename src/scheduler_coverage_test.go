package main

import (
    "testing"
    "time"
)

// On a busy 4x4 RVi mosaic, fresh motion on previously unseen camera tiles
// must compete with known people rather than suffer 10+ second starvation.
func TestV3ColdStartCCTVDiscovery(t *testing.T) {
    now:=time.Unix(10000,0)
    regions:=regionsForProfile(960,540,3)
    var scheduler scanScheduler
    scheduler.reset(len(regions),960,540,3)
    if len(regions)!=17 {t.Fatalf("wrong tile count: %d",len(regions))}
    for i:=1;i<len(regions);i++ {scheduler.coreActivity[i]=.05}
    distinct:=make(map[int]bool)
    for n:=0;n<12;n++ {
        selected:=scheduler.ensureFirstCoverage(3,regions,1)
        if selected<1 || selected>16 {t.Fatalf("invalid camera index %d",selected)}
        distinct[selected]=true
        scheduler.markScanned(selected,now.Add(time.Duration(n)*time.Second),false)
    }
    if len(distinct)<8 {t.Fatalf("new moving camera tiles were starved: %d seen",len(distinct))}
}

// When many camera tiles contain no movement and an active human is already
// being tracked, unseen *quiet* cameras must not monopolize inference.
func TestV3CCTVQuietBackgroundPreservesTrackingQuota(t *testing.T){
    regions:=regionsForProfile(960,540,3)
    var scheduler scanScheduler
    scheduler.reset(len(regions),960,540,3)
    now:=time.Unix(10000,0)
    scheduler.markScanned(1,now,true)
    kept:=0
    for n:=0;n<12;n++ {
        got:=scheduler.ensureFirstCoverage(3,regions,1)
        if got==1 {kept++}
        scheduler.markScanned(got,now.Add(time.Duration(n)*time.Second),false)
    }
    if kept<8 {t.Fatalf("quiet camera enumeration stole %d of 12 tracked-person scans",12-kept)}
}

func TestV3NoCoverageOverrideForFullscreen(t *testing.T) {
    var scheduler scanScheduler
    regions:=regionsForProfile(960,540,3)
    scheduler.reset(len(regions),960,540,3)
    if got:=scheduler.ensureFirstCoverage(0,regions,0);got!=0{
        t.Fatalf("fullscreen profile illegally switched camera to %d",got)
    }
}
