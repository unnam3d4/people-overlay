//go:build windows

package main

import (
    "fmt"
    "runtime"
    "time"
)

// Each physical monitor owns an independent scheduler, video frame and tracker.
// One bounded inference worker is deliberately shared by ALL monitors on old CPUs.
// No network traffic or RVi integration is involved.
type v3Screen struct {
    rect desktopRect
    snap dib
    regions []scanRegion
    sch scanScheduler
    tracker personTracker
    alarm alarmGate
    epoch uint64
    profile int32
    candidateProfile int32
    candidateVotes int
    layoutAt time.Time
    captureFailures int
    // AI completed on this screen; dispatch does not count as detection.
    completed []time.Time
}

func v3AllRects(monitors []desktopMonitor) []desktopRect {
    if len(monitors)>4 {monitors=monitors[:4]}
    result:=make([]desktopRect,0,len(monitors))
    for _,m:=range monitors {
        if m.Rect.width()>=64 && m.Rect.height()>=64 {
            result=append(result,m.Rect)
        }
    }
    return result
}

// Weighted fair AI dispatch. An expanded CCTV feed receives three slots
// for each grid monitor slot, without completely starving any monitor.
// Service counts reflect actual queued neural jobs, not captured display FPS.
func v3SelectNextAI(screens []*v3Screen,served []int,after int) int {
    if len(screens)==0 || len(served)<len(screens){return 0}
    best:=-1
    bestScore:=0.
    for d:=1;d<=len(screens);d++{
        i:=(after+d)%len(screens)
        screen:=screens[i]
        if screen==nil||screen.captureFailures>=3{continue}
        weight:=1.
        if screen.profile==0{weight=3}
        score:=float64(served[i])/weight
        if best<0||score<bestScore {
            best,bestScore=i,score
        }
    }
    if best<0 {return (after+1)%len(screens)}
    return best
}

// A layered overlay occupies the union in virtual desktop coordinates.
// Source captures are per-monitor: never stretch a camera across two screens.
func v3UnionRect(rects []desktopRect) desktopRect {
    if len(rects)==0{return desktopRect{}}
    union:=rects[0]
    for _,r:=range rects[1:] {
        if r.Left<union.Left {union.Left=r.Left}
        if r.Top<union.Top {union.Top=r.Top}
        if r.Right>union.Right {union.Right=r.Right}
        if r.Bottom>union.Bottom {union.Bottom=r.Bottom}
    }
    return union
}

func v3SameRects(a,b []desktopRect) bool {
    if len(a)!=len(b){return false}
    for i:=range a {if a[i]!=b[i]{return false}}
    return true
}

func v3RelativeBox(b Rect, monitor, union desktopRect) Rect {
    b.X+=float64(monitor.Left-union.Left)
    b.Y+=float64(monitor.Top-union.Top)
    return b
}

func v3CreateScreens(screen uintptr, rects []desktopRect, profile int32, epoch uint64) ([]*v3Screen,error) {
    if profile==-1 {profile=0} // auto starts fullscreen until a grid is proven
    screens:=make([]*v3Screen,0,len(rects))
    for _,rect:=range rects {
        w,h:=rect.width(),rect.height()
        snap,err:=newDIB(screen,w,h)
        if err!=nil {
            for _,s:=range screens {s.snap.close()}
            return nil,err
        }
        s:=&v3Screen{rect:rect,snap:snap,epoch:epoch,profile:profile}
        s.regions=regionsForProfile(w,h,profile)
        s.completed=make([]time.Time,len(s.regions))
        s.sch.reset(len(s.regions),w,h,profile)
        s.tracker.enableConfirmation()
        s.tracker.layout(w,h,v3GridColumns(profile))
        screens=append(screens,s)
    }
    return screens,nil
}

// Rebuild camera regions and tracking only for the screen whose RVi layout
// changed. Local neural epochs reject obsolete boxes from old layouts.
func v3SetScreenProfile(s *v3Screen,profile int32,epoch uint64) {
    if profile<0||profile>3{return}
    s.profile=profile
    s.epoch=epoch
    s.tracker.reset()
    s.tracker.enableConfirmation()
    s.tracker.layout(s.snap.w,s.snap.h,v3GridColumns(profile))
    s.regions=regionsForProfile(s.snap.w,s.snap.h,profile)
    s.sch.reset(len(s.regions),s.snap.w,s.snap.h,profile)
    s.completed=make([]time.Time,len(s.regions))
    s.alarm.resetScene()
}

func v3Publish(screens []*v3Screen, union desktopRect, now time.Time, last *[]TrackView) {
    var views []TrackView
    pending,confirmed,lost:=0,0,0
    for i,s:=range screens {
        a,b,c:=s.tracker.statusCounts(now)
        pending+=a
        confirmed+=b
        lost+=c
        for _,v:=range s.tracker.views(now) {
            v.Box=v3RelativeBox(v.Box,s.rect,union)
            // Track IDs are independent per monitor. Encoded ID is display-local,
            // not identity recognition of a person across cameras.
            v.ID=(i+1)*1000000+v.ID
            views=append(views,v)
        }
    }
    boxes.mu.Lock()
    boxes.items=views
    boxes.updated=now
    boxes.mu.Unlock()
    tentativeCount.Store(int32(pending))
    confirmedCount.Store(int32(confirmed))
    lostCount.Store(int32(lost))
    trackedCount.Store(int32(len(views)))
    if !sameTrackViews(*last,views) {
        *last=append((*last)[:0],views...)
        version.Add(1)
    }
}

// Actual camera coverage rather than optimistic promised detection latency.
// A zero scan timestamp is an unseen camera; measure how long it has remained
// unseen since the scheduler started. Per-screen screen cuts reset only its
// timestamps, never claim that an absent inference has inspected a camera.
func v3CameraCoverage(screens []*v3Screen, now time.Time) (unseen int, oldest time.Duration) {
    for _, screen := range screens {
        if screen == nil {continue}
        start:=1
        if len(screen.completed)==1 {start=0}
        for i:=start;i<len(screen.completed);i++ {
            scanAt:=screen.completed[i]
            if scanAt.IsZero() {
                unseen++
                scanAt=screen.sch.firstDecision
            }
            if scanAt.IsZero() {continue}
            if age:=now.Sub(scanAt);age>oldest {
                oldest=age
            }
        }
    }
    if oldest<0 {oldest=0}
    return unseen,oldest
}

// Only apply a delayed neural result to the physical screen and scene
// generation it was captured from. Topology changes and hard scene cuts
// invalidate only their matching requests.
func v3AcceptReply(screens []*v3Screen, result inferenceReply) bool {
    if result.display<0 || result.display>=len(screens) {return false}
    s:=screens[result.display]
    return s!=nil && s.epoch==result.epoch
}

func v3CaptureCadence(profile int32, screens int, hasPeople, fast bool) time.Duration {
    cadence:=frameCadence(profile,hasPeople,fast)
    // Original v20 waits 300 ms on a quiet scene. With four monitors that
    // would sample each screen only every 1.2 seconds, before inference.
    // Bound aggregate per-screen capture latency without spinning at 60 FPS.
    if screens>1 && cadence>110*time.Millisecond {
        return 110*time.Millisecond
    }
    return cadence
}

func v3AIPause(profile int32,busy time.Duration,fullQuality bool) time.Duration {
    if profile==0 && fullQuality{return 0}
    return throttleFor(profile,busy)
}
func v3FrameWait(profile int32,monitors int,hasPeople,fast,fullQuality bool) time.Duration {
    if profile==0 && fullQuality{return 85*time.Millisecond}
    return v3CaptureCadence(profile,monitors,hasPeople,fast)
}

// v3Analyze replaces single-display analyze(), retaining production NCNN,
// timestamp-anchored detections, tracking checks and the anti-noise alarm.
func v3Analyze(detector *Detector) {
    runtime.LockOSThread()
    defer runtime.UnlockOSThread()
    screen,_,_:=pDC.Call(0)
    if screen==0 {logError(fmt.Errorf("v3 GetDC failed"));return}
    defer pReleaseDC.Call(0,screen)

    req:=make(chan inferenceRequest,1)
    replies:=make(chan inferenceReply,1)
    done:=make(chan struct{})
    go func(){defer close(done);inferenceWorker(detector,req,replies)}()
    defer func(){close(req);<-done}()

    var screens []*v3Screen
    defer func(){for _,s:=range screens{s.snap.close()}}()
    var lastRects []desktopRect
    var lastPublished []TrackView
    var lastGeneration uint64=^uint64(0)
    var epoch uint64
    var pending bool
    var nextInfer time.Time
    var lastBusy=time.Duration(350)*time.Millisecond
    var nextAI int
    var aiServed []int
    var turn int
    var lastMonitorPoll time.Time
    var chosen []desktopRect
    var union desktopRect
    var probeAt time.Time
    var probeFirst []byte
    var globalAlarmAt time.Time
    // Each screen can advance its scene generation independently.

    clear:=func(){
        boxes.mu.Lock()
        boxes.items=nil
        boxes.updated=time.Time{}
        boxes.mu.Unlock()
        lastPublished=nil
        trackedCount.Store(0)
        pendingCamera.Store(0)
        v3UnvisitedCameras.Store(0)
        v3WorstCameraAgeMS.Store(0)
        v3CapturePreviewReady.Store(false)
        tentativeCount.Store(0)
        confirmedCount.Store(0)
        lostCount.Store(0)
        version.Add(1)
    }

    for !stopped.Load() {
        if !enabled.Load() || testOnly.Load() {
            if len(screens)>0 {
                epoch++
                for _,s:=range screens{s.snap.close()}
                screens=nil
                lastRects=nil
                clear()
            }
            time.Sleep(120*time.Millisecond)
            continue
        }

        now:=time.Now()
        if now.Sub(lastMonitorPoll)>time.Second || len(chosen)==0 {
            chosen=v3AllRects(monitorList())
            lastMonitorPoll=now
        }
        generation:=configurationGeneration.Load()
        configuredProfile:=scanMode.Load()
        if configuredProfile < -1 || configuredProfile>3 {configuredProfile=3}
        if !v3SameRects(lastRects,chosen) || generation!=lastGeneration || len(screens)==0 {
            epoch++
            for _,s:=range screens{s.snap.close()}
            screens=nil
            clear()
            lastGeneration=generation
            lastRects=append(lastRects[:0],chosen...)
            union=v3UnionRect(chosen)
            if len(chosen)==0 {
                time.Sleep(time.Second)
                continue
            }
            var err error
            screens,err=v3CreateScreens(screen,chosen,configuredProfile,epoch)
            if err!=nil {
                logError(err)
                time.Sleep(time.Second)
                continue
            }
            nextAI=0
            aiServed=make([]int,len(screens))
            turn=0
            nextInfer=time.Time{}
            probeAt=time.Time{}
            probeFirst=nil
        }

        // The single AI worker may finish during the capture of any display.
        // A result must only affect the matching display and scene generation.
        for drain:=true;drain; {
            select {
            case result:=<-replies:
                pending=false
                pendingCamera.Store(0)
                if !v3AcceptReply(screens,result) {continue}
                s:=screens[result.display]
                if result.err!=nil {logError(result.err);nextInfer=now.Add(time.Second);continue}
                idx:=regionIndex(s.regions,result.zone)
                if idx<0 {continue}
                found:=v3KeepCameraDetections(result.detected,idx,s.snap.w,s.snap.h,s.profile)
                s.sch.markScannedPeople(idx,now,found)
                s.completed[idx]=now
                s.tracker.observePeopleFromCapture(found,result.captured,result.pixels,result.zone,result.capturedAt,now)
                if idx!=0 || len(s.regions)==1 {
                    tileID:=0
                    if v3GridColumns(s.profile)>0 {tileID=idx}
                    s.tracker.noteRegionScanSnapshot(result.zone,now,result.capturedAt,tileID,result.captured)
                }
                lastInferenceMS.Store(result.duration.Milliseconds())
                lastBusy=result.duration
                s.tracker.maxAIAge=recheckTTL(len(s.regions),s.profile,lastBusy)
                nextInfer=now.Add(v3AIPause(s.profile,result.duration,v3FullAccuracy.Load()))
            default:
                drain=false
            }
        }

        if len(screens)==0 {time.Sleep(150*time.Millisecond);continue}
        slot:=turn%len(screens)
        turn++
        s:=screens[slot]
        profile:=s.profile
        w,h:=s.snap.w,s.snap.h
        start:=time.Now()
        flags:=uintptr(srcCopy)
        if captureCompat.Load(){flags|=0x40000000}
        ok,_,_:=pBlt.Call(s.snap.dc,0,0,uintptr(w),uintptr(h),screen,
            uintptr(int64(s.rect.Left)),uintptr(int64(s.rect.Top)),flags)
        if ok==0 {
            if int(displaySelection.Load())==slot {
                v3CapturePreviewReady.Store(false)
            }
            s.captureFailures++
            // A dead video source must not leave stale green markers onscreen.
            if s.captureFailures==3 {
                epoch++
                s.epoch=epoch // reject any AI result from the failed capture
                s.tracker.reset()
                s.sch.reset(len(s.regions),w,h,profile)
                s.completed=make([]time.Time,len(s.regions))
                s.alarm.resetScene()
                v3Publish(screens,union,time.Now(),&lastPublished)
            }
            // Failed capture of the screen expected by the AI scheduler must
            // not block every other display from neural checks.
            if slot==nextAI {nextAI=v3SelectNextAI(screens,aiServed,slot)}
            if s.captureFailures==1 || s.captureFailures%20==0 {
                logError(fmt.Errorf("v3 monitor %d BitBlt failed (%d consecutive)",slot+1,s.captureFailures))
            }
            time.Sleep(150*time.Millisecond)
            continue
        }
        s.captureFailures=0
        pGdiFlush.Call()
        if configuredProfile==-1 && now.Sub(s.layoutAt)>850*time.Millisecond {
            s.layoutAt=now
            detected:=v3RecognizeLayout(s.snap.pixels,w,h)
            if detected==s.candidateProfile {
                s.candidateVotes++
            } else {
                s.candidateProfile=detected
                s.candidateVotes=1
            }
            if s.candidateVotes>=2 && detected!=s.profile {
                // Only invalidate the affected monitor; an in-flight result
                // from another monitor remains valid.
                epoch++
                v3SetScreenProfile(s,detected,epoch)
                aiServed=make([]int,len(screens))
                nextAI=slot // immediately prioritize newly expanded RVi camera
                profile=s.profile
                nextInfer=time.Time{}
            }
        }
        lastCaptureUS.Store(time.Since(start).Microseconds())

        // Diagnostic capture stays local and is scoped to the selected monitor.
        diagnostic:=int(displaySelection.Load())
        if diagnostic<0||diagnostic>=len(screens){diagnostic=0}
        if slot==diagnostic {
            stats:=v3CapturePixelStats(s.snap.pixels,w,h)
            if stats.Samples>0 {
                v3CapturePreviewMean.Store(int32(stats.MeanBrightness))
                v3CapturePreviewBlack.Store(int32(stats.BlackPercent))
                v3CapturePreviewRange.Store(int32(stats.DynamicRange))
                v3CapturePreviewSuspect.Store(stats.PossiblyBlank)
                v3CapturePreviewReady.Store(true)
            }
            if captureRequested.Swap(false) {
                go saveCapture(append([]byte(nil),s.snap.pixels...),w,h)
            }
            if captureProbeRequested.Swap(false) {
                probeFirst=append([]byte(nil),s.snap.pixels...)
                probeAt=time.Now()
                probeReady.Store(false)
                go saveCapture(append([]byte(nil),probeFirst...),w,h)
            }
            if !probeAt.IsZero() && time.Since(probeAt)>1400*time.Millisecond {
                delta:=sampleCaptureDifference(probeFirst,s.snap.pixels,w,h)
                captureProbePercent.Store(int32(delta))
                probeReady.Store(true)
                logError(fmt.Errorf("v3 capture test monitor %d: %d%% changed pixels",slot+1,delta))
                go saveCapture(append([]byte(nil),s.snap.pixels...),w,h)
                probeAt=time.Time{}
                probeFirst=nil
            }
        }

        if s.sch.observe(s.snap.pixels,w,h,s.regions) {
            // A scene cut on one display must not reject fresh inference
            // from unrelated displays. Requests carry per-display epochs.
            epoch++
            s.epoch=epoch
            s.tracker.reset()
            s.tracker.layout(w,h,v3GridColumns(profile))
            s.alarm.resetScene()
            s.sch.reset(len(s.regions),w,h,profile)
            s.completed=make([]time.Time,len(s.regions))
            nextInfer=time.Time{}
        }

        now=time.Now()
        trackingStart:=time.Now()
        s.tracker.step(s.snap.pixels,w,h,now)
        lastTrackingUS.Store(time.Since(trackingStart).Microseconds())

        // A strict screen round-robin prevents an active first monitor from
        // indefinitely starving displays two, three and four.
        if !pending && !now.Before(nextInfer) && slot==nextAI {
            index:=s.sch.nextAdaptive(now,profile,s.tracker.schedulerViews(now),s.regions)
            index=s.sch.confirmationRecheck(now,s.tracker.tentativeViews(now),s.regions,index)
            index=s.sch.ensureFirstCoverage(profile,s.regions,index)
            if index>=0 && index<len(s.regions) {
                region:=s.regions[index]
                crop:=copyRegionBGRA(s.snap.pixels,w,h,region)
                if crop!=nil {
                    captured:=s.tracker.schedulerViews(now)
                    select {
                    case req<-inferenceRequest{pixels:crop,contrast:contrastMode.Load(),
                        zone:region,epoch:s.epoch,display:slot,captured:captured,capturedAt:now}:
                        pending=true
                        // Human-readable status encodes physical monitor and zone.
                        pendingCamera.Store(int32((slot+1)*100+index))
                        s.sch.markScanned(index,now,false)
                        aiServed[slot]++
                        nextAI=v3SelectNextAI(screens,aiServed,slot)
                    default:
                    }
                }
            }
        }

        if s.alarm.tick(&s.tracker,now,alarmEnabled.Load(),time.Duration(alarmPauseSecs.Load())*time.Second) {
            if globalAlarmAt.IsZero() || now.Sub(globalAlarmAt)>=time.Duration(alarmPauseSecs.Load())*time.Second {
                globalAlarmAt=now
                playAviationAlert(alarmFull.Load())
            }
        }
        v3Publish(screens,union,now,&lastPublished)
        unseen,worst:=v3CameraCoverage(screens,now)
        v3UnvisitedCameras.Store(int32(unseen))
        v3WorstCameraAgeMS.Store(worst.Milliseconds())
        cadence:=v3FrameWait(profile,len(screens),len(s.tracker.people)>0,fastAI.Load(),v3FullAccuracy.Load())
        if elapsed:=time.Since(start);elapsed<cadence{time.Sleep(cadence-elapsed)}
    }
}
