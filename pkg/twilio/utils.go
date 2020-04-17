package twilio

import "fmt"

func TwiMLResponse(body string) []byte {
	return []byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><Response><Message><Body>%s</Body></Message></Response>`, body))
}
