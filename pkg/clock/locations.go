package clock

import "time"

func MustLoadLocation(name string) *time.Location {
	if loc, err := time.LoadLocation(name); err != nil {
		panic(err)
	} else {
		return loc
	}
}

func GetDayInZone(loc *time.Location) string {
	return Clock().In(loc).Format("Monday")
}
