package main

import (
	"regexp"
	"time"
)

var phoneexp = regexp.MustCompile(`[^\+0-9]`)
var validexp = regexp.MustCompile(`\+[0-9]{11,13}`)

func clock() *time.Time {
	t := time.Now()
	return &t
}

func CleanPhoneNumber(number string) string {
	if len(number) < 1 {
		return ""
	}

	val := phoneexp.ReplaceAllString(number, "")

	// Nothing further to do here if there's a country code.
	if val[0] == '+' {
		return val
	}

	// If it's 10 characters then...
	switch len(val) {
	case 10:
		return "+1" + val
	case 11, 12, 13:
		return "+" + val
	default:
		// this indicates it's invalid.
		return number
	}
}

func IsCleanPhoneNumber(number string) bool {
	if len(number) < 1 {
		return false
	}

	return validexp.MatchString(number)
}

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
