package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// scriptedRuntime is a RuntimeControl whose behavior the test scripts, to exercise the
// HTTP wiring (the interface is the Config seam; the real controller's own lifecycle is
// tested in cmd/webapp).
type scriptedRuntime struct {
	running  bool
	startErr error
}

func (s *scriptedRuntime) StartModels() error {
	if s.startErr != nil {
		return s.startErr
	}
	s.running = true
	return nil
}
func (s *scriptedRuntime) StopModels() error { s.running = false; return nil }
func (s *scriptedRuntime) ModelsRunning() bool { return s.running }

// TestRuntimeControlHandlers locks the C3 wiring: start/stop drive the controller and the
// runtime status reports running; a start error surfaces as ok:false, not a 500.
func TestRuntimeControlHandlers(t *testing.T) {
	rt := &scriptedRuntime{}
	srv := httptest.NewServer(New(Config{Runtime: rt}).Handler())
	defer srv.Close()

	post := func(path string) map[string]any {
		resp, err := http.Post(srv.URL+path, "application/json", strings.NewReader("{}"))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var m map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&m)
		return m
	}
	controlRunning := func() bool {
		resp, err := http.Get(srv.URL + "/api/runtime")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var d struct {
			Control struct {
				Available bool `json:"available"`
				Running   bool `json:"running"`
			} `json:"control"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&d)
		if !d.Control.Available {
			t.Fatal("runtime status must advertise control availability")
		}
		return d.Control.Running
	}

	if controlRunning() {
		t.Fatal("precondition: not running")
	}
	if r := post("/api/runtime/start"); r["ok"] != true || !controlRunning() {
		t.Fatalf("start must bring the stack up, got %v", r)
	}
	if r := post("/api/runtime/stop"); r["ok"] != true || controlRunning() {
		t.Fatalf("stop must bring it down, got %v", r)
	}

	// A start error surfaces as ok:false (not a hard 500).
	rt.startErr = errors.New("no models available")
	if r := post("/api/runtime/start"); r["ok"] == true {
		t.Fatalf("a start error must report ok:false, got %v", r)
	}
}
