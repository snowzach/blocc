package blocc

import (
	"time"
)

// ParseUnixTime will parse a unix timestamp and returns a pointer to time.Time or nil if time = 0
// If it's less than zero, it returns the relative time to now
func ParseUnixTime(t int64) *time.Time {
	var parsedTime time.Time
	if t == 0 {
		return nil
	} else if t < 0 {
		parsedTime = time.Unix(time.Now().UTC().Unix()+t, 0)
	} else {
		parsedTime = time.Unix(t, 0)
	}
	return &parsedTime
}
