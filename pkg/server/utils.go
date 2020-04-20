package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/bradhe/stopwatch"
)

func newLoggedHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		wrapper := newLoggingResponseWriter(w)

		defer stopwatch.Start().Timer(func(watch stopwatch.Watch) {
			logger.WithFields(map[string]interface{}{
				"status": wrapper.StatusCode,
				"bytes":  bytes(wrapper.Bytes, req.ContentLength),
				"time":   watch,
			}).Infof("served %s %s to %s", req.Method, req.URL.Path, req.RemoteAddr)
		})

		h.ServeHTTP(wrapper, req)
	})
}

func isStopMessage(str string) bool {
	return strings.ToLower(strings.TrimSpace(str)) == "stop"
}

func Dump(obj interface{}) []byte {
	if buf, err := json.Marshal(obj); err != nil {
		panic(err)
	} else {
		return buf
	}
}

func (s *Server) timezoneOrDefault(str string) string {
	if str == "" {
		return s.DefaultTimeZone
	}

	// Let's try to load this timezone. If it fails we'll just use the default timezone.
	if _, err := time.LoadLocation(str); err != nil {
		logger.WithField("requested_timezone", str).WithField("default_timezone", s.DefaultTimeZone).Info("railed to load timezone")
		return s.DefaultTimeZone
	} else {
		return str
	}
}
