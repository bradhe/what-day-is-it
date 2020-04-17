package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bradhe/what-day-is-it/models"
	"github.com/bradhe/what-day-is-it/storage"
	"github.com/bradhe/what-day-is-it/ui"
	"github.com/gorilla/mux"
)

const DefaultTimeZone = "America/Los_Angeles"

type ClockFunc func() *time.Time

var Clock ClockFunc = clock

type Sender struct {
	accountSID string
	authToken  string
	fromNumber string
}

func NewSender(accountSID, authToken, fromNumber string) Sender {
	return Sender{
		accountSID,
		authToken,
		fromNumber,
	}
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
		log.Printf("ERR failed to send request to Twilio: %s", err.Error())
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var data twilioResponse

		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			log.Printf("ERR failed to parse Twilio response: %s", err.Error())
		} else {
			log.Printf("message `%s` delivered", data.SID)
		}
	} else {
		log.Printf("ERR Twilio response code: `%s`", resp.Status)

		buf, _ := ioutil.ReadAll(resp.Body)
		log.Printf("ERR body: `%s`", string(buf))
	}

	return nil
}

func doSendMessages(managers storage.Managers, sender *Sender) {
	manager := managers.PhoneNumbers()

	for range time.Tick(15 * time.Minute) {
		log.Printf("Starting delivery run.")

		for {
			numbers, err := manager.GetNBySendDeadline(10, Clock())

			if err != nil {
				log.Printf("ERR failed to lookup batch for delivery: %s", err.Error())
				break
			}

			if len(numbers) < 1 {
				break
			}

			for _, number := range numbers {
				if !number.IsSendable {
					log.Printf("skipping unsendable number")

					// Update this anyway so we don't check it again for a while.
					manager.UpdateSkipped(&number, clock())

					continue
				}

				body := fmt.Sprintf("Today is %s", GetDayInZone(MustLoadLocation(number.Timezone)))

				if err := sender.Send(number.Number, body); err != nil {
					log.Printf("WARN failed to deliver message: %s", err.Error())
				}

				// We'll finish this for the day.
				manager.UpdateSent(&number, clock())
			}
		}
	}
}

type Server struct {
	managers storage.Managers
	server   *http.Server
	sender   *Sender
}

func (s *Server) ListenAndServe(addr string) error {
	s.server.Addr = addr
	log.Printf("[Server] starting HTTP server on %s", addr)
	return s.server.ListenAndServe()
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

func timezoneOrDefault(str string) string {
	if str == "" {
		return DefaultTimeZone
	}

	// Let's try to load this timezone. If it fails we'll just use the default timezone.
	if _, err := time.LoadLocation(str); err != nil {
		log.Printf("failed to load timezone `%s` so using default `%s` instead", str, DefaultTimeZone)
		return DefaultTimeZone
	} else {
		return str
	}
}

func Dump(obj interface{}) []byte {
	if buf, err := json.Marshal(obj); err != nil {
		panic(err)
	} else {
		return buf
	}
}

type IncomingMessageRequest struct {
	AccountSID string `json:"account_sid"`
	From       string `json:"from"`
	Body       string `json:"body"`
}

func getTwiMLSMS(message string) []byte {
	return []byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><Response><Message><Body>%s</Body></Message></Response>`, message))
}

func isStopMessage(str string) bool {
	return strings.ToLower(strings.TrimSpace(str)) == "stop"
}

func (s *Server) PostIncomingMessage(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	log.Printf("[Server] processing unsubscribe request from Twilio")

	var req IncomingMessageRequest

	buf, _ := ioutil.ReadAll(r.Body)

	if vals, err := url.ParseQuery(string(buf)); err != nil {
		log.Printf("[Server] ERR failed to decode Twilio webhook request. %s", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else {
		req.From = vals.Get("From")
		req.Body = vals.Get("Body")
		req.AccountSID = vals.Get("AccountSid")
	}

	if isStopMessage(req.Body) {
		if phoneNumber, err := s.managers.PhoneNumbers().Get(req.From); err != nil {
			log.Printf("[Server] ERR to find phone number associated with Twilio webhook request. %s", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		} else {
			// We update this record to mark it as not sendable.
			if err := s.managers.PhoneNumbers().UpdateNotSendable(&phoneNumber); err != nil {
				log.Printf("[Server] ERR failed to update record as not sendable. %s", err.Error())
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		// We can reply to the user now that they've been dropped.
		log.Printf("[Server] user unsubscribed")
		w.Write(getTwiMLSMS(`Okay, I'll stop reminding you starting...NOW!`))
	} else {
		log.Printf("[Server] not sure what user wants: `%s`", req.Body)
		w.Write(getTwiMLSMS(`You do know you're talking to a robot right?`))
	}
}

func (s *Server) PostSubscribe(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var req PostSubscribeRequest
	var resp PostSubscribeResponse

	log.Printf("[Server] handling post")

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[Server] ERR failed to decode body: %s", err.Error())
		w.WriteHeader(http.StatusBadRequest)

		resp.Subscribed = false
		resp.Error = "Failed to read the subscribe request. Did you send it as JSON?"
		w.Write(Dump(resp))

		return
	}

	if num := CleanPhoneNumber(req.Number); !IsCleanPhoneNumber(num) {
		log.Printf("[Server] ERR invalid phone number")
		w.WriteHeader(http.StatusPreconditionFailed)

		resp.Subscribed = false
		resp.Error = "Invalid phone number."

		w.Write(Dump(resp))
	} else {
		phoneNumber := models.PhoneNumber{
			Number:   num,
			Timezone: timezoneOrDefault(req.Timezone),

			// They just signed up so we'll always be sendable in this case.
			IsSendable: true,
		}

		if err := s.managers.PhoneNumbers().Create(phoneNumber); err != nil {
			if err == storage.ErrRecordExists {
				// TODO: They resubscribed so we should update their record I guess.

				resp.Number = num
				resp.Timezone = phoneNumber.Timezone
				resp.Subscribed = true
				resp.Error = ""

				w.Write(Dump(resp))
			} else {
				log.Printf("[Server] ERR failed to save phone number: %s", err.Error())
				w.WriteHeader(http.StatusInternalServerError)

				resp.Subscribed = false
				resp.Error = "An internal error occured."

				w.Write(Dump(resp))
			}
		} else {
			s.sender.Send(num, "Yo! Okay, every morning I'll text you what day it is. Just say STOP to make me stop.")
			s.sender.Send(num, fmt.Sprintf("Today is %s by the way.", GetDayInZone(MustLoadLocation(phoneNumber.Timezone))))

			// We'll update this record so we don't send something again later...
			s.managers.PhoneNumbers().UpdateSent(&phoneNumber, clock())

			resp.Number = num
			resp.Timezone = phoneNumber.Timezone
			resp.Subscribed = true
			resp.Error = ""

			w.Write(Dump(resp))
		}
	}
}

func (s *Server) GetFile(name string, useCompiled bool) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if useCompiled {
			w.Write(ui.MustAsset("assets/index.html"))
		} else {
			buf, err := ioutil.ReadFile("ui/assets/index.html")

			if err != nil {
				panic(err)
			}

			w.Write(buf)
		}
	}
}

type GetHealthResponse struct {
	OK bool `json:"ok"`
}

func (s *Server) GetHealth(w http.ResponseWriter, r *http.Request) {
	log.Printf("[Server] checking health")
	w.Write(Dump(GetHealthResponse{true}))
}

func NewServer(managers storage.Managers, sender *Sender, development bool) *Server {
	server := &Server{
		managers: managers,
		sender:   sender,
	}

	r := mux.NewRouter()
	r.HandleFunc("/api/health", server.GetHealth)
	r.HandleFunc("/api/subscribe", server.PostSubscribe)
	r.HandleFunc("/api/incoming-message", server.PostIncomingMessage)

	// The only static assets taht we have will be loaded out of memory in production.
	r.HandleFunc("/index.html", server.GetFile("index.html", !development))
	r.HandleFunc("/", server.GetFile("index.html", !development))

	base := &http.Server{
		Handler: r,
	}

	server.server = base
	return server
}

func main() {
	var (
		development         = flag.Bool("development", false, "Put the app in development mode. Basically load UI assets from disk instead of memory.")
		twilioAccountSid    = flag.String("twilio-account-sid", "", "The account SID to authenticate with.")
		twilioAuthToken     = flag.String("twilio-auth-token", "", "The Twilio authentication token to authenticate with.")
		twilioPhoneNumber   = flag.String("twilio-phone-number", "", "The Twilio phone number to use when sending messages.")
		cloudformationStack = flag.String("cloudformation-stack", "what-day-is-it-1", "The stack that we want to store data in.")
		addr                = flag.String("addr", "localhost:8081", "Address to bind the server to.")
	)

	flag.Parse()

	sender := NewSender(*twilioAccountSid, *twilioAuthToken, *twilioPhoneNumber)
	managers := storage.New(*cloudformationStack)

	// Start the delivery loop off right!
	go doSendMessages(managers, &sender)

	if err := NewServer(managers, &sender, *development).ListenAndServe(*addr); err != nil {
		panic(err)
	}
}
