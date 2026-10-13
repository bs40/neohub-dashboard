//go:build linux

// Linux entry point: a headless service, normally run by systemd from the .deb package.
//
//	neohub            run the dashboard (what the service does)
//	neohub status     show the hub the running service found
//	neohub phone      print the phone link (address and QR code) in the terminal; "qr" works too
//	neohub version
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/bs40/neohub-dashboard/internal/config"
)

func main() {
	port := flag.Int("port", 0, "override the listen port")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "neoHeat %s\n\nusage: neohub [-port N]        run the dashboard\n       neohub status           show the hub the running service found\n       neohub phone            print the phone link (address and QR code)\n       neohub version\n\nData directory: %s (override with NEOHUB_DATA)\n", Version, config.Dir())
	}
	flag.Parse()
	cfg := config.Load()
	listenPort := cfg.ListenPort
	if *port != 0 {
		listenPort = *port
	}
	base := fmt.Sprintf("http://localhost:%d", listenPort)

	switch flag.Arg(0) {
	case "version":
		fmt.Println("neoHeat", Version)
		return
	case "status":
		os.Exit(cmdStatus(base))
	case "qr", "phone":
		os.Exit(cmdQR(base))
	case "":
	default:
		flag.Usage()
		os.Exit(2)
	}

	// The package's neohub-mdns.service publishes neoheat.local; offer it as the friendly address.
	// Set NEOHUB_NAME_URL to override (or to "-" to hide it).
	nameURL = "http://neoheat.local/"
	if v := os.Getenv("NEOHUB_NAME_URL"); v == "-" {
		nameURL = ""
	} else if v != "" {
		nameURL = v
	}
	// systemd captures stderr into the journal, so log there.
	logger := log.New(os.Stderr, "", 0)
	logger.Printf("neoHeat %s starting; data in %s", Version, config.Dir())
	a, err := newApp(cfg, listenPort, logger)
	if err != nil {
		logger.Printf("%v", err)
		os.Exit(1)
	}
	a.start()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	a.shutdown()
}

func getJSON(url string, v any) error {
	c := &http.Client{Timeout: 5 * time.Second}
	r, err := c.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	if r.StatusCode != 200 {
		return fmt.Errorf("HTTP %d: %s", r.StatusCode, b)
	}
	return json.Unmarshal(b, v)
}

func cmdStatus(base string) int {
	var info struct {
		Host, Firmware, LastFound, LastError string
		Searching                            bool
	}
	if err := getJSON(base+"/api/hub", &info); err != nil {
		fmt.Fprintf(os.Stderr, "neoHeat is not running on %s (%v)\n  check: systemctl status neohub\n", base, err)
		return 1
	}
	var addr struct{ Primary string }
	_ = getJSON(base+"/api/addresses", &addr)
	switch {
	case info.Host != "":
		fmt.Printf("hub:        %s (firmware %s)\n", info.Host, info.Firmware)
	case info.Searching:
		fmt.Println("hub:        searching the network…")
	default:
		fmt.Printf("hub:        not found (%s)\n", info.LastError)
	}
	fmt.Printf("dashboard:  %s\n", addr.Primary)
	return 0
}

func cmdQR(base string) int {
	var addr struct{ Primary string }
	if err := getJSON(base+"/api/addresses", &addr); err != nil || addr.Primary == "" {
		fmt.Fprintf(os.Stderr, "neoHeat is not running on %s\n", base)
		return 1
	}
	q, err := qrcode.New(addr.Primary, qrcode.Medium)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println()
	fmt.Print(q.ToSmallString(false))
	fmt.Printf("\n  %s\n  Point your phone's camera at the code, then \"Add to Home Screen\".\n\n", addr.Primary)
	return 0
}
