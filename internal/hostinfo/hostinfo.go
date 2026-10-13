// Package hostinfo describes the machine neoHub runs on, for the dashboard's Host tab: what
// it is, how busy, how full, and where to find the dashboard on the network. Works on Linux
// (a Raspberry Pi) and Windows. Processor use is a difference between two readings, so a
// reading is taken every two seconds in the background; everything else is read when asked.
package hostinfo

import (
	"net"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Snapshot is what /api/host returns.
type Snapshot struct {
	Machine Machine `json:"machine"`
	Live    Live    `json:"live"`
	App     App     `json:"app"`
	Network Network `json:"network"`
}

type Machine struct {
	Name         string `json:"name"`
	Model        string `json:"model,omitempty"`
	OS           string `json:"os"`
	Kernel       string `json:"kernel,omitempty"`
	Architecture string `json:"architecture"`
	Processor    string `json:"processor,omitempty"`
	Cores        int    `json:"cores"`
	UpSince      int64  `json:"upSince"` // unix ms
	TimeZone     string `json:"timeZone"`
	Now          int64  `json:"now"` // unix ms
}

type Live struct {
	CPUPercent         *float64 `json:"cpuPercent"`
	TemperatureCelsius *float64 `json:"temperatureCelsius"`
	MemoryTotal        *uint64  `json:"memoryTotal"`
	MemoryAvailable    *uint64  `json:"memoryAvailable"`
	DiskTotal          *uint64  `json:"diskTotal"`
	DiskFree           *uint64  `json:"diskFree"`
	DiskName           string   `json:"diskName,omitempty"`
}

type App struct {
	Version    string   `json:"version"`
	Runtime    string   `json:"runtime"`
	Started    int64    `json:"started"` // unix ms
	Memory     *uint64  `json:"memory"`  // resident set, bytes
	CPUPercent *float64 `json:"cpuPercent"`
	Goroutines int      `json:"goroutines"`
	Data       string   `json:"data"`
	DataSize   uint64   `json:"dataSize"`
	HubHost    string   `json:"hubHost,omitempty"`
}

type Interface struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Addresses []string `json:"addresses"`
}

type Network struct {
	Interfaces []Interface `json:"interfaces"`
	URLs       []string    `json:"urls"`
	Port       int         `json:"port"`
}

// Collector samples CPU use in the background. Create one with New and call Run once.
type Collector struct {
	Version string
	DataDir string
	Port    int
	NameURL string          // e.g. http://neohub.local/, "" if none
	HubHost func() string   // the hub address, for the app card
	LanIPs  func() []string // best-first LAN addresses, for the "open at" list

	mu         sync.Mutex
	cpu        *float64
	processCPU *float64
	started    time.Time
}

// New returns a collector; call Run in a goroutine.
func New() *Collector { return &Collector{started: time.Now()} }

// Run samples processor use every two seconds until the process ends.
func (c *Collector) Run() {
	var lastIdle, lastTotal uint64
	var lastProc float64
	lastAt := time.Now()
	first := true
	for {
		idle, total, ok := cpuTimes()
		proc, pok := processCPUSeconds()
		now := time.Now()
		elapsed := now.Sub(lastAt).Seconds()
		c.mu.Lock()
		if ok && !first && total > lastTotal {
			v := 100 * (1 - float64(idle-lastIdle)/float64(total-lastTotal))
			v = round1(v)
			c.cpu = &v
		}
		if pok && !first && elapsed > 0 {
			v := round1(100 * (proc - lastProc) / elapsed / float64(runtime.NumCPU()))
			if v < 0 {
				v = 0
			}
			c.processCPU = &v
		}
		c.mu.Unlock()
		lastIdle, lastTotal, lastProc, lastAt, first = idle, total, proc, now, false
		time.Sleep(2 * time.Second)
	}
}

// Snapshot reads everything now.
func (c *Collector) Snapshot() Snapshot {
	c.mu.Lock()
	cpu, pcpu := c.cpu, c.processCPU
	c.mu.Unlock()

	host, _ := os.Hostname()
	zone, _ := time.Now().Zone()
	memTotal, memAvail := memory()
	diskTotal, diskFree, diskName := disk(c.DataDir)
	up := uptime()

	var ifaces []Interface
	all, _ := net.Interfaces()
	for _, ifc := range all {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		var list []string
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipn.IP
			if ip4 := ip.To4(); ip4 != nil {
				list = append([]string{ip4.String()}, list...)
			} else if !ip.IsLinkLocalUnicast() {
				list = append(list, ip.String())
			}
		}
		if len(list) == 0 {
			continue
		}
		sort.SliceStable(list, func(i, j int) bool { return !strings.Contains(list[i], ":") && strings.Contains(list[j], ":") })
		ifaces = append(ifaces, Interface{Name: ifc.Name, Kind: kindOf(ifc.Name), Addresses: list})
	}

	suffix := ""
	if c.Port != 0 && c.Port != 80 {
		suffix = ":" + itoa(c.Port)
	}
	var urls []string
	if c.NameURL != "" {
		urls = append(urls, c.NameURL)
	}
	if c.LanIPs != nil {
		for _, ip := range c.LanIPs() {
			urls = append(urls, "http://"+ip+suffix+"/")
		}
	}

	app := App{Version: c.Version, Runtime: runtime.Version() + " (" + runtime.GOOS + "/" + runtime.GOARCH + ")", Started: c.started.UnixMilli(),
		CPUPercent: pcpu, Goroutines: runtime.NumGoroutine(), Data: c.DataDir, DataSize: dirSize(c.DataDir)}
	if rss, ok := processRSS(); ok {
		app.Memory = &rss
	}
	if c.HubHost != nil {
		app.HubHost = c.HubHost()
	}

	return Snapshot{
		Machine: Machine{Name: host, Model: model(), OS: osName(), Kernel: kernel(), Architecture: runtime.GOARCH, Processor: processorName(),
			Cores: runtime.NumCPU(), UpSince: time.Now().Add(-up).UnixMilli(), TimeZone: zone, Now: time.Now().UnixMilli()},
		Live:    Live{CPUPercent: cpu, TemperatureCelsius: temperature(), MemoryTotal: memTotal, MemoryAvailable: memAvail, DiskTotal: diskTotal, DiskFree: diskFree, DiskName: diskName},
		App:     app,
		Network: Network{Interfaces: ifaces, URLs: urls, Port: c.Port},
	}
}

func kindOf(name string) string {
	n := strings.ToLower(name)
	switch {
	case strings.HasPrefix(n, "wl"), strings.Contains(n, "wi-fi"), strings.Contains(n, "wifi"), strings.Contains(n, "wireless"):
		return "Wi-Fi"
	case strings.HasPrefix(n, "eth"), strings.HasPrefix(n, "en"), strings.Contains(n, "ethernet"):
		return "Wired"
	case strings.Contains(n, "vpn"), strings.Contains(n, "wireguard"), strings.Contains(n, "tailscale"), strings.HasPrefix(n, "tun"), strings.HasPrefix(n, "wg"):
		return "VPN"
	case strings.HasPrefix(n, "docker"), strings.HasPrefix(n, "br-"), strings.HasPrefix(n, "veth"), strings.Contains(n, "virtual"), strings.Contains(n, "vethernet"):
		return "Virtual"
	}
	return "Network"
}

func dirSize(dir string) uint64 {
	if dir == "" {
		return 0
	}
	var total uint64
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	for _, e := range entries {
		if info, err := e.Info(); err == nil && !info.IsDir() {
			total += uint64(info.Size())
		}
	}
	return total
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}

func itoa(n int) string { return strconv.Itoa(n) }
