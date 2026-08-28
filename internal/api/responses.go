package api

import "time"

func truncateTime(t time.Time) time.Time {
	return t.Truncate(time.Second)
}
