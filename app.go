// neoHub: finds a Heatmiser neoHub on the local network and serves the dashboard to the LAN.
// This file is the platform-neutral core; main_windows.go adds the tray icon and
// main_linux.go the headless service entry point.
package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/bs40/neohub-dashboard/internal/config"
	"github.com/bs40/neohub-dashboard/internal/discover"
	"github.com/bs40/neohub-dashboard/internal/history"
	"github.com/bs40/neohub-dashboard/internal/hub"
	"github.com/bs40/neohub-dashboard/internal/server"
)

// Version is set at build time with -ldflags "-X main.Version=1.2.3".
var Version = "dev"

//go:embed public
var publicFS embed.FS

//go:embed img
var imgFS embed.FS

type app struct {
	cfg  *config.Config
	hub  *hub.Hub
	srv  *server.Server
	http *http.Server
	log  *log.Logger
	port int

	mu        sync.Mutex
	searching bool
	firmware  string
	lastErr   string
	lastFound time.Time
	kick      chan struct{} // asks the discovery loop to search now

	cancel context.CancelFunc
}

// newApp wires everything up and starts listening. It returns an error if the port is
// taken (most likely because a copy is already running).
func newApp(cfg *config.Config, listenPort int, logger *log.Logger) (*app, error) {
	a := &app{cfg: cfg, log: logger, kick: make(chan struct{}, 1), port: listenPort}
	a.hub = hub.New(cfg.HubHost, cfg.HubPort, time.Duration(cfg.HubTimeoutMs)*time.Millisecond)
	hist := history.Open(filepath.Join(config.Dir(), "history.jsonl"), time.Duration(cfg.HistoryHeartbeatMs)*time.Millisecond)
	pub, _ := fs.Sub(publicFS, "public")
	img, _ := fs.Sub(imgFS, "img")
	a.srv = server.New(server.Options{
		Hub: a.hub, History: hist, Public: pub, Img: img,
		StatusCache:  time.Duration(cfg.StatusCacheMs) * time.Millisecond,
		PollInterval: time.Duration(cfg.HistoryPollMs) * time.Millisecond,
		HubInfo:      a.hubInfo, Rediscover: a.rediscover, Addresses: a.lanAddresses, ListenPort: listenPort,
		Version: Version, Logger: logger,
	})
	addr := net.JoinHostPort(cfg.ListenHost, fmt.Sprint(listenPort))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("cannot listen on %s: %w", addr, err)
	}
	a.http = &http.Server{Handler: a.srv, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := a.http.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Printf("http: %v", err)
		}
	}()
	logger.Printf("dashboard on http://localhost:%d", listenPort)
	return a, nil
}

// start runs discovery and the connectivity poller until shutdown.
func (a *app) start() {
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	go a.discoveryLoop(ctx)
	go a.srv.Run()
}

// waitForHub blocks until discovery has an address or the timeout passes.
func (a *app) waitForHub(timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for a.hub.Host() == "" && time.Now().Before(deadline) {
		time.Sleep(250 * time.Millisecond)
	}
}

func (a *app) shutdown() {
	if a.cancel != nil {
		a.cancel()
	}
	a.srv.Stop()
	ctx, c := context.WithTimeout(context.Background(), 2*time.Second)
	defer c()
	_ = a.http.Shutdown(ctx)
	a.log.Printf("stopped")
}

func (a *app) url() string { return fmt.Sprintf("http://localhost:%d/", a.port) }

// ---------- discovery ----------

// discoveryLoop keeps the hub address current: it searches at start if nothing is known,
// re-checks the known hub every minute, and sweeps again when it stops answering or when
// asked via rediscover().
func (a *app) discoveryLoop(ctx context.Context) {
	search := func() {
		a.mu.Lock()
		a.searching = true
		a.mu.Unlock()
		sctx, cancel := context.WithTimeout(ctx, 90*time.Second)
		r, err := discover.Find(sctx, a.cfg.HubHost, a.cfg.HubPort)
		cancel()
		a.mu.Lock()
		a.searching = false
		if err != nil {
			a.lastErr = err.Error()
			a.mu.Unlock()
			a.log.Printf("discovery: %v", err)
			return
		}
		a.lastErr, a.firmware, a.lastFound = "", r.Firmware, time.Now()
		a.mu.Unlock()
		if r.IP != a.hub.Host() {
			a.log.Printf("discovery: hub at %s (firmware %s)", r.IP, r.Firmware)
			a.hub.SetHost(r.IP)
			a.cfg.HubHost = r.IP
			if err := a.cfg.Save(); err != nil {
				a.log.Printf("config save: %v", err)
			}
		}
	}
	search()
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-a.kick:
			search()
		case <-t.C:
			if host := a.hub.Host(); host != "" {
				if _, ok := hub.Probe(host, a.cfg.HubPort, 3*time.Second); ok {
					continue
				}
				a.log.Printf("hub at %s not answering; searching again", host)
			}
			search()
		}
	}
}

func (a *app) rediscover() {
	select {
	case a.kick <- struct{}{}:
	default:
	}
}

func (a *app) hubInfo() server.HubInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	info := server.HubInfo{Host: a.hub.Host(), Port: a.cfg.HubPort, Firmware: a.firmware, Searching: a.searching, LastError: a.lastErr}
	if !a.lastFound.IsZero() {
		info.LastFound = a.lastFound.Format(time.RFC3339)
	}
	return info
}
