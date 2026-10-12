// neoHub: a Windows tray application that finds a Heatmiser neoHub on the local network
// and serves the dashboard to the LAN. Build with:
//
//	go build -ldflags "-s -w -H windowsgui" -o dist/neoHub.exe .
//
// Run with -console to keep a terminal and skip the tray (handy for debugging).
package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"fyne.io/systray"

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

//go:embed assets/tray.ico
var trayIcon []byte

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
}

func main() {
	console := flag.Bool("console", false, "run in the terminal without a tray icon")
	port := flag.Int("port", 0, "override the listen port")
	openFlag := flag.Bool("open", false, "open the dashboard in the browser once it is up")
	flag.Parse()

	firstRun := !fileExists(filepath.Join(config.Dir(), "config.json"))
	cfg := config.Load()
	listenPort := cfg.ListenPort
	if *port != 0 { // a command-line override is for this run only; it is not saved
		listenPort = *port
	}
	logger := openLog(*console)
	logger.Printf("neoHub %s starting; data in %s", Version, config.Dir())

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
		// Most likely a copy is already running: just show the dashboard and leave.
		logger.Printf("cannot listen on %s (%v); opening the existing dashboard", addr, err)
		openBrowser(a.url())
		return
	}
	a.http = &http.Server{Handler: a.srv, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := a.http.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Printf("http: %v", err)
		}
	}()
	logger.Printf("dashboard on http://localhost:%d", listenPort)

	ctx, cancel := context.WithCancel(context.Background())
	go a.discoveryLoop(ctx)
	go a.srv.Run()
	if *openFlag || firstRun {
		// Give discovery a head start so the first page shows real data.
		go func() {
			for i := 0; i < 20 && a.hub.Host() == ""; i++ {
				time.Sleep(250 * time.Millisecond)
			}
			openBrowser(a.url())
		}()
	}

	if *console {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
		a.shutdown(cancel)
		return
	}
	systray.Run(func() { a.onTrayReady(cancel) }, func() {})
}

func (a *app) shutdown(cancel context.CancelFunc) {
	cancel()
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

// ---------- tray ----------

func (a *app) onTrayReady(cancel context.CancelFunc) {
	systray.SetIcon(trayIcon)
	systray.SetTitle("neoHub")
	systray.SetTooltip("neoHub dashboard")
	open := systray.AddMenuItem("Open dashboard", "Open the dashboard in your browser")
	phone := systray.AddMenuItem("QR code for phones", "Show a QR code other devices can scan")
	status := systray.AddMenuItem("Hub: searching…", "")
	status.Disable()
	find := systray.AddMenuItem("Find hub again", "Sweep the network for the hub")
	systray.AddSeparator()
	startup := systray.AddMenuItemCheckbox("Start with Windows", "Run neoHub when you sign in", autostartEnabled())
	systray.AddSeparator()
	quit := systray.AddMenuItem("Quit neoHub", "Stop the dashboard")

	go func() {
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-open.ClickedCh:
				openBrowser(a.url())
			case <-phone.ClickedCh:
				openBrowser(a.url() + "phone")
			case <-find.ClickedCh:
				a.rediscover()
			case <-startup.ClickedCh:
				if startup.Checked() {
					_ = setAutostart(false)
					startup.Uncheck()
				} else {
					if err := setAutostart(true); err == nil {
						startup.Check()
					}
				}
			case <-quit.ClickedCh:
				a.shutdown(cancel)
				systray.Quit()
				return
			case <-t.C:
				info := a.hubInfo()
				switch {
				case info.Searching && info.Host == "":
					status.SetTitle("Hub: searching the network…")
				case info.Host != "":
					status.SetTitle(fmt.Sprintf("Hub: %s (firmware %s)", info.Host, info.Firmware))
				default:
					status.SetTitle("Hub: not found")
				}
				systray.SetTooltip(fmt.Sprintf("neoHub dashboard · %s", a.url()))
			}
		}
	}()
}

func openBrowser(url string) {
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// ---------- logging ----------

func openLog(console bool) *log.Logger {
	flags := log.Ldate | log.Ltime
	f, err := os.OpenFile(filepath.Join(config.Dir(), "neohub.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return log.New(os.Stderr, "", flags)
	}
	if fi, err := f.Stat(); err == nil && fi.Size() > 5<<20 { // keep the log small
		f.Close()
		_ = os.Rename(filepath.Join(config.Dir(), "neohub.log"), filepath.Join(config.Dir(), "neohub.old.log"))
		f, _ = os.OpenFile(filepath.Join(config.Dir(), "neohub.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	}
	if console {
		return log.New(io.MultiWriter(os.Stderr, f), "", flags)
	}
	return log.New(f, "", flags)
}
