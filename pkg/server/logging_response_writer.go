package server

import "net/http"

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
