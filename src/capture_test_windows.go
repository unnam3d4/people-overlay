//go:build windows

package main

// Opt-in desktop integration test. It displays only the supplied fixture,
// captures that rectangle with the production GDI path, runs actual inference,
// and submits the real transparent overlay. It never opens or changes RVi.
import (
 "encoding/json"
 "fmt"
 "image"
 "image/png"
 "os"
 "path/filepath"
 "runtime"
 "syscall"
 "time"
 "unsafe"
)

func saveTestPixels(path string,pixels []byte,w,h int)error{
 img:=image.NewNRGBA(image.Rect(0,0,w,h))
 for i:=0;i<w*h;i++{q:=i*4;img.Pix[q],img.Pix[q+1],img.Pix[q+2],img.Pix[q+3]=pixels[q+2],pixels[q+1],pixels[q],255}
 f,e:=os.Create(path);if e!=nil{return e};defer f.Close();return png.Encode(f,img)
}

func runCaptureTest(manifestPath,output string)error{
 runtime.LockOSThread();defer runtime.UnlockOSThread();pDPI.Call()
 raw,e:=os.ReadFile(manifestPath);if e!=nil{return e}
 var m benchmarkManifest;if e=json.Unmarshal(raw,&m);e!=nil{return e}
 index:=m.FPS*3;if index>=len(m.Frames){index=0};if len(m.Frames)==0{return fmt.Errorf("empty manifest")}
 path:=filepath.Join(filepath.Dir(manifestPath),m.Frames[index].File)
 pixels,w,h,e:=benchmarkLoad(path);if e!=nil{return e}
 sw,sh:=screenSize();if w>sw||h>sh{return fmt.Errorf("fixture %dx%d exceeds interactive desktop %dx%d",w,h,sw,sh)}
 if e=os.MkdirAll(output,0755);e!=nil{return e}
 cls:=syscall.StringToUTF16Ptr("STATIC");title:=syscall.StringToUTF16Ptr("People Overlay - isolated CCTV test fixture")
 wnd,_,err:=pWindow.Call(toolWindow|noActivate|topMost,uintptr(unsafe.Pointer(cls)),uintptr(unsafe.Pointer(title)),popUp,0,0,uintptr(w),uintptr(h),0,0,0,0)
 if wnd==0{return fmt.Errorf("fixture window: %v",err)}
 defer trayDestroyWindow.Call(wnd)
 pShow.Call(wnd,4)
 // Pump the initial WM_PAINT before supplying the video frame.
 peek:=user32.NewProc("PeekMessageW");var msg message
 for {ok,_,_:=peek.Call(uintptr(unsafe.Pointer(&msg)),0,0,0,1);if ok==0{break};pTranslate.Call(uintptr(unsafe.Pointer(&msg)));pDispatch.Call(uintptr(unsafe.Pointer(&msg)))}
 screen,_,_:=pDC.Call(0);if screen==0{return fmt.Errorf("no interactive screen DC")};defer pReleaseDC.Call(0,screen)
 source,e:=newDIB(screen,w,h);if e!=nil{return e};defer source.close();copy(source.pixels,pixels)
 client,_,_:=pDC.Call(wnd);if client==0{return fmt.Errorf("fixture GetDC failed")};defer pReleaseDC.Call(wnd,client)
 if ok,_,_:=pBlt.Call(client,0,0,uintptr(w),uintptr(h),source.dc,0,0,srcCopy);ok==0{return fmt.Errorf("fixture BitBlt failed")}
 pGdiFlush.Call();time.Sleep(250*time.Millisecond)
 captured,e:=newDIB(screen,w,h);if e!=nil{return e};defer captured.close()
 begun:=time.Now();ok,_,_:=pBlt.Call(captured.dc,0,0,uintptr(w),uintptr(h),screen,0,0,srcCopy);pGdiFlush.Call();captureMS:=benchmarkMS(time.Since(begun))
 if ok==0{return fmt.Errorf("screen BitBlt failed")}
 different:=0
 for i:=0;i<w*h;i++{q:=i*4;for c:=0;c<3;c++{if absTrack(int(pixels[q+c])-int(captured.pixels[q+c]))>4{different++;break}}}
 mismatch:=float64(different)/float64(w*h)
 if mismatch>.02{return fmt.Errorf("interactive screen did not reproduce fixture (%.2f%% pixels differ)",mismatch*100)}
 if e=saveTestPixels(filepath.Join(output,"captured.png"),captured.pixels,w,h);e!=nil{return e}
 root,e:=locateResources();if e!=nil{return e};d,e:=NewDetector(root);if e!=nil{return e};defer d.Close()
 regs:=regionsForProfile(w,h,m.Profile);var found []Rect;var stages []benchmarkTimes
 for i,r:=range regs{
  if i==0&&len(regs)>1{continue}
  crop:=copyRegionBGRA(captured.pixels,w,h,r)
  boxes,timing,err:=benchmarkPredict(d,crop,r.w,r.h,r,.32,false);if err!=nil{return err}
  found=append(found,boxes...);stages=append(stages,timing)
 }
 var tracker personTracker;tracker.enableConfirmation();cols:=0;if m.Profile==3{cols=4};tracker.layout(w,h,cols)
 at:=time.Now();tracker.observePeopleFromCapture(found,nil,captured.pixels,scanRegion{0,0,w,h},at,at)
 views:=tracker.views(at)
 if len(views)==0{return fmt.Errorf("screen capture inference produced no confirmed people")}
 overlay,e:=makeOverlay(w,h);if e!=nil{return e};defer trayDestroyWindow.Call(overlay)
 layer,e:=newDIB(screen,w,h);if e!=nil{return e};defer layer.close();drawTrackedCorners(&layer,views,1,false)
 pos,size,origin,mode:=point{},dimensions{int32(w),int32(h)},point{},blend{ConstantAlpha:255,AlphaFormat:1}
 begun=time.Now();ok,_,_=pUpdate.Call(overlay,screen,uintptr(unsafe.Pointer(&pos)),uintptr(unsafe.Pointer(&size)),layer.dc,uintptr(unsafe.Pointer(&origin)),0,uintptr(unsafe.Pointer(&mode)),ulwAlpha)
 renderMS:=benchmarkMS(time.Since(begun));if ok==0{return fmt.Errorf("UpdateLayeredWindow integration test failed")}
 // Explicitly permit the isolated test overlay in its proof screenshot.
 pAffinity.Call(overlay,0);time.Sleep(200*time.Millisecond)
 proof,e:=newDIB(screen,w,h);if e!=nil{return e};defer proof.close()
 ok,_,_=pBlt.Call(proof.dc,0,0,uintptr(w),uintptr(h),screen,0,0,srcCopy|0x40000000);pGdiFlush.Call()
 if ok==0{return fmt.Errorf("proof screenshot capture failed")}
 if e=saveTestPixels(filepath.Join(output,"overlay.png"),proof.pixels,w,h);e!=nil{return e}
 return benchmarkWrite(filepath.Join(output,"capture-test.json"),map[string]interface{}{
  "passed":true,"capture_ms":captureMS,"render_ms":renderMS,"source_mismatch_fraction":mismatch,
  "tracks":views,"inference_stages":stages,"frame":m.Frames[index],"profile":m.Profile,
  "scope":"actual interactive Win32 fixture window -> screen BitBlt/DIB -> tiled NCNN -> tracker -> UpdateLayeredWindow -> screenshot; single displayed frame, not RVi or real moving footage",
 })
}
