package server

import (
	"net/http"

	"github.com/bradhe/what-day-is-it/pkg/storage/managers"
	"github.com/bradhe/what-day-is-it/pkg/twilio"
	"github.com/gorilla/mux"
)

var DefaultTimeZone = "America/Los_Angeles"

type Server struct {
	DefaultTimeZone string

	managers managers.Managers
	server   *http.Server
	sender   *twilio.Sender
}

func (s *Server) ListenAndServe(addr string) error {
	s.server.Addr = addr
	logger.Infof("starting HTTP server on %s", addr)
	return s.server.ListenAndServe()
}

func NewServer(managers managers.Managers, sender *twilio.Sender, development bool) *Server {
	server := &Server{
		DefaultTimeZone: DefaultTimeZone,
		managers:        managers,
		sender:          sender,
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
