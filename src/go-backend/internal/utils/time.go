package utils

import "time"

func TimeToSeconds(t time.Time) int {
	return t.Hour()*3600 + t.Minute()*60 + t.Second()
}
