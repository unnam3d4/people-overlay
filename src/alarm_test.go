package main

import (
 "encoding/binary"
 "testing"
 "time"
)
func TestRecreatedUserAlarmSamples(t *testing.T){
 for _,pulses:=range []int{3,8}{
  wav,err:=aviationAlarmWave(pulses)
  if err!=nil{t.Fatal(err)}
  if string(wav[:4])!="RIFF"||string(wav[8:12])!="WAVE"{t.Fatalf("not WAV: %d",pulses)}
  if binary.LittleEndian.Uint32(wav[24:28])!=8000||wav[34]!=8{t.Fatal("wrong Windows 7 compatible PCM parameters")}
  if int(binary.LittleEndian.Uint32(wav[40:44]))!=len(wav)-44{t.Fatal("WAV data length incorrect")}
  if len(wav)<6000{t.Fatal("truncated alarm")}
 }
 if _,err:=aviationAlarmWave(0);err==nil{t.Fatal("invalid alert length accepted")}
}
func TestNewPersonTriggersOnceAndCooldown(t *testing.T){
 now:=time.Unix(100000,0)
 var gate alarmGate
 var tr personTracker
 tr.layout(1920,1080,4)
 tr.observePeople([]Rect{{X:100,Y:120,W:40,H:95,Score:.94}},now)
 if gate.tick(&tr,now,true,30*time.Second){t.Fatal("instant alarm without persistence")}
 if !gate.tick(&tr,now.Add(800*time.Millisecond),true,30*time.Second){t.Fatal("confirmed new person should sound")}
 if gate.tick(&tr,now.Add(32*time.Second),true,30*time.Second){t.Fatal("same standing person repeated")}
 // Same zone, new ID does not re-trigger within 90s.
 tr.reset()
 tr.observePeople([]Rect{{X:108,Y:125,W:40,H:95,Score:.94}},now.Add(35*time.Second))
 _=gate.tick(&tr,now.Add(35*time.Second),true,30*time.Second)
 if gate.tick(&tr,now.Add(36*time.Second),true,30*time.Second){t.Fatal("same area retriggered")}
 // Brand new area outside global cooldown can sound.
 tr.observePeople([]Rect{{X:850,Y:620,W:50,H:110,Score:.95}},now.Add(45*time.Second))
 _=gate.tick(&tr,now.Add(45*time.Second),true,30*time.Second)
 if !gate.tick(&tr,now.Add(46*time.Second),true,30*time.Second){t.Fatal("new area was suppressed forever")}
}
func TestMutedDoesNotQueueLaterAlarm(t *testing.T){
 now:=time.Unix(10000,0)
 var gate alarmGate
 var tr personTracker
 tr.observePeople([]Rect{{X:50,Y:50,W:40,H:100,Score:.96}},now)
 _=gate.tick(&tr,now,false,15*time.Second)
 if gate.tick(&tr,now.Add(time.Second),false,15*time.Second){t.Fatal("muted alarm")}
 if gate.tick(&tr,now.Add(2*time.Second),true,15*time.Second){t.Fatal("old person beeped when sound unmuted")}
}
