//go:build windows && ncnn && picodet

package main

import (
	"fmt"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"
)

// Distinct compile-time engine: fails closed if paired with a YOLO DLL.
// The C++ bridge decodes PicoDet's 4 (classification, DFL) feature heads.
type Detector struct {
	dll *syscall.DLL
	load,infer,closeProc,setThreads *syscall.Proc
}

func NewDetector(root string) (*Detector,error) {
	dll,e:=syscall.LoadDLL(filepath.Join(root,"ncnnbridge.dll"))
	if e!=nil{return nil,fmt.Errorf("PicoDet NCNN DLL: %w",e)}
	fail:=func(cause error)(*Detector,error){dll.Release();return nil,cause}
	kind,e:=dll.FindProc("PPLModelKind")
	if e!=nil{return fail(fmt.Errorf("PicoDet build requires its own verified DLL: %w",e))}
	magic,_,_:=kind.Call()
	if magic!=0x5049434f{return fail(fmt.Errorf("incorrect neural engine: expected PicoDet, got 0x%x",magic))}
	lp,e:=dll.FindProc("PPLLoad");if e!=nil{return fail(e)}
	ip,e:=dll.FindProc("PPLInfer");if e!=nil{return fail(e)}
	cp,e:=dll.FindProc("PPLClose");if e!=nil{return fail(e)}
	sp,e:=dll.FindProc("PPLSetThreads");if e!=nil{return fail(e)}
	d:=&Detector{dll:dll,load:lp,infer:ip,closeProc:cp,setThreads:sp}
	param,e:=syscall.UTF16PtrFromString(filepath.Join(root,"model.param"));if e!=nil{return fail(e)}
	bin,e:=syscall.UTF16PtrFromString(filepath.Join(root,"model.bin"));if e!=nil{return fail(e)}
	ret,_,_:=d.load.Call(uintptr(unsafe.Pointer(param)),uintptr(unsafe.Pointer(bin)))
	runtime.KeepAlive(param);runtime.KeepAlive(bin)
	if int32(ret)!=0{d.Close();return nil,fmt.Errorf("PicoDet NCNN model load failed: %d",int32(ret))}
	return d,nil
}
func(d *Detector)Predict(input []float32)([]float32,int,error){
	if len(input)!=3*modelInputSize*modelInputSize{return nil,0,fmt.Errorf("PicoDet input requires 3x%dx%d",modelInputSize,modelInputSize)}
	out:=make([]float32,6*100)
	var count int32
	ret,_,_:=d.infer.Call(uintptr(unsafe.Pointer(&input[0])),uintptr(len(input)),uintptr(unsafe.Pointer(&out[0])),uintptr(len(out)),uintptr(unsafe.Pointer(&count)))
	runtime.KeepAlive(input);runtime.KeepAlive(out)
	if int32(ret)!=0{return nil,0,fmt.Errorf("PicoDet inference failed (%d)",int32(ret))}
	if count<0||count>100{return nil,0,fmt.Errorf("invalid PicoDet count %d",count)}
	return out[:6*int(count)],int(count),nil
}
func(d *Detector)SetThreads(n int){
	if n<1{n=1};if n>2{n=2}
	if d!=nil&&d.setThreads!=nil{d.setThreads.Call(uintptr(n))}
}
func(d *Detector)Close(){
	if d==nil{return}
	if d.closeProc!=nil{d.closeProc.Call();d.closeProc=nil}
	if d.dll!=nil{d.dll.Release();d.dll=nil}
}
