package util

import "time"

// TimeNow returns the current time UTC with millisecond precision
func TimeNow() time.Time {
	return time.Now().UTC().Truncate(time.Millisecond)
}
