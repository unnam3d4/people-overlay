//go:build windows

package main

import (
 "fmt"
 "time"
)

type inferenceRequest struct {
 input []float32
 scale,padX,padY float64
 zone scanRegion
 epoch uint64
}

type inferenceReply struct {
 detected []Rect
 zone scanRegion
 epoch uint64
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
   begun:=time.Now()
   output,n,err:=detector.Predict(req.input)
   var found []Rect
   if err==nil {
    found=decodePeople(output,n,req.zone.w,req.zone.h,req.scale,req.padX,req.padY,confidenceThreshold(sensitivity.Load()))
    for i:=range found{
     found[i].X+=float64(req.zone.x)
     found[i].Y+=float64(req.zone.y)
    }
   }
   result:=inferenceReply{detected:found,zone:req.zone,epoch:req.epoch,duration:time.Since(begun),err:err}
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

 req:=make(chan inferenceRequest)
 replies:=make(chan inferenceReply,1)
 workerDone:=make(chan struct{})
 go func(){defer close(workerDone);inferenceWorker(d,req,replies)}()
 defer func(){close(req);<-workerDone}()

 var sch scanScheduler
 var tracker personTracker
 var regions []scanRegion
 var epoch uint64
 var nextInference time.Time
 var lastBusy time.Duration=350*time.Millisecond
 lastEnabled:=false

 publish:=func(now time.Time){
  items:=tracker.views(now)
  boxes.mu.Lock()
  changed:=!sameTrackViews(boxes.items,items)
  boxes.items=items
  boxes.updated=now
  boxes.mu.Unlock()
  if changed{version.Add(1)}
 }

 for !stopped.Load(){
  active:=enabled.Load()&&!testOnly.Load()
  if !active{
   if lastEnabled{
    tracker.reset()
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
   epoch++
   nextInference=time.Time{}
   publish(time.Now())
  }

  start:=time.Now()
  ok,_,_:=pBlt.Call(snap.dc,0,0,uintptr(w),uintptr(h),screen,0,0,srcCopy)
  if ok==0{logError(fmt.Errorf("BitBlt capture failed"));time.Sleep(time.Second);continue}
  if captureRequested.Swap(false){
   pixels:=append([]byte(nil),snap.pixels...)
   go saveCapture(pixels,w,h)
  }
  sch.observe(snap.pixels,w,h,regions)
  now:=time.Now()
  tracker.step(snap.pixels,w,h,now)

  // Consume any finished inference from the previous camera crop.
  // Correct tracked positions with the neural boxes, but leave all other
  // tracked people free to continue moving between expensive detections.
  for drain:=true;drain;{
   select{
   case result:=<-replies:
    if result.epoch!=epoch{continue}
    if result.err!=nil{
     logError(result.err)
     enabled.Store(false)
     version.Add(1)
     break
    }
    sch.markScanned(regionIndex(regions,result.zone),now,len(result.detected)>0)
    tracker.observePeople(result.detected,now)
    lastBusy=result.duration
    nextInference=now.Add(throttleFor(profile,result.duration))
   default:
    drain=false
   }
  }
  if !enabled.Load(){time.Sleep(150*time.Millisecond);continue}

  // The unbuffered request channel is a backpressure mechanism: when AI is
  // busy, send fails instantly instead of accumulating old video frames.
  if !now.Before(nextInference){
   index:=sch.next(now,profile)
   region:=regions[index]
   if len(req)==0{
    input,scale,px,py:=prepareRegionFrameEnhanced(snap.pixels,w,h,region,contrastMode.Load())
    if input!=nil {
     select{
     case req<-inferenceRequest{input:input,scale:scale,padX:px,padY:py,zone:region,epoch:epoch}:
      sch.markScanned(index,now,false)
      nextInference=now.Add(throttleFor(profile,lastBusy))
     default:
     }
    }
   }
  }
  publish(time.Now())
  // This lightweight loop moves markers even during a multi-second CPU AI
  // call. Eco and balanced protect older machines from excessive capture.
  cadence:=230*time.Millisecond
  if profile==1{cadence=320*time.Millisecond}
  if profile==3{cadence=140*time.Millisecond}
  elapsed:=time.Since(start)
  if elapsed<cadence{time.Sleep(cadence-elapsed)}
 }
}
func regionIndex(regions []scanRegion,which scanRegion)int{
 for i,r:=range regions{if r==which{return i}}
 return 0
}
