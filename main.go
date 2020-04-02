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

const DefaultTimeZone = "Pacific/Los_Angeles"

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

func GetDayInZone(loc *time.Location) string {
	return time.Now().In(loc).Format("Monday")
}

func (s Sender) Send(to, body string) error {
	urlStr := "https://api.twilio.com/2010-04-01/Accounts/" + s.accountSID + "/Messages.json"

	msgData := url.Values{}
	msgData.Set("To", "+15412311514")
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

func MustLoadLocation(name string) *time.Location {
	if loc, err := time.LoadLocation("UTC"); err != nil {
		panic(err)
	} else {
		return loc
	}
}

func clock() *time.Time {
	t := time.Now()
	return &t
}

func doSendMessages(managers storage.Managers, sender *Sender) {
	manager := managers.PhoneNumbers()

	for range time.Tick(15 * time.Minute) {
		log.Printf("Starting delivery run.")

		for {
			numbers, err := manager.GetNBySendDeadline(10, clock())

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
}

func (s *Server) ListenAndServe(addr string) error {
	s.server.Addr = addr
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

func (s *Server) PostSubscribe(w http.ResponseWriter, r *http.Request) {
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

		if err := s.managers.PhoneNumbers().Save(phoneNumber); err != nil {
			log.Printf("[Server] ERR failed to save phone number: %s", err.Error())
			w.WriteHeader(http.StatusInternalServerError)

			resp.Subscribed = false
			resp.Error = "An internal error occured."

			w.Write(Dump(resp))

			return
		} else {
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

func NewServer(managers storage.Managers, development bool) *Server {
	server := &Server{
		managers: managers,
	}

	r := mux.NewRouter()
	r.HandleFunc("/api/subscribe", server.PostSubscribe)

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

	if err := NewServer(managers, *development).ListenAndServe(*addr); err != nil {
		panic(err)
	}
}
