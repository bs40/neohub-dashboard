// Package history records hub connectivity over time for the dashboard's History tab.
// A snapshot is appended to history.jsonl whenever anything changes (hub reachable, NTP
// state, offline or low-battery stats) plus a heartbeat at a fixed interval so the
// timeline can tell "fine" from "not monitored".
package history

import (
	"bufio"
	"encoding/json"
	"os"
	"reflect"
	"sync"
	"time"
)

// Snapshot is one line of the log. Hub is nil for informational notes.
type Snapshot struct {
	T     int64    `json:"t"`
	Hub   *bool    `json:"hub"`
	Ntp   string   `json:"ntp,omitempty"`
	Drift int      `json:"drift,omitempty"`
	Off   []string `json:"off,omitempty"`
	Low   []string `json:"low,omitempty"`
	Zones []string `json:"zones"`
	Err   string   `json:"err,omitempty"`
	Note  string   `json:"note,omitempty"`
}

// Store is the in-memory log backed by a JSONL file.
type Store struct {
	mu        sync.Mutex
	path      string
	snaps     []Snapshot
	last      *Snapshot
	heartbeat time.Duration
}

// Open loads an existing log (if any).
func Open(path string, heartbeat time.Duration) *Store {
	s := &Store{path: path, heartbeat: heartbeat}
	if f, err := os.Open(path); err == nil {
		defer f.Close()
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			var snap Snapshot
			if json.Unmarshal(sc.Bytes(), &snap) == nil {
				s.snaps = append(s.snaps, snap)
			}
		}
		if n := len(s.snaps); n > 0 {
			s.last = &s.snaps[n-1]
		}
	}
	return s
}

// LastZones returns the zone list from the latest snapshot (for notes).
func (s *Store) LastZones() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.last == nil {
		return []string{}
	}
	return s.last.Zones
}

// Record appends snap if it differs from the last one or the heartbeat is due. force
// always appends.
func (s *Store) Record(snap Snapshot, force bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if snap.Zones == nil {
		snap.Zones = []string{}
	}
	if !force && s.last != nil && same(*s.last, snap) && time.Duration(snap.T-s.last.T)*time.Millisecond < s.heartbeat {
		return
	}
	s.snaps = append(s.snaps, snap)
	s.last = &s.snaps[len(s.snaps)-1]
	if f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
		b, _ := json.Marshal(snap)
		_, _ = f.Write(append(b, '\n'))
		f.Close()
	}
}

// Note appends an informational line such as "dashboard started".
func (s *Store) Note(text string) {
	s.Record(Snapshot{T: time.Now().UnixMilli(), Hub: nil, Note: text, Zones: s.LastZones()}, true)
}

func same(a, b Snapshot) bool {
	return boolEq(a.Hub, b.Hub) && a.Ntp == b.Ntp && reflect.DeepEqual(a.Off, b.Off) && reflect.DeepEqual(a.Low, b.Low) && reflect.DeepEqual(a.Zones, b.Zones)
}

func boolEq(a, b *bool) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// Since returns snapshots from `since` (ms) onward, including the last one before it so
// the caller knows the state at the left edge of its window.
func (s *Store) Since(since int64) []Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	start := len(s.snaps)
	for i, snap := range s.snaps {
		if snap.T >= since {
			start = i
			break
		}
	}
	if start > 0 {
		start--
	}
	out := make([]Snapshot, len(s.snaps)-start)
	copy(out, s.snaps[start:])
	return out
}

// Count returns the number of snapshots held.
func (s *Store) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.snaps)
}
