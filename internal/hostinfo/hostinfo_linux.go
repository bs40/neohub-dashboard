//go:build linux

package hostinfo

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// cpuTimes reads cumulative idle and total ticks from /proc/stat.
func cpuTimes() (idle, total uint64, ok bool) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		return 0, 0, false
	}
	fields := strings.Fields(sc.Text())
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0, false
	}
	var vals []uint64
	for _, s := range fields[1:] {
		v, _ := strconv.ParseUint(s, 10, 64)
		vals = append(vals, v)
	}
	idle = vals[3]
	if len(vals) > 4 {
		idle += vals[4] // iowait
	}
	n := len(vals)
	if n > 8 {
		n = 8
	}
	for _, v := range vals[:n] {
		total += v
	}
	return idle, total, true
}

// processCPUSeconds is this process's user+system time from /proc/self/stat.
func processCPUSeconds() (float64, bool) {
	b, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return 0, false
	}
	// Fields after the ")" of the command name: state is field 3; utime is 14, stime 15.
	s := string(b)
	i := strings.LastIndex(s, ")")
	if i < 0 {
		return 0, false
	}
	fields := strings.Fields(s[i+1:])
	if len(fields) < 13 {
		return 0, false
	}
	ut, _ := strconv.ParseFloat(fields[11], 64)
	st, _ := strconv.ParseFloat(fields[12], 64)
	return (ut + st) / 100, true // clock ticks are 100/s on Linux
}

func memory() (total, available *uint64) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return nil, nil
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		key, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		kb, _ := strconv.ParseUint(strings.Fields(rest)[0], 10, 64)
		v := kb * 1024
		switch key {
		case "MemTotal":
			total = &v
		case "MemAvailable":
			available = &v
		}
	}
	return
}

func disk(dir string) (total, free *uint64, name string) {
	if dir == "" {
		dir = "/"
	}
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return nil, nil, ""
	}
	t := uint64(st.Blocks) * uint64(st.Bsize)
	f := uint64(st.Bavail) * uint64(st.Bsize)
	return &t, &f, dir
}

func uptime() time.Duration {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	s, _ := strconv.ParseFloat(strings.Fields(string(b))[0], 64)
	return time.Duration(s * float64(time.Second))
}

func temperature() *float64 {
	b, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp")
	if err != nil {
		return nil
	}
	m, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
	if err != nil {
		return nil
	}
	v := round1(m / 1000)
	return &v
}

func model() string {
	if b, err := os.ReadFile("/proc/device-tree/model"); err == nil {
		return strings.TrimRight(string(b), "\x00\n")
	}
	return ""
}

func osName() string {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return "Linux"
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), "PRETTY_NAME="); ok {
			return strings.Trim(v, `"`)
		}
	}
	return "Linux"
}

func kernel() string {
	var u syscall.Utsname
	if syscall.Uname(&u) != nil {
		return ""
	}
	return cstr(u.Release[:])
}

func cstr[T int8 | uint8](a []T) string {
	b := make([]byte, 0, len(a))
	for _, c := range a {
		if c == 0 {
			break
		}
		b = append(b, byte(c))
	}
	return string(b)
}

func processorName() string {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	defer f.Close()
	part := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if v, ok := strings.CutPrefix(line, "model name"); ok {
			if _, name, ok := strings.Cut(v, ":"); ok {
				return strings.TrimSpace(name)
			}
		}
		if v, ok := strings.CutPrefix(line, "CPU part"); ok && part == "" {
			if _, p, ok := strings.Cut(v, ":"); ok {
				part = strings.TrimSpace(p)
			}
		}
	}
	// Arm boards name the core by part number instead.
	switch part {
	case "0xd03":
		return "Arm Cortex-A53"
	case "0xd08":
		return "Arm Cortex-A72"
	case "0xd0b":
		return "Arm Cortex-A76"
	case "0xd04":
		return "Arm Cortex-A35"
	case "0xd05":
		return "Arm Cortex-A55"
	case "":
		return ""
	}
	return "Arm (part " + part + ")"
}

func processRSS() (uint64, bool) {
	f, err := os.Open("/proc/self/status")
	if err != nil {
		return 0, false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), "VmRSS:"); ok {
			kb, _ := strconv.ParseUint(strings.Fields(v)[0], 10, 64)
			return kb * 1024, true
		}
	}
	return 0, false
}
