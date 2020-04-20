package server

import (
	"net/http"

	"github.com/bradhe/what-day-is-it/pkg/storage/managers"
	"github.com/bradhe/what-day-is-it/pkg/twilio"
	"github.com/bradhe/what-day-is-it/pkg/ui"
	"github.com/gorilla/mux"
)

var DefaultTimeZone = "America/Los_Angeles"

type Server struct {
	DefaultTimeZone string

	managers managers.Managers
	server   *http.Server
	sender   *twilio.Sender

	apiHandler http.Handler
	uiHandler  ui.Handler
}

func (s *Server) ListenAndServe(addr string) error {
	s.server.Addr = addr
	logger.Infof("starting HTTP server on %s", addr)
	return s.server.ListenAndServe()
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.uiHandler.IsAssetRequest(r) {
		s.uiHandler.ServeHTTP(w, r)
	} else {
		s.apiHandler.ServeHTTP(w, r)
	}
}

func NewServer(managers managers.Managers, sender *twilio.Sender, development bool, assetBasedir string) *Server {
	server := &Server{
		DefaultTimeZone: DefaultTimeZone,
		managers:        managers,
		sender:          sender,
	}

	r := mux.NewRouter()
	r.HandleFunc("/api/health", server.GetHealth)
	r.HandleFunc("/api/subscribe", server.PostSubscribe)
	r.HandleFunc("/api/incoming-message", server.PostIncomingMessage)

	base := &http.Server{
		Handler: newLoggedHandler(server),
	}

	server.server = base
	server.apiHandler = r

	// TODO: Can this be cleaned up?? Maybe should be pushed in to `ui` package.
	server.uiHandler = ui.NewHandler(development)
	server.uiHandler.Basedir = assetBasedir

	return server
}
