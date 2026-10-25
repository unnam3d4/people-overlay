//go:build windows && ncnn && !picodet
package main

import (
 "fmt"
 "path/filepath"
 "runtime"
 "syscall"
 "unsafe"
)

// Win7-targeted NCNN backend: single native inference thread, no MSVC runtime DLL.
type Detector struct {
 dll *syscall.DLL
 load,infer,closeProc,setThreads *syscall.Proc
}

func NewDetector(root string)(*Detector,error){
 dll,e:=syscall.LoadDLL(filepath.Join(root,"ncnnbridge.dll"))
 if e!=nil{return nil,fmt.Errorf("ncnnbridge.dll failed to load: %w",e)}
 fail:=func(e error)(*Detector,error){dll.Release();return nil,e}
 lp,e:=dll.FindProc("PPLLoad");if e!=nil{return fail(e)}
 ip,e:=dll.FindProc("PPLInfer");if e!=nil{return fail(e)}
 cp,e:=dll.FindProc("PPLClose");if e!=nil{return fail(e)}
 sp,e:=dll.FindProc("PPLSetThreads");if e!=nil{return fail(e)}
 d:=&Detector{dll:dll,load:lp,infer:ip,closeProc:cp,setThreads:sp}
 param,e:=syscall.UTF16PtrFromString(filepath.Join(root,"model.param"));if e!=nil{return fail(e)}
 bin,e:=syscall.UTF16PtrFromString(filepath.Join(root,"model.bin"));if e!=nil{return fail(e)}
 ret,_,_:=d.load.Call(uintptr(unsafe.Pointer(param)),uintptr(unsafe.Pointer(bin)))
 runtime.KeepAlive(param);runtime.KeepAlive(bin)
 if int32(ret)!=0{return fail(fmt.Errorf("NCNN model failed to load (code %d)",int32(ret)))}
 return d,nil
}
func(d *Detector)Predict(input []float32)([]float32,int,error){
 if len(input)!=3*640*640{return nil,0,fmt.Errorf("NCNN expected 3x640x640")}
 // Use exact fixed shape YOLOv8n COCO output: class probabilities + bounding boxes.
 out:=make([]float32,84*8400)
 var count int32
 ret,_,_:=d.infer.Call(uintptr(unsafe.Pointer(&input[0])),uintptr(len(input)),uintptr(unsafe.Pointer(&out[0])),uintptr(len(out)),uintptr(unsafe.Pointer(&count)))
 runtime.KeepAlive(input);runtime.KeepAlive(out)
 if int32(ret)!=0{return nil,0,fmt.Errorf("NCNN inference failure (code %d)",int32(ret))}
 if count<1||count>8400{return nil,0,fmt.Errorf("NCNN output count invalid: %d",count)}
 return out[:84*int(count)],int(count),nil
}
func(d *Detector)SetThreads(n int){
 if n<1{n=1};if n>2{n=2}
 if d!=nil&&d.setThreads!=nil{d.setThreads.Call(uintptr(n))}
}
func(d *Detector)Close(){
 if d==nil{return}
 if d.closeProc!=nil{d.closeProc.Call()}
 if d.dll!=nil{d.dll.Release();d.dll=nil}
}
