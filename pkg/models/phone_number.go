package models

import (
	"regexp"
	"time"
)

var phoneexp = regexp.MustCompile(`[^\+0-9]`)
var validexp = regexp.MustCompile(`\+[0-9]{11,13}`)

type PhoneNumber struct {
	Number       string
	Timezone     string
	LastSentAt   *time.Time
	IsSendable   bool
	SendDeadline *time.Time
}

func CleanPhoneNumber(number string) string {
	if len(number) < 1 {
		return ""
	}

	val := phoneexp.ReplaceAllString(number, "")

	if val == "" {
		return number
	}

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
