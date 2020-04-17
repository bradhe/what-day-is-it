package twilio

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/bradhe/what-day-is-it/pkg/logs"
)

var logger = logs.WithPackage("twilio")

type Sender struct {
	accountSID string
	authToken  string
	fromNumber string
}

type twilioResponse struct {
	SID string `json:"sid"`
}

func (s Sender) Send(to, body string) error {
	urlStr := "https://api.twilio.com/2010-04-01/Accounts/" + s.accountSID + "/Messages.json"

	msgData := url.Values{}
	msgData.Set("To", to)
	msgData.Set("From", s.fromNumber)
	msgData.Set("Body", body)

	client := &http.Client{}
	req, _ := http.NewRequest("POST", urlStr, strings.NewReader(msgData.Encode()))
	req.SetBasicAuth(s.accountSID, s.authToken)

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)

	if err != nil {
		logger.WithError(err).Error("failed to send request to Twilio")
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var data twilioResponse

		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			logger.WithError(err).Error("failed to parse Twilio response")
		} else {
			logger.Infof("message `%s` delivered", data.SID)
		}
	} else {
		logger.Errorf("invalid twilio response code: %s", resp.Status)
	}

	return nil
}

func NewSender(accountSID, authToken, fromNumber string) Sender {
	return Sender{
		accountSID,
		authToken,
		fromNumber,
	}
}
