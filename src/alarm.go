package main

import "time"

// Don't be noisy: one alert for a NEW independently confirmed person, not for
// every AI result, every movement, or a track whose ID was just renewed.
type alarmCandidate struct{seen time.Time; last time.Time}
type alarmGate struct{
 candidates map[int]alarmCandidate
 lastGlobal time.Time
 byZone map[int]time.Time
}
func (g *alarmGate)tick(t *personTracker,now time.Time,enabled bool,globalPause time.Duration)bool{
 if g.candidates==nil{g.candidates=make(map[int]alarmCandidate)}
 if g.byZone==nil{g.byZone=make(map[int]time.Time)}
 shouldPlay:=false
 for _,p:=range t.people{
  c,ok:=g.candidates[p.ID]
  if !ok{
   g.candidates[p.ID]=alarmCandidate{seen:now,last:now}
   continue
  }
  c.last=now
  g.candidates[p.ID]=c
  // 700 ms of persistence and either two consistent visual frames or one
  // high-confidence detection. This filters flickering false positives.
  verified:=p.LastVisual.After(c.seen.Add(250*time.Millisecond))||p.Box.Score>=.79
  if !verified||now.Sub(c.seen)<700*time.Millisecond{continue}
  if !enabled{continue}
  if !g.lastGlobal.IsZero()&&now.Sub(g.lastGlobal)<globalPause{continue}
  zone:=t.tileOf(p.Box)
  // Same video region can't fire alarm every time an unstable tracker ID
  // is recreated. A person walking there stays silent for 90 seconds.
  if !g.byZone[zone].IsZero()&&now.Sub(g.byZone[zone])<90*time.Second{continue}
  g.byZone[zone]=now
  g.lastGlobal=now
  shouldPlay=true
  // Important: never play more than one sound even if 30 people appear.
  break
 }
 // IDs can grow throughout a long-running CCTV session. Prune event records.
 for id,c:=range g.candidates{
  if now.Sub(c.last)>2*time.Minute{delete(g.candidates,id)}
 }
 return shouldPlay
}
