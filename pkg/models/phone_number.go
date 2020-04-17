package models

import "time"

type PhoneNumber struct {
	Number       string
	Timezone     string
	LastSentAt   *time.Time
	IsSendable   bool
	SendDeadline *time.Time
}
