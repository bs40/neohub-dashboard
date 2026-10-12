// Package hub is a minimal client for the Heatmiser neoHub legacy TCP API (port 4242).
// Each command is a single-key JSON object terminated with "\0"; the reply is JSON
// terminated the same way. Commands are serialised: the hub copes badly with several
// simultaneous connections.
package hub

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// LogEntry is one command/reply pair kept for the dashboard's Advanced tab.
type LogEntry struct {
	At    string `json:"at"`
	Ms    int64  `json:"ms"`
	Cmd   string `json:"cmd"`
	OK    bool   `json:"ok"`
	Reply string `json:"reply"`
}

// Hub talks to one neoHub.
type Hub struct {
	mu      sync.Mutex // serialises commands
	host    string
	port    int
	timeout time.Duration

	logMu sync.Mutex
	log   []LogEntry
}

// New returns a client. host may be empty until discovery has found the hub.
func New(host string, port int, timeout time.Duration) *Hub {
	if port == 0 {
		port = 4242
	}
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	return &Hub{host: host, port: port, timeout: timeout}
}

// ErrNoHub is returned while no hub address is known.
var ErrNoHub = errors.New("no hub found yet")

// Host returns the current hub address ("" if unknown).
func (h *Hub) Host() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.host
}

// Port returns the hub port.
func (h *Hub) Port() int { return h.port }

// SetHost changes the hub address.
func (h *Hub) SetHost(host string) {
	h.mu.Lock()
	h.host = host
	h.mu.Unlock()
}

// Send runs one command and returns the raw JSON reply. cmd is anything that marshals to
// a single-key object, e.g. map[string]any{"SET_TEMP": []any{20, []string{"Hall"}}}.
func (h *Hub) Send(cmd any) (json.RawMessage, error) {
	payload, err := json.Marshal(cmd)
	if err != nil {
		return nil, err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.host == "" {
		return nil, ErrNoHub
	}
	started := time.Now()
	reply, err := sendOnce(net.JoinHostPort(h.host, strconv.Itoa(h.port)), payload, h.timeout)
	h.addLog(started, payload, reply, err)
	return reply, err
}

// Probe checks whether addr speaks the neoHub protocol and returns the firmware version.
func Probe(host string, port int, timeout time.Duration) (string, bool) {
	reply, err := sendOnce(net.JoinHostPort(host, strconv.Itoa(port)), []byte(`{"FIRMWARE":0}`), timeout)
	if err != nil {
		return "", false
	}
	var fw struct {
		Version string `json:"firmware version"`
	}
	if json.Unmarshal(reply, &fw) != nil || fw.Version == "" {
		return "", false
	}
	return fw.Version, true
}

func sendOnce(addr string, payload []byte, timeout time.Duration) (json.RawMessage, error) {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := conn.Write(append(payload, 0)); err != nil {
		return nil, err
	}
	r := bufio.NewReaderSize(conn, 64*1024)
	data, err := r.ReadBytes(0)
	if err != nil && len(data) == 0 {
		return nil, fmt.Errorf("hub gave no reply: %w", err)
	}
	data = []byte(strings.TrimRight(string(data), "\x00"))
	if !json.Valid(data) {
		return nil, fmt.Errorf("hub returned non-JSON: %.200s", data)
	}
	return json.RawMessage(data), nil
}

func (h *Hub) addLog(started time.Time, payload []byte, reply json.RawMessage, err error) {
	e := LogEntry{At: started.Format(time.RFC3339), Ms: time.Since(started).Milliseconds(), Cmd: clip(string(payload), 200), OK: err == nil}
	if err != nil {
		e.Reply = err.Error()
	} else {
		e.Reply = clip(string(reply), 200)
	}
	h.logMu.Lock()
	h.log = append(h.log, e)
	if len(h.log) > 200 {
		h.log = h.log[len(h.log)-200:]
	}
	h.logMu.Unlock()
}

// Log returns recent entries, newest first.
func (h *Hub) Log() []LogEntry {
	h.logMu.Lock()
	defer h.logMu.Unlock()
	out := make([]LogEntry, 0, len(h.log))
	for i := len(h.log) - 1; i >= 0; i-- {
		out = append(out, h.log[i])
	}
	return out
}

func clip(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
