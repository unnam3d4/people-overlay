//go:build windows

package main

import (
 "fmt"
 "time"
)

type inferenceRequest struct {
 // Raw cropped BGRA frame: no 640px resize on the capture/tracking thread.
 pixels []byte
 contrast bool
 zone scanRegion
 epoch uint64
 captured []TrackView
}

type inferenceReply struct {
 detected []Rect
 zone scanRegion
 epoch uint64
 captured []TrackView
 duration time.Duration
 err error
}

// YOLO is deliberately confined to one worker on one CPU thread. Capturing
// screen images and moving existing corner markers never wait for inference.
func inferenceWorker(detector *Detector,requests <-chan inferenceRequest,replies chan<- inferenceReply){
 for !stopped.Load(){
  select{
  case <-time.After(200*time.Millisecond):
   continue
  case req,ok:=<-requests:
   if !ok{return}
   // Switch only on the AI worker, never while an inference is in flight.
   if control,ok:=interface{}(detector).(interface{SetThreads(int)});ok{
    threads:=1;if fastAI.Load(){threads=2};control.SetThreads(threads)
   }
   begun:=time.Now()
   // Bilinear resize, contrast and float conversion execute only here.
   // This prevents old single-core Windows 7 systems from pausing overlay
   // tracking while preparing a costly 640x640 neural input.
   input,scale,px,py:=prepareRegionFrameEnhanced(req.pixels,req.zone.w,req.zone.h,
    scanRegion{0,0,req.zone.w,req.zone.h},req.contrast)
   var output []float32
   var n int
   var err error
   if input==nil{err=fmt.Errorf("invalid captured AI crop")}else{output,n,err=detector.Predict(input)}
   var found []Rect
   if err==nil {
    found=decodePeople(output,n,req.zone.w,req.zone.h,scale,px,py,confidenceThreshold(sensitivity.Load()))
    for i:=range found{
     found[i].X+=float64(req.zone.x)
     found[i].Y+=float64(req.zone.y)
    }
   }
   result:=inferenceReply{detected:found,zone:req.zone,epoch:req.epoch,captured:req.captured,duration:time.Since(begun),err:err}
   // One result slot suffices because requests are unbuffered and producer
   // deliberately never queues stale camera images.
   select{case replies<-result:case <-time.After(500*time.Millisecond):}
  }
 }
}

func analyze(d *Detector){
 screen,_,_:=pDC.Call(0)
 if screen==0{logError(fmt.Errorf("GetDC failed"));return}
 defer pReleaseDC.Call(0,screen)
 var snap dib
 defer snap.close()

 req:=make(chan inferenceRequest,1)
 replies:=make(chan inferenceReply,1)
 workerDone:=make(chan struct{})
 go func(){defer close(workerDone);inferenceWorker(d,req,replies)}()
 defer func(){close(req);<-workerDone}()

 var sch scanScheduler
 var tracker personTracker
 var alarm alarmGate
 var regions []scanRegion
 var epoch uint64
 var pending bool
 var nextInference time.Time
 var lastBusy time.Duration=350*time.Millisecond
 lastEnabled:=false
 var lastPaint []TrackView
 var probeAt time.Time
 var probeFirst []byte

 publish:=func(now time.Time){
  items:=tracker.views(now)
  boxes.mu.Lock()
  boxes.items=items
  boxes.updated=now
  trackedCount.Store(int32(len(items)))
  boxes.mu.Unlock()
  // Compare against the last frame actually sent to the renderer, not
  // the prior tracking sample; otherwise slow 1px-per-frame movement
  // would never exceed the repaint threshold.
  if !sameTrackViews(lastPaint,items){
   lastPaint=append(lastPaint[:0],items...)
   version.Add(1)
  }
 }

 for !stopped.Load(){
  active:=enabled.Load()&&!testOnly.Load()
  if !active{
   if lastEnabled{
    // An in-flight old frame must not re-create markers after re-enabling.
    epoch++
    tracker.reset()
    alarm.resetScene()
    lastPaint=nil
    boxes.mu.Lock();boxes.items=nil;boxes.updated=time.Time{};boxes.mu.Unlock()
    version.Add(1)
   }
   lastEnabled=false
   time.Sleep(130*time.Millisecond)
   continue
  }
  lastEnabled=true
  w,h:=screenSize()
  if w<64||h<64{time.Sleep(time.Second);continue}
  profile:=scanMode.Load()
  if profile<1||profile>3{profile=2}
  if snap.w!=w||snap.h!=h||profile!=sch.previousProfile{
   snap.close()
   var e error
   snap,e=newDIB(screen,w,h)
   if e!=nil{logError(e);time.Sleep(time.Second);continue}
   regions=regionsForProfile(w,h,profile)
   sch.reset(len(regions),w,h,profile)
   tracker.reset()
   cols:=0
   if profile==3{cols=4}
   tracker.layout(w,h,cols)
   epoch++
   nextInference=time.Time{}
   publish(time.Now())
  }

  start:=time.Now()
  captureFlags:=uintptr(srcCopy)
  if captureCompat.Load(){captureFlags|=0x40000000} // CAPTUREBLT: compositor/layered-window compatibility
  ok,_,_:=pBlt.Call(snap.dc,0,0,uintptr(w),uintptr(h),screen,0,0,captureFlags)
  if ok==0{logError(fmt.Errorf("BitBlt capture failed"));time.Sleep(time.Second);continue}
  if captureRequested.Swap(false){
   pixels:=append([]byte(nil),snap.pixels...)
   go saveCapture(pixels,w,h)
  }
  // Explicit capture diagnostic: two independently sampled actual BitBlt
  // frames, about 1.4 seconds apart. Photos can work while GPU video layers
  // return blank/frozen data. These screenshots stay local to the PC.
  if captureProbeRequested.Swap(false){
   probeFirst=append(probeFirst[:0],snap.pixels...)
   probeAt=time.Now()
   probeReady.Store(false)
   go saveCapture(append([]byte(nil),probeFirst...),w,h)
  }
  if !probeAt.IsZero()&&time.Since(probeAt)>1400*time.Millisecond{
   delta:=sampleCaptureDifference(probeFirst,snap.pixels,w,h)
   captureProbePercent.Store(int32(delta))
   probeReady.Store(true)
   logError(fmt.Errorf("Capture diagnostic: changed pixels %d%% in 1.4s (0%% with visible video motion suggests inaccessible GPU video layer); compatibility=%v",delta,captureCompat.Load()))
   pixels:=append([]byte(nil),snap.pixels...)
   go saveCapture(pixels,w,h)
   probeAt=time.Time{}
   probeFirst=nil
  }
  // A simultaneous hard scene cut on most cameras means the user switched
  // RVi layouts. Drop stale tracks and reject any AI result from the old view.
  if sch.observe(snap.pixels,w,h,regions){
   epoch++
   tracker.reset()
   tracker.layout(w,h,func()int{if profile==3{return 4};return 0}())
   alarm.resetScene()
   sch.reset(len(regions),w,h,profile)
   nextInference=time.Time{}
   publish(time.Now())
  }
  now:=time.Now()
  tracker.step(snap.pixels,w,h,now)

  // Consume any finished inference from the previous camera crop.
  // Correct tracked positions with the neural boxes, but leave all other
  // tracked people free to continue moving between expensive detections.
  for drain:=true;drain;{
   select{
   case result:=<-replies:
    pending=false
    if result.epoch!=epoch{continue}
    if result.err!=nil{
     logError(result.err)
     enabled.Store(false)
     version.Add(1)
     break
    }
    sch.markScanned(regionIndex(regions,result.zone),now,len(result.detected)>0)
    corrected:=compensateLateDetections(result.detected,result.captured,tracker.views(now))
    tracker.observePeople(corrected,now)
    // A targeted camera crop that no longer contains the person is
    // negative evidence. Two negative neural checks remove ghost corners.
    if regionIndex(regions,result.zone)!=0 {
     tracker.noteRegionScan(result.zone,now)
    }
    lastInferenceMS.Store(result.duration.Milliseconds())
    lastBusy=result.duration
    tracker.maxAIAge=recheckTTL(len(regions),profile,lastBusy)
    nextInference=now.Add(throttleFor(profile,result.duration))
   default:
    drain=false
   }
  }
  if !enabled.Load(){time.Sleep(150*time.Millisecond);continue}

  // The unbuffered request channel is a backpressure mechanism: when AI is
  // busy, send fails instantly instead of accumulating old video frames.
  if !pending&&!now.Before(nextInference){
   index:=sch.priorityTracked(tracker.views(now),regions,now)
   if index<0{index=sch.next(now,profile)}
   region:=regions[index]
   crop:=copyRegionBGRA(snap.pixels,w,h,region)
   if crop!=nil{
    captured:=tracker.views(now)
    // At most one raw crop is queued; inference never builds up a backlog.
    // The source DIB may change during inference, so copy only selected ROI.
    select{
    case req<-inferenceRequest{pixels:crop,contrast:contrastMode.Load(),zone:region,epoch:epoch,captured:captured}:
     pending=true
     sch.markScanned(index,now,false)
     nextInference=now.Add(throttleFor(profile,lastBusy))
    default:
    }
   }
  }
  now=time.Now()
  // The alarm gate sees the same unique people as the overlay. It requires
  // persistence and visual validation; it never fires on every AI frame.
  if alarm.tick(&tracker,now,alarmEnabled.Load(),time.Duration(alarmPauseSecs.Load())*time.Second){
   playAviationAlert(alarmFull.Load())
  }
  publish(now)
  // This lightweight loop moves markers even during a multi-second CPU AI
  // call. Eco and balanced protect older machines from excessive capture.
  cadence:=300*time.Millisecond
  if len(tracker.people)>0{
   cadence=210*time.Millisecond
   if profile==3{cadence=115*time.Millisecond}
   if profile==1{cadence=340*time.Millisecond}
  }else if profile==1{cadence=440*time.Millisecond}
  elapsed:=time.Since(start)
  if elapsed<cadence{time.Sleep(cadence-elapsed)}
 }
}
func regionIndex(regions []scanRegion,which scanRegion)int{
 for i,r:=range regions{if r==which{return i}}
 return 0
}
