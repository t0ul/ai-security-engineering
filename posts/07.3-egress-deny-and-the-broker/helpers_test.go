package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type servers struct{ ok, internal string }

func startServers(t *testing.T) servers {
	okS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "ALLOWED: this week's weather")
	}))
	t.Cleanup(okS.Close)
	inS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		internalHits++
		io.WriteString(w, "INTERNAL-SECRET")
	}))
	t.Cleanup(inS.Close)
	return servers{ok: "weather.example:" + port(okS.URL), internal: "internal.example:" + port(inS.URL)}
}
