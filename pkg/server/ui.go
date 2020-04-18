package server

import (
	"io/ioutil"
	"net/http"

	"github.com/bradhe/what-day-is-it/pkg/ui"
)

func (s *Server) GetFile(name string, useCompiled bool) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if useCompiled {
			w.Write(ui.MustAsset("assets/" + name))
		} else {
			buf, err := ioutil.ReadFile("pkg/ui/dist/" + name)

			if err != nil {
				panic(err)
			}

			w.Write(buf)
		}
	}
}
