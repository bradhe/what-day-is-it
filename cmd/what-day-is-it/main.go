package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bradhe/stopwatch"
	"github.com/bradhe/what-day-is-it/pkg/logs"
	"github.com/bradhe/what-day-is-it/pkg/models"
	"github.com/bradhe/what-day-is-it/pkg/storage"
	"github.com/bradhe/what-day-is-it/pkg/storage/managers"
	"github.com/bradhe/what-day-is-it/pkg/twilio"
	"github.com/bradhe/what-day-is-it/pkg/ui"
	"github.com/gorilla/mux"
)

var logger = logs.WithPackage("main")

const DefaultTimeZone = "America/Los_Angeles"

type ClockFunc func() *time.Time

var Clock ClockFunc = clock

func doSendMessages(managers managers.Managers, sender *twilio.Sender) {
	manager := managers.PhoneNumbers()

	for range time.Tick(15 * time.Minute) {
		logger.Info("starting delivery run")

		for {
			numbers, err := manager.GetNBySendDeadline(10, Clock())

			if err != nil {
				logger.WithError(err).Error("failed to lookup batch for delivery")
				break
			}

			if len(numbers) < 1 {
				break
			}

			for _, number := range numbers {
				if !number.IsSendable {
					logger.Debug("skipping unsendable number")

					// Update this anyway so we don't check it again for a while.
					manager.UpdateSkipped(&number, clock())

					continue
				}

				body := fmt.Sprintf("Today is %s", GetDayInZone(MustLoadLocation(number.Timezone)))

				if err := sender.Send(number.Number, body); err != nil {
					logger.WithError(err).Warn("failed to deliver message via Twilio")
				}

				// We'll finish this for the day.
				manager.UpdateSent(&number, clock())
			}
		}
	}
}

type Server struct {
	managers managers.Managers
	server   *http.Server
	sender   *twilio.Sender
}

func (s *Server) ListenAndServe(addr string) error {
	s.server.Addr = addr
	logger.Infof("starting HTTP server on %s", addr)
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
		logger.WithField("requested_timezone", str).WithField("default_timezone", DefaultTimeZone).Info("railed to load timezone")
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

func isStopMessage(str string) bool {
	return strings.ToLower(strings.TrimSpace(str)) == "stop"
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

	if num := CleanPhoneNumber(req.Number); !IsCleanPhoneNumber(num) {
		logger.Error("invalid phone number")
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
			w.Write(ui.MustAsset("assets/" + name))
		} else {
			buf, err := ioutil.ReadFile("pkg/ui/assets/" + name)

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
	logger.Info("checking health")
	w.Write(Dump(GetHealthResponse{true}))
}

type loggingResponseWriter struct {
	http.ResponseWriter
	StatusCode int
	Bytes      int64
}

func (w *loggingResponseWriter) WriteHeader(code int) {
	w.StatusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *loggingResponseWriter) Write(buf []byte) (int, error) {
	// this is the implicit status code unless one has been explicitly written.
	if w.StatusCode == 0 {
		w.StatusCode = http.StatusOK
	}

	n, err := w.ResponseWriter.Write(buf)
	w.Bytes += int64(n)
	return n, err
}

func newLoggingResponseWriter(base http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{base, 0, 0}
}

func bytes(arr ...int64) uint64 {
	var acc uint64

	for _, b := range arr {
		if b > 0 {
			acc += uint64(b)
		}
	}

	return acc
}

func newRouteHandler(r *mux.Router) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		wrapper := newLoggingResponseWriter(w)

		defer stopwatch.Start().Timer(func(watch stopwatch.Watch) {
			logger.WithFields(map[string]interface{}{
				"status": wrapper.StatusCode,
				"bytes":  bytes(wrapper.Bytes, req.ContentLength),
				"time":   watch,
			}).Infof("served %s to %s", req.Method, req.RemoteAddr)
		})

		r.ServeHTTP(wrapper, req)
	})
}

func NewServer(managers managers.Managers, sender *twilio.Sender, development bool) *Server {
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
		Handler: newRouteHandler(r),
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

	sender := twilio.NewSender(*twilioAccountSid, *twilioAuthToken, *twilioPhoneNumber)
	managers := storage.New(*cloudformationStack)

	// Start the delivery loop off right!
	go doSendMessages(managers, &sender)

	if err := NewServer(managers, &sender, *development).ListenAndServe(*addr); err != nil {
		panic(err)
	}
}
