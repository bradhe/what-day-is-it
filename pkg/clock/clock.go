package clock

import "time"

type ClockFunc func() *time.Time

var Clock ClockFunc = clock

func clock() *time.Time {
	t := time.Now()
	return &t
}
