//go:build windows

package main

import("errors";"fmt";"os";"path/filepath";"runtime";"syscall";"unsafe")

type Detector struct{
 dll *syscall.DLL
 api uintptr
 env,options,session,memory uintptr
 inputName,outputName string
}
func ptrString(p uintptr)string{
 if p==0{return ""}
 var b []byte
 for i:=uintptr(0);i<2048;i++{
  c:=*(*byte)(unsafe.Pointer(p+i));if c==0{break};b=append(b,c)
 }
 return string(b)
}
func(d *Detector)fn(i uintptr)uintptr{return *(*uintptr)(unsafe.Pointer(d.api+i*unsafe.Sizeof(uintptr(0))))}
func ortCall(fn uintptr,args ...uintptr)uintptr{r,_,_:=syscall.SyscallN(fn,args...);return r}
func(d *Detector)check(s uintptr)error{
 if s==0{return nil}
 m:=ptrString(ortCall(d.fn(2),s))
 ortCall(d.fn(93),s)
 if m==""{m="Unknown ONNX Runtime error"}
 return errors.New(m)
}
func NewDetector(root string)(*Detector,error){
 dll,e:=syscall.LoadDLL(filepath.Join(root,"onnxruntime.dll"))
 if e!=nil{return nil,fmt.Errorf("onnxruntime.dll: %w",e)}
 p,e:=dll.FindProc("OrtGetApiBase")
 if e!=nil{dll.Release();return nil,e}
 base,_,_:=p.Call()
 if base==0{dll.Release();return nil,errors.New("OrtGetApiBase returned NULL")}
 getter:=*(*uintptr)(unsafe.Pointer(base))
 api:=ortCall(getter,11) // ABI v11: ONNX Runtime 1.11.1 or compatible newer runtime
 if api==0{dll.Release();return nil,errors.New("ORT API 11 unavailable")}
 d:=&Detector{dll:dll,api:api}
 fail:=func(e error)(*Detector,error){d.Close();return nil,e}
 id:=[]byte("PeopleOverlay\x00")
 if e=d.check(ortCall(d.fn(3),3,uintptr(unsafe.Pointer(&id[0])),uintptr(unsafe.Pointer(&d.env))));e!=nil{return fail(e)}
 if e=d.check(ortCall(d.fn(10),uintptr(unsafe.Pointer(&d.options))));e!=nil{return fail(e)}
 if e=d.check(ortCall(d.fn(24),d.options,1));e!=nil{return fail(e)}
 if e=d.check(ortCall(d.fn(25),d.options,1));e!=nil{return fail(e)}
 _=d.check(ortCall(d.fn(23),d.options,99))
 path,e:=syscall.UTF16PtrFromString(filepath.Join(root,"model.onnx"))
 if e!=nil{return fail(e)}
 if e=d.check(ortCall(d.fn(7),d.env,uintptr(unsafe.Pointer(path)),d.options,uintptr(unsafe.Pointer(&d.session))));e!=nil{return fail(fmt.Errorf("loading model: %w",e))}
 if e=d.check(ortCall(d.fn(69),0,0,uintptr(unsafe.Pointer(&d.memory))));e!=nil{return fail(e)}
 var alloc uintptr
 if e=d.check(ortCall(d.fn(78),uintptr(unsafe.Pointer(&alloc))));e!=nil{return fail(e)}
 if alloc==0{return fail(errors.New("ORT default allocator NULL"))}
 for idx:=0;idx<2;idx++{
  var name uintptr
  if e=d.check(ortCall(d.fn(uintptr(36+idx)),d.session,0,alloc,uintptr(unsafe.Pointer(&name))));e!=nil{return fail(e)}
  if name==0{return fail(errors.New("ONNX tensor name NULL"))}
  str:=ptrString(name);ortCall(d.fn(76),alloc,name)
  if str==""{return fail(errors.New("empty model tensor name"))}
  if idx==0{d.inputName=str}else{d.outputName=str}
 }
 return d,nil
}
func(d *Detector)Close(){
 if d==nil||d.api==0{return}
 if d.memory!=0{ortCall(d.fn(94),d.memory)}
 if d.session!=0{ortCall(d.fn(95),d.session)}
 if d.options!=0{ortCall(d.fn(100),d.options)}
 if d.env!=0{ortCall(d.fn(92),d.env)}
 if d.dll!=nil{d.dll.Release()}
 d.api=0
}
func(d *Detector)Predict(input []float32)([]float32,int,error){
 if len(input)!=3*640*640{return nil,0,errors.New("expected 3x640x640 input")}
 shape:=[4]int64{1,3,640,640}
 var in,out,si uintptr
 defer func(){
  if si!=0{ortCall(d.fn(99),si)}
  if out!=0{ortCall(d.fn(96),out)}
  if in!=0{ortCall(d.fn(96),in)}
 }()
 e:=d.check(ortCall(d.fn(49),d.memory,uintptr(unsafe.Pointer(&input[0])),uintptr(len(input)*4),uintptr(unsafe.Pointer(&shape[0])),4,1,uintptr(unsafe.Pointer(&in))))
 if e!=nil{return nil,0,e}
 ins,outs:=append([]byte(d.inputName),0),append([]byte(d.outputName),0)
 ip,op:=uintptr(unsafe.Pointer(&ins[0])),uintptr(unsafe.Pointer(&outs[0]))
 e=d.check(ortCall(d.fn(9),d.session,0,uintptr(unsafe.Pointer(&ip)),uintptr(unsafe.Pointer(&in)),1,uintptr(unsafe.Pointer(&op)),1,uintptr(unsafe.Pointer(&out))))
 runtime.KeepAlive(input);runtime.KeepAlive(ins);runtime.KeepAlive(outs)
 if e!=nil{return nil,0,e}
 if out==0{return nil,0,errors.New("ONNX output missing")}
 if e=d.check(ortCall(d.fn(65),out,uintptr(unsafe.Pointer(&si))));e!=nil{return nil,0,e}
 var rank uintptr
 if e=d.check(ortCall(d.fn(61),si,uintptr(unsafe.Pointer(&rank))));e!=nil{return nil,0,e}
 if rank!=3{return nil,0,fmt.Errorf("unexpected model output rank %d",rank)}
 dims:=[3]int64{}
 if e=d.check(ortCall(d.fn(62),si,uintptr(unsafe.Pointer(&dims[0])),3));e!=nil{return nil,0,e}
 if dims[0]!=1||dims[1]!=84||dims[2]<1||dims[2]>50000{return nil,0,fmt.Errorf("unsupported YOLO output shape %v",dims)}
 var ptr uintptr
 if e=d.check(ortCall(d.fn(51),out,uintptr(unsafe.Pointer(&ptr))));e!=nil{return nil,0,e}
 if ptr==0{return nil,0,errors.New("ONNX output data NULL")}
 data:=unsafe.Slice((*float32)(unsafe.Pointer(ptr)),int(dims[1]*dims[2]))
 return append([]float32(nil),data...),int(dims[2]),nil
}
