package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/bradhe/what-day-is-it/pkg/clock"
	"github.com/bradhe/what-day-is-it/pkg/models"
	"github.com/bradhe/what-day-is-it/pkg/storage/managers"
	"github.com/bradhe/what-day-is-it/pkg/twilio"
)

type IncomingMessageRequest struct {
	AccountSID string `json:"account_sid"`
	From       string `json:"from"`
	Body       string `json:"body"`
}

func (s *Server) PostIncomingMessage(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	logger.Infof("processing Twilio webhook")

	var req IncomingMessageRequest

	buf, _ := ioutil.ReadAll(r.Body)

	if vals, err := url.ParseQuery(string(buf)); err != nil {
		logger.WithError(err).Error("failed to decode Twilio webhook request")
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else {
		req.From = vals.Get("From")
		req.Body = vals.Get("Body")
		req.AccountSID = vals.Get("AccountSid")
	}

	if isStopMessage(req.Body) {
		if phoneNumber, err := s.managers.PhoneNumbers().Get(req.From); err != nil {
			logger.WithError(err).Error("failed to find phone number associated with Twilio webhook request")
			w.WriteHeader(http.StatusInternalServerError)
			return
		} else {
			// We update this record to mark it as not sendable.
			if err := s.managers.PhoneNumbers().UpdateNotSendable(&phoneNumber); err != nil {
				logger.WithError(err).Error("failed to update record as not sendable")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		// We can reply to the user now that they've been dropped.
		logger.Info("user unsubscribed")
		w.Write(twilio.TwiMLResponse(`Okay, I'll stop reminding you starting...NOW!`))
	} else {
		logger.Infof("unknown request from user: `%s`", req.Body)
		w.Write(twilio.TwiMLResponse(`You do know you're talking to a robot right?`))
	}
}

type PostSubscribeRequest struct {
	// The phone number to establish a subscription to.
	Number string `json:"number"`

	// The time zone that the user selected.
	Timezone string `json:"timezone"`
}

type PostSubscribeResponse struct {
	Number string `json:"number,omitempty"`

	Timezone string `json:"timezone,omitempty"`

	Error string `json:"error,omitempty"`

	Subscribed bool `json:"subscribed"`
}

func (s *Server) PostSubscribe(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var req PostSubscribeRequest
	var resp PostSubscribeResponse

	logger.Info("handling subscribe request")

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.WithError(err).Error("failed to decode request body")
		w.WriteHeader(http.StatusBadRequest)

		resp.Subscribed = false
		resp.Error = "Failed to read the subscribe request. Did you send it as JSON?"
		w.Write(Dump(resp))

		return
	}

	if num := models.CleanPhoneNumber(req.Number); !models.IsCleanPhoneNumber(num) {
		logger.Error("invalid phone number")
		w.WriteHeader(http.StatusPreconditionFailed)

		resp.Subscribed = false
		resp.Error = "Invalid phone number."

		w.Write(Dump(resp))
	} else {
		phoneNumber := models.PhoneNumber{
			Number:   num,
			Timezone: s.timezoneOrDefault(req.Timezone),

			// They just signed up so we'll always be sendable in this case.
			IsSendable: true,
		}

		if err := s.managers.PhoneNumbers().Create(phoneNumber); err != nil {
			if err == managers.ErrRecordExists {
				// TODO: They resubscribed so we should update their record I guess.

				resp.Number = num
				resp.Timezone = phoneNumber.Timezone
				resp.Subscribed = true
				resp.Error = ""

				w.Write(Dump(resp))
			} else {
				logger.WithError(err).Error("failed to save phone number")
				w.WriteHeader(http.StatusInternalServerError)

				resp.Subscribed = false
				resp.Error = "An internal error occured."

				w.Write(Dump(resp))
			}
		} else {
			s.sender.Send(num, "Yo! Okay, every morning I'll text you what day it is. Just say STOP to make me stop.")
			s.sender.Send(num, fmt.Sprintf("Today is %s by the way.", clock.GetDayInZone(clock.MustLoadLocation(phoneNumber.Timezone))))

			// We'll update this record so we don't send something again later...
			s.managers.PhoneNumbers().UpdateSent(&phoneNumber, clock.Clock())

			resp.Number = num
			resp.Timezone = phoneNumber.Timezone
			resp.Subscribed = true
			resp.Error = ""

			w.Write(Dump(resp))
		}
	}
}

type GetHealthResponse struct {
	OK bool `json:"ok"`
}

func (s *Server) GetHealth(w http.ResponseWriter, r *http.Request) {
	logger.Info("checking health")
	w.Write(Dump(GetHealthResponse{true}))
}
