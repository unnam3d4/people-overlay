//go:build windows
package main

import (
 "fmt"
 "sync"
 "syscall"
 "unsafe"
)

var (
 alarmWinmm=syscall.NewLazyDLL("winmm.dll")
 alarmPlaySound=alarmWinmm.NewProc("PlaySoundW")
 alarmWaves sync.Once
 alarmShort,alarmFull []byte
 alarmLoadErr error
)
const(
 soundAsync=1
 soundNodefault=2
 soundMemory=4
 soundNostop=16
)
func playAviationAlert(full bool){
 alarmWaves.Do(func(){
  alarmShort,alarmLoadErr=aviationAlarmWave(3)
  if alarmLoadErr!=nil{return}
  alarmFull,alarmLoadErr=aviationAlarmWave(8)
 })
 if alarmLoadErr!=nil{logError(alarmLoadErr);return}
 wave:=alarmShort
 if full{wave=alarmFull}
 if len(wave)==0{return}
 // The global slice stays alive for the entire session, even while
 // PlaySoundW plays it asynchronously.
 ok,_,err:=alarmPlaySound.Call(uintptr(unsafe.Pointer(&wave[0])),0,soundAsync|soundNodefault|soundMemory|soundNostop)
 if ok==0{logError(fmt.Errorf("PlaySoundW: %v",err))}
}
func stopAviationAlert(){alarmPlaySound.Call(0,0,0)}
