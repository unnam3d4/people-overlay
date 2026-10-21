package main

import "time"

func frameCadence(profile int32, hasPeople, fast bool) time.Duration {
	if !hasPeople {
		if profile == 1 {
			return 440 * time.Millisecond
		}
		return 300 * time.Millisecond
	}
	switch profile {
	case 1:
		return 340 * time.Millisecond
	case 3:
		if fast {
			return 66 * time.Millisecond
		}
		return 100 * time.Millisecond
	default:
		return 210 * time.Millisecond
	}
}

// Optional benchmark hook. Older baseline sources retain their own cadence.
func (t *personTracker) captureCadence(profile int32) time.Duration {
	return frameCadence(profile, len(t.people) > 0, false)
}
