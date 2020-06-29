package rehook

import (
	"bytes"
	"go.uber.org/zap"
	"io/ioutil"
	"net/http"
	"net/url"
)

type dumpedreq struct {
	Headers    http.Header `json:"headers"`
	Method     string
	URL        *url.URL
	Body       string
	Host       string
	RemoteAddr string
	RequestURI string
}

func (self *Server) getDumpedRequest(w http.ResponseWriter, r *http.Request, body bool) *dumpedreq {
	dump := &dumpedreq{
		Headers:    r.Header,
		Method:     r.Method,
		URL:        r.URL,
		Host:       r.Host,
		RemoteAddr: r.RemoteAddr,
		RequestURI: r.RequestURI,
	}

	if body {
		data, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			self.logger.Error("failed read request body", zap.Any("req", dump), zap.Error(err))
			return nil
		}

		dump.Body = string(data)
		r.Body = ioutil.NopCloser(bytes.NewReader(data))
	}

	return dump
}
