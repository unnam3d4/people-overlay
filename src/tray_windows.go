//go:build windows

package main

import (
 "fmt"
 "syscall"
 "unsafe"
)

const (
 trayMessage=0x8000+73 // WM_APP+73
 trayAdd=0
 trayDelete=2
 trayNIFMessage=1
 trayNIFIcon=2
 trayNIFTip=4
 menuString=0
 menuSeparator=0x800
 menuChecked=8
 menuReturnCmd=0x100
 menuRightButton=2
 menuIDToggle=100
 menuIDProfileEco=110
 menuIDProfileBalanced=111
 menuIDProfileCCTV=112
 menuIDSensLow=120
 menuIDSensNormal=121
 menuIDSensHigh=122
 menuIDContrast=130
 menuIDGreen=140
 menuIDRed=141
 menuIDOrange=142
 menuIDTest=150
 menuIDCapture=151
 menuIDExit=199
 wmRButtonUp=0x0205
 wmLButtonUp=0x0202
 wmLButtonDouble=0x0203
 wmContextMenu=0x007B
)

var(
 shell32=syscall.NewLazyDLL("shell32.dll")
 trayNotify=shell32.NewProc("Shell_NotifyIconW")
 trayCreateMenu=user32.NewProc("CreatePopupMenu")
 trayAppendMenu=user32.NewProc("AppendMenuW")
 trayDestroyMenu=user32.NewProc("DestroyMenu")
 trayTrackMenu=user32.NewProc("TrackPopupMenu")
 trayGetCursor=user32.NewProc("GetCursorPos")
 traySetForeground=user32.NewProc("SetForegroundWindow")
 trayRegisterWindowMessage=user32.NewProc("RegisterWindowMessageW")
 trayCreateIcon=user32.NewProc("CreateIcon")
 trayDestroyIcon=user32.NewProc("DestroyIcon")
 trayLoadIcon=user32.NewProc("LoadIconW")
)
type trayData struct{
 CbSize uint32
 HWnd uintptr
 UID uint32
 UFlags uint32
 UCallbackMessage uint32
 HIcon uintptr
 Tip [128]uint16
 State uint32
 StateMask uint32
 Info [256]uint16
 TimeoutVersion uint32
 InfoTitle [64]uint16
 InfoFlags uint32
 Guid [16]byte
 BalloonIcon uintptr
}
type appTray struct{
 hwnd uintptr
 icon uintptr
 ownsIcon bool
 data trayData
 taskbarMessage uint32
}
func trayIcon() (uintptr,bool){
 // Small green person within detector corner marks; transparent 32x32 icon.
 const n=32
 andBits:=make([]byte,n*4)
 for i:=range andBits{andBits[i]=255}
 xorBits:=make([]byte,n*n*4)
 mark:=func(x,y int,r,g,b uint8){
  if x<0||x>=n||y<0||y>=n{return}
  index:=(y*n+x)*4
  xorBits[index],xorBits[index+1],xorBits[index+2],xorBits[index+3]=b,g,r,255
  mask:=y*4+x/8
  andBits[mask]&^=byte(0x80>>uint(x%8))
 }
 for i:=5;i<=12;i++{
  for t:=0;t<3;t++{
   mark(i,4+t,6,229,97);mark(i,27-t,6,229,97)
   mark(4+t,i,6,229,97);mark(27-t,i,6,229,97)
  }
 }
 for x:=13;x<=18;x++{for y:=10;y<=14;y++{if (x-16)*(x-16)+(y-12)*(y-12)<=10{mark(x,y,250,250,250)}}}
 for x:=12;x<=19;x++{for y:=17;y<=24;y++{if x>=13+(y-17)/6&&x<=18-(y-17)/6{mark(x,y,250,250,250)}}}
 h,_,_:=trayCreateIcon.Call(0,n,n,1,32,uintptr(unsafe.Pointer(&andBits[0])),uintptr(unsafe.Pointer(&xorBits[0])))
 if h!=0{return h,true}
 fallback,_,_:=trayLoadIcon.Call(0,32512)
 return fallback,false
}
func newTray(hwnd uintptr)(*appTray,error){
 icon,owned:=trayIcon()
 t:=&appTray{hwnd:hwnd,icon:icon,ownsIcon:owned}
 t.data.CbSize=uint32(unsafe.Sizeof(t.data))
 t.data.HWnd=hwnd
 t.data.UID=1
 t.data.UFlags=trayNIFMessage|trayNIFIcon|trayNIFTip
 t.data.UCallbackMessage=trayMessage
 t.data.HIcon=icon
 copy(t.data.Tip[:],syscall.StringToUTF16("People Overlay AI - video person detection"))
 msg:=syscall.StringToUTF16Ptr("TaskbarCreated")
 taskbar,_,_:=trayRegisterWindowMessage.Call(uintptr(unsafe.Pointer(msg)))
 t.taskbarMessage=uint32(taskbar)
 ok,_,e:=trayNotify.Call(trayAdd,uintptr(unsafe.Pointer(&t.data)))
 if ok==0{if owned{trayDestroyIcon.Call(icon)};return nil,fmt.Errorf("Shell_NotifyIconW(NIM_ADD): %v",e)}
 return t,nil
}
func(t *appTray)reAdd(){if t!=nil{trayNotify.Call(trayAdd,uintptr(unsafe.Pointer(&t.data)))}}
func(t *appTray)close(){
 if t==nil{return}
 trayNotify.Call(trayDelete,uintptr(unsafe.Pointer(&t.data)))
 if t.ownsIcon{trayDestroyIcon.Call(t.icon)}
}
func appendMenu(menu uintptr,id uintptr,s string,checked bool){
 flags:=uintptr(menuString)
 if checked{flags|=menuChecked}
 str:=syscall.StringToUTF16Ptr(s)
 trayAppendMenu.Call(menu,flags,id,uintptr(unsafe.Pointer(str)))
}
func separator(menu uintptr){trayAppendMenu.Call(menu,menuSeparator,0,0)}
func(t *appTray)popupMenu(){
 if t==nil{return}
 menu,_,_:=trayCreateMenu.Call()
 if menu==0{return}
 defer trayDestroyMenu.Call(menu)
 active:=enabled.Load()
 action:="Временно выключить"
 if !active{action="Включить детекцию"}
 appendMenu(menu,menuIDToggle,action,false)
 separator(menu)
 appendMenu(menu,menuIDProfileEco,"Режим: экономный (2×2)",scanMode.Load()==1)
 appendMenu(menu,menuIDProfileBalanced,"Режим: сбалансированный (3×3)",scanMode.Load()==2)
 appendMenu(menu,menuIDProfileCCTV,"Режим: видеонаблюдение (4×4)",scanMode.Load()==3)
 separator(menu)
 appendMenu(menu,menuIDSensLow,"Чувствительность: низкая",sensitivity.Load()==1)
 appendMenu(menu,menuIDSensNormal,"Чувствительность: обычная",sensitivity.Load()==2)
 appendMenu(menu,menuIDSensHigh,"Чувствительность: высокая",sensitivity.Load()==3)
 appendMenu(menu,menuIDContrast,"Усилить контраст камер",contrastMode.Load())
 separator(menu)
 appendMenu(menu,menuIDGreen,"Цвет: зелёный",colour.Load()==1)
 appendMenu(menu,menuIDRed,"Цвет: красный",colour.Load()==2)
 appendMenu(menu,menuIDOrange,"Цвет: оранжевый",colour.Load()==3)
 separator(menu)
 appendMenu(menu,menuIDTest,"Проверить уголки",testOnly.Load())
 appendMenu(menu,menuIDCapture,"Сохранить снимок захвата",false)
 separator(menu)
 appendMenu(menu,menuIDExit,"Выход",false)
 var p point
 trayGetCursor.Call(uintptr(unsafe.Pointer(&p))
 traySetForeground.Call(t.hwnd)
 cmd,_,_:=trayTrackMenu.Call(menu,menuReturnCmd|menuRightButton,uintptr(p.X),uintptr(p.Y),0,t.hwnd,0)
 t.action(cmd)
}
func(t *appTray)action(id uintptr){
 switch id{
 case menuIDToggle:enabled.Store(!enabled.Load());version.Add(1)
 case menuIDProfileEco:scanMode.Store(1);version.Add(1)
 case menuIDProfileBalanced:scanMode.Store(2);version.Add(1)
 case menuIDProfileCCTV:scanMode.Store(3);version.Add(1)
 case menuIDSensLow:sensitivity.Store(1)
 case menuIDSensNormal:sensitivity.Store(2)
 case menuIDSensHigh:sensitivity.Store(3)
 case menuIDContrast:contrastMode.Store(!contrastMode.Load())
 case menuIDGreen:colour.Store(1);version.Add(1)
 case menuIDRed:colour.Store(2);version.Add(1)
 case menuIDOrange:colour.Store(3);version.Add(1)
 case menuIDTest:testOnly.Store(!testOnly.Load());version.Add(1)
 case menuIDCapture:captureRequested.Store(true)
 case menuIDExit:stopped.Store(true)
 }
}
