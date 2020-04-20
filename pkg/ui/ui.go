//go:generate bash ./build.sh
//go:generate go-bindata -pkg=ui -prefix=build -o=ui.gen.go -ignore=\.swp ./dist/...
package ui

import (
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
)

const DefaultBasedir = "pkg/ui/dist"

const AssetPrefix = "dist"

type Handler struct {
	// Indicates that we should serve assets off disk, basically, instead of from memory.
	DevelopmentMode bool

	// The main directory that this content will come out of in development mode.
	Basedir string
}

func getContentType(assetName string) string {
	return mime.TypeByExtension(path.Ext(assetName))
}

func (h Handler) isFileSystemAsset(name string) bool {
	info, err := os.Stat(h.Basedir + name)

	if os.IsNotExist(err) {
		return false
	}

	if info.IsDir() {
		return false
	}

	return true
}

func (h Handler) getFileSystemAsset(name string) ([]byte, error) {
	return ioutil.ReadFile(h.Basedir + name)
}

func (h Handler) isMemoryAsset(name string) bool {
	// TODO: Is there a better way to check to see if a compiled resouce exists
	// without having to reach in to the private implementation?
	_, ok := _bindata[AssetPrefix+name]
	return ok
}

func (h Handler) writeAsset(w http.ResponseWriter, buf []byte, name string) {
	w.Header().Set("Content-Type", getContentType(name))

	// TODO: Should we gzip this content?
	w.Write(buf)
}

func (h Handler) IsAssetRequest(r *http.Request) bool {
	assetName := getAssetName(r.URL)

	if h.DevelopmentMode {
		// Check to see if this file is on disk...
		return h.isFileSystemAsset(assetName)
	} else {
		return h.isMemoryAsset(assetName)
	}
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	assetName := getAssetName(r.URL)

	if h.DevelopmentMode {
		buf, err := h.getFileSystemAsset(assetName)

		// TODO: We should find a better way to determine what failed here. If the
		// asset is not found for whatever reason, we should render a 404.
		if err != nil {
			logger.WithError(err).Infof("failed to render asset `%s` in development mode", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		} else {
			logger.WithField("asset", assetName).Debugf("serving filesystem asset asset")
			h.writeAsset(w, buf, assetName)
		}
	} else {
		buf, err := Asset(AssetPrefix + assetName)

		if err != nil {
			logger.WithError(err).Infof("failed to render asset `%s` in non-development mode", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		} else {
			h.writeAsset(w, buf, assetName)
		}
	}
}

func NewHandler(development bool) Handler {
	return Handler{
		DevelopmentMode: development,
		Basedir:         DefaultBasedir,
	}
}

func getAssetName(url *url.URL) string {
	assetName := url.Path

	switch assetName {
	case "/":
		return "/index.html"
	default:
		return assetName
	}
}
