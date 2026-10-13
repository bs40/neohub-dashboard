// Package server is the HTTP side of the dashboard: the JSON API the front-end talks to,
// the static files, the status cache and the connectivity poller.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bs40/neohub-dashboard/internal/history"
	"github.com/bs40/neohub-dashboard/internal/hub"
)

// Options wires the server to its collaborators.
type Options struct {
	Hub          *hub.Hub
	History      *history.Store
	Public       fs.FS // the dashboard files (index.html, app.js, ...)
	Img          fs.FS // background photos, served under /img/
	StatusCache  time.Duration
	PollInterval time.Duration
	HubInfo      func() HubInfo  // current discovery state for /api/hub
	Rediscover   func()          // asks the app to search for the hub again
	Addresses    func() []string // this machine's LAN IPv4 addresses, best first
	NameURL      string          // a friendly name address such as http://neohub.local/ (Pi), "" if none
	ListenPort   int
	Version      string
	Logger       *log.Logger
}

// HubInfo is what the UI can show about the hub connection.
type HubInfo struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Firmware  string `json:"firmware"`
	Searching bool   `json:"searching"`
	LastError string `json:"lastError,omitempty"`
	LastFound string `json:"lastFound,omitempty"`
}

// Server is the HTTP handler plus its background poller.
type Server struct {
	opt    Options
	mux    *http.ServeMux
	cache  statusCache
	ctx    context.Context
	cancel context.CancelFunc
}

type statusCache struct {
	mu      sync.Mutex
	at      time.Time
	data    map[string]json.RawMessage
	live    liveData
	system  systemData
	pending *sync.WaitGroup
}

// liveData / systemData are the few fields the poller needs from GET_LIVE_DATA / GET_SYSTEM.
type liveData struct {
	Devices []struct {
		ZoneName   string `json:"ZONE_NAME"`
		DeviceID   int    `json:"DEVICE_ID"`
		Offline    bool   `json:"OFFLINE"`
		LowBattery bool   `json:"LOW_BATTERY"`
	} `json:"devices"`
}
type systemData struct {
	NtpOn string  `json:"NTP_ON"`
	UTC   float64 `json:"UTC"`
}

// New builds the server. Call Run to start the poller, and use it as an http.Handler.
func New(opt Options) *Server {
	if opt.Logger == nil {
		opt.Logger = log.Default()
	}
	if opt.StatusCache == 0 {
		opt.StatusCache = 4 * time.Second
	}
	if opt.PollInterval == 0 {
		opt.PollInterval = time.Minute
	}
	_ = mime.AddExtensionType(".webmanifest", "application/manifest+json")
	s := &Server{opt: opt, mux: http.NewServeMux()}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.routes()
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

// Run starts the connectivity poller until Stop is called.
func (s *Server) Run() {
	s.opt.History.Note("dashboard started")
	// Give discovery a moment to find the hub before the first poll, so a fresh start does
	// not log a spurious "hub unreachable".
	for i := 0; i < 60 && s.opt.Hub.Host() == ""; i++ {
		select {
		case <-s.ctx.Done():
			return
		case <-time.After(500 * time.Millisecond):
		}
	}
	s.pollOnce(true)
	t := time.NewTicker(s.opt.PollInterval)
	defer t.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-t.C:
			s.pollOnce(false)
		}
	}
}

// Stop ends the poller and writes the closing note.
func (s *Server) Stop() {
	s.cancel()
	s.opt.History.Note("dashboard stopped")
}

// ---------- status ----------

var statusCommands = []struct{ key, cmd string }{
	{"live", "GET_LIVE_DATA"}, {"system", "GET_SYSTEM"}, {"engineers", "GET_ENGINEERS"}, {"profiles", "GET_PROFILES"},
	{"holiday", "GET_HOLIDAY"}, {"firmware", "FIRMWARE"}, {"zones", "GET_ZONES"}, {"serials", "DEVICES_SN"},
}

func (s *Server) getStatus(force bool) (map[string]json.RawMessage, liveData, systemData, error) {
	c := &s.cache
	c.mu.Lock()
	if !force && c.data != nil && time.Since(c.at) < s.opt.StatusCache {
		d, l, sy := c.data, c.live, c.system
		c.mu.Unlock()
		return d, l, sy, nil
	}
	if c.pending != nil { // someone else is already fetching: wait for them
		wg := c.pending
		c.mu.Unlock()
		wg.Wait()
		c.mu.Lock()
		d, l, sy := c.data, c.live, c.system
		c.mu.Unlock()
		if d == nil {
			return nil, liveData{}, systemData{}, errors.New("hub unreachable")
		}
		return d, l, sy, nil
	}
	wg := &sync.WaitGroup{}
	wg.Add(1)
	c.pending = wg
	c.mu.Unlock()

	data := map[string]json.RawMessage{}
	var firstErr error
	for _, sc := range statusCommands {
		reply, err := s.opt.Hub.Send(map[string]any{sc.cmd: 0})
		if err != nil {
			firstErr = err
			break
		}
		data[sc.key] = reply
	}
	var live liveData
	var system systemData
	if firstErr == nil {
		_ = json.Unmarshal(data["live"], &live)
		_ = json.Unmarshal(data["system"], &system)
		now := time.Now()
		data["fetchedAt"], _ = json.Marshal(now.UTC().Format(time.RFC3339))
		data["serverTime"], _ = json.Marshal(now.UnixMilli())
	}

	c.mu.Lock()
	c.pending = nil
	wg.Done()
	if firstErr != nil {
		c.mu.Unlock()
		return nil, liveData{}, systemData{}, firstErr
	}
	c.data, c.live, c.system, c.at = data, live, system, time.Now()
	c.mu.Unlock()
	return data, live, system, nil
}

func (s *Server) invalidate() {
	s.cache.mu.Lock()
	s.cache.at = time.Time{}
	s.cache.mu.Unlock()
}

func (s *Server) pollOnce(force bool) {
	now := time.Now().UnixMilli()
	_, live, system, err := s.getStatus(true)
	if err != nil {
		f := false
		s.opt.History.Record(history.Snapshot{T: now, Hub: &f, Err: clip(err.Error(), 120), Zones: s.opt.History.LastZones()}, force)
		return
	}
	t := true
	snap := history.Snapshot{T: now, Hub: &t, Ntp: system.NtpOn, Drift: int(system.UTC - float64(now)/1000), Off: []string{}, Low: []string{}, Zones: []string{}}
	devs := live.Devices
	sort.Slice(devs, func(i, j int) bool { return devs[i].DeviceID < devs[j].DeviceID })
	for _, d := range devs {
		snap.Zones = append(snap.Zones, d.ZoneName)
		if d.Offline {
			snap.Off = append(snap.Off, d.ZoneName)
		}
		if d.LowBattery {
			snap.Low = append(snap.Low, d.ZoneName)
		}
	}
	sort.Strings(snap.Off)
	sort.Strings(snap.Low)
	s.opt.History.Record(snap, force)
}

// ---------- routes ----------

func (s *Server) routes() {
	m := s.mux
	m.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
		data, _, _, err := s.getStatus(r.URL.Query().Has("force"))
		if err != nil {
			s.fail(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, data)
	})
	m.HandleFunc("GET /api/zone", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if name == "" {
			s.fail(w, http.StatusBadRequest, errors.New("name required"))
			return
		}
		out := map[string]json.RawMessage{}
		for key, cmd := range map[string]any{
			"profile0": map[string]any{"GET_PROFILE_0": name},
			"comfort":  map[string]any{"READ_COMFORT_LEVELS": []string{name}},
			"timer0":   map[string]any{"GET_TIMER_0": name},
			"roc":      map[string]any{"VIEW_ROC": []string{name}},
		} {
			reply, err := s.opt.Hub.Send(cmd)
			if err != nil {
				s.fail(w, http.StatusInternalServerError, err)
				return
			}
			out[key] = reply
		}
		writeJSON(w, http.StatusOK, out)
	})
	m.HandleFunc("GET /api/templog", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if name == "" {
			s.fail(w, http.StatusBadRequest, errors.New("name required"))
			return
		}
		reply, err := s.opt.Hub.Send(map[string]any{"GET_TEMPLOG": []string{name}})
		if err != nil {
			s.fail(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, reply)
	})
	m.HandleFunc("GET /api/log", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, s.opt.Hub.Log()) })
	m.HandleFunc("GET /api/config", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"hubHost": s.opt.Hub.Host(), "hubPort": s.opt.Hub.Port(), "version": s.opt.Version})
	})
	m.HandleFunc("GET /api/hub", func(w http.ResponseWriter, r *http.Request) {
		if s.opt.HubInfo != nil {
			writeJSON(w, http.StatusOK, s.opt.HubInfo())
			return
		}
		writeJSON(w, http.StatusOK, HubInfo{Host: s.opt.Hub.Host(), Port: s.opt.Hub.Port()})
	})
	m.HandleFunc("GET /api/history", func(w http.ResponseWriter, r *http.Request) {
		since, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
		writeJSON(w, http.StatusOK, map[string]any{
			"now": time.Now().UnixMilli(), "pollMs": s.opt.PollInterval.Milliseconds(),
			"heartbeatMs": 10 * 60 * 1000, "snapshots": s.opt.History.Since(since),
		})
	})
	m.HandleFunc("POST /api/action", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			s.fail(w, http.StatusBadRequest, err)
			return
		}
		var p map[string]any
		if len(body) > 0 {
			if err := json.Unmarshal(body, &p); err != nil {
				s.fail(w, http.StatusBadRequest, fmt.Errorf("bad JSON: %w", err))
				return
			}
		}
		name, _ := p["action"].(string)
		if name == "rediscover" {
			if s.opt.Rediscover != nil {
				s.opt.Rediscover()
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "result": "searching"})
			return
		}
		fn, ok := actions[name]
		if !ok {
			s.fail(w, http.StatusBadRequest, fmt.Errorf("unknown action %q", name))
			return
		}
		result, err := fn(s.opt.Hub, p)
		if err != nil {
			s.fail(w, http.StatusInternalServerError, err)
			return
		}
		s.invalidate()
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "result": result})
	})

	s.phoneRoutes()

	// Static files: photos under /img/, everything else from the public folder.
	imgFS := http.FileServer(http.FS(s.opt.Img))
	m.Handle("GET /img/", http.StripPrefix("/img/", cacheHeader(imgFS, "public, max-age=86400")))
	m.Handle("GET /", cacheHeader(http.FileServer(http.FS(s.opt.Public)), "no-cache"))
}

func cacheHeader(h http.Handler, value string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", value)
		h.ServeHTTP(w, r)
	})
}

func (s *Server) fail(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func clip(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return strings.TrimSpace(s)
}
