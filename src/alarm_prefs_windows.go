//go:build windows
package main

import (
 "encoding/json"
 "os"
 "path/filepath"
)

type alarmPreferences struct {
 Enabled bool `json:"enabled"`
 Full bool `json:"full"`
 PauseSeconds int32 `json:"pause_seconds"`
}

func alarmPreferencesPath()string{
 root:=os.Getenv("LOCALAPPDATA")
 if root==""{return ""}
 return filepath.Join(root,"PeopleOverlay","alarm_settings.json")
}

// Alert settings survive application / computer restarts. Other tray
// preferences intentionally stay session-local and do not alter RVi.
func loadAlarmPreferences(){
 path:=alarmPreferencesPath()
 if path==""{return}
 raw,err:=os.ReadFile(path)
 if err!=nil{return}
 var p alarmPreferences
 if err=json.Unmarshal(raw,&p);err!=nil{logError(err);return}
 if p.PauseSeconds!=15&&p.PauseSeconds!=30&&p.PauseSeconds!=60{p.PauseSeconds=30}
 alarmEnabled.Store(p.Enabled)
 alarmFull.Store(p.Full)
 alarmPauseSecs.Store(p.PauseSeconds)
}
func saveAlarmPreferences(){
 path:=alarmPreferencesPath()
 if path==""{return}
 if err:=os.MkdirAll(filepath.Dir(path),0700);err!=nil{logError(err);return}
 data,err:=json.Marshal(alarmPreferences{
  Enabled:alarmEnabled.Load(),
  Full:alarmFull.Load(),
  PauseSeconds:alarmPauseSecs.Load(),
 })
 if err!=nil{logError(err);return}
 tmp:=path+".tmp"
 if err=os.WriteFile(tmp,data,0600);err!=nil{logError(err);return}
 if err=os.Rename(tmp,path);err!=nil{logError(err);_ = os.Remove(tmp)}
}
