//go:build windows

// Windows entry point: a tray icon application. Build with:
//
//	go build -ldflags "-s -w -H windowsgui" -o dist/neoHub.exe .
//
// Run with -console to keep a terminal and skip the tray (handy for debugging).
package main

import (
	_ "embed"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"fyne.io/systray"

	"github.com/bs40/neohub-dashboard/internal/config"
)

//go:embed assets/tray.ico
var trayIcon []byte

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

	a, err := newApp(cfg, listenPort, logger)
	if err != nil {
		// Most likely a copy is already running: just show the dashboard and leave.
		logger.Printf("%v; opening the existing dashboard", err)
		openBrowser(fmt.Sprintf("http://localhost:%d/", listenPort))
		return
	}
	a.start()
	if *openFlag || firstRun {
		go func() { a.waitForHub(5 * time.Second); openBrowser(a.url()) }()
	}

	if *console {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
		a.shutdown()
		return
	}
	systray.Run(func() { a.onTrayReady() }, func() {})
}

// ---------- tray ----------

func (a *app) onTrayReady() {
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
				} else if err := setAutostart(true); err == nil {
					startup.Check()
				}
			case <-quit.ClickedCh:
				a.shutdown()
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

// openLog writes to %LOCALAPPDATA%\neoHub\neohub.log (and the terminal with -console).
func openLog(console bool) *log.Logger {
	flags := log.Ldate | log.Ltime
	path := filepath.Join(config.Dir(), "neohub.log")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return log.New(os.Stderr, "", flags)
	}
	if fi, err := f.Stat(); err == nil && fi.Size() > 5<<20 { // keep the log small
		f.Close()
		_ = os.Rename(path, filepath.Join(config.Dir(), "neohub.old.log"))
		f, _ = os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	}
	if console {
		return log.New(io.MultiWriter(os.Stderr, f), "", flags)
	}
	return log.New(f, "", flags)
}
