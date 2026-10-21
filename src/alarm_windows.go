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
 alarmShortPCM,alarmFullPCM []byte
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
  alarmShortPCM,alarmLoadErr=aviationAlarmWave(3)
  if alarmLoadErr!=nil{return}
  alarmFullPCM,alarmLoadErr=aviationAlarmWave(8)
 })
 if alarmLoadErr!=nil{logError(alarmLoadErr);return}
 wave:=alarmShortPCM
 if full{wave=alarmFullPCM}
 if len(wave)==0{return}
 // The global slice stays alive for the entire session, even while
 // PlaySoundW plays it asynchronously.
 ok,_,err:=alarmPlaySound.Call(uintptr(unsafe.Pointer(&wave[0])),0,soundAsync|soundNodefault|soundMemory|soundNostop)
 if ok==0{logError(fmt.Errorf("PlaySoundW: %v",err))}
}
func stopAviationAlert(){alarmPlaySound.Call(0,0,0)}
