//go:build windows

package hostinfo

import (
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	kernel32            = windows.NewLazySystemDLL("kernel32.dll")
	procGetSystemTimes  = kernel32.NewProc("GetSystemTimes")
	procGlobalMemStatus = kernel32.NewProc("GlobalMemoryStatusEx")
	procGetTickCount64  = kernel32.NewProc("GetTickCount64")
	procGetDiskFreeEx   = kernel32.NewProc("GetDiskFreeSpaceExW")
	procGetProcMemInfo  = kernel32.NewProc("K32GetProcessMemoryInfo")
)

func ftToUint(ft windows.Filetime) uint64 {
	return uint64(ft.HighDateTime)<<32 | uint64(ft.LowDateTime)
}

// cpuTimes: kernel time includes idle time, so total = kernel + user.
func cpuTimes() (idle, total uint64, ok bool) {
	var i, k, u windows.Filetime
	r, _, _ := procGetSystemTimes.Call(uintptr(unsafe.Pointer(&i)), uintptr(unsafe.Pointer(&k)), uintptr(unsafe.Pointer(&u)))
	if r == 0 {
		return 0, 0, false
	}
	return ftToUint(i), ftToUint(k) + ftToUint(u), true
}

func processCPUSeconds() (float64, bool) {
	var c, e, k, u windows.Filetime
	h := windows.CurrentProcess()
	if err := windows.GetProcessTimes(h, &c, &e, &k, &u); err != nil {
		return 0, false
	}
	return float64(ftToUint(k)+ftToUint(u)) / 1e7, true // 100 ns units
}

type memoryStatusEx struct {
	Length, MemoryLoad                                                                       uint32
	TotalPhys, AvailPhys, TotalPageFile, AvailPageFile, TotalVirtual, AvailVirtual, AvailExt uint64
}

func memory() (total, available *uint64) {
	var m memoryStatusEx
	m.Length = uint32(unsafe.Sizeof(m))
	r, _, _ := procGlobalMemStatus.Call(uintptr(unsafe.Pointer(&m)))
	if r == 0 {
		return nil, nil
	}
	t, a := m.TotalPhys, m.AvailPhys
	return &t, &a
}

func disk(dir string) (total, free *uint64, name string) {
	root := dir
	if len(root) >= 2 && root[1] == ':' {
		root = root[:2] + `\`
	} else if root == "" {
		root = `C:\`
	}
	p, err := syscall.UTF16PtrFromString(root)
	if err != nil {
		return nil, nil, ""
	}
	var avail, tot, totFree uint64
	r, _, _ := procGetDiskFreeEx.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&avail)), uintptr(unsafe.Pointer(&tot)), uintptr(unsafe.Pointer(&totFree)))
	if r == 0 {
		return nil, nil, ""
	}
	return &tot, &avail, root
}

func uptime() time.Duration {
	r, _, _ := procGetTickCount64.Call()
	return time.Duration(r) * time.Millisecond
}

func temperature() *float64 { return nil }

func regString(key registry.Key, path, name string) string {
	k, err := registry.OpenKey(key, path, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	v, _, err := k.GetStringValue(name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(v)
}

func model() string {
	maker := regString(registry.LOCAL_MACHINE, `HARDWARE\DESCRIPTION\System\BIOS`, "SystemManufacturer")
	product := regString(registry.LOCAL_MACHINE, `HARDWARE\DESCRIPTION\System\BIOS`, "SystemProductName")
	return strings.TrimSpace(maker + " " + product)
}

func osName() string {
	const nt = `SOFTWARE\Microsoft\Windows NT\CurrentVersion`
	name := regString(registry.LOCAL_MACHINE, nt, "ProductName")
	if name == "" {
		name = "Windows"
	}
	build := regString(registry.LOCAL_MACHINE, nt, "CurrentBuildNumber")
	// The product name still says "Windows 10" on Windows 11; the build number tells them apart.
	if b, _ := strconv.Atoi(build); b >= 22000 {
		name = strings.Replace(name, "Windows 10", "Windows 11", 1)
	}
	if rel := regString(registry.LOCAL_MACHINE, nt, "DisplayVersion"); rel != "" {
		name += " " + rel
	}
	if build != "" {
		name += " (build " + build + ")"
	}
	return name
}

func kernel() string { return "" }

func processorName() string {
	return regString(registry.LOCAL_MACHINE, `HARDWARE\DESCRIPTION\System\CentralProcessor\0`, "ProcessorNameString")
}

type processMemoryCounters struct {
	Cb                                                                                                                                                                     uint32
	PageFaultCount                                                                                                                                                         uint32
	PeakWorkingSetSize, WorkingSetSize, QuotaPeakPagedPoolUsage, QuotaPagedPoolUsage, QuotaPeakNonPagedPoolUsage, QuotaNonPagedPoolUsage, PagefileUsage, PeakPagefileUsage uintptr
}

func processRSS() (uint64, bool) {
	var pmc processMemoryCounters
	pmc.Cb = uint32(unsafe.Sizeof(pmc))
	r, _, _ := procGetProcMemInfo.Call(uintptr(windows.CurrentProcess()), uintptr(unsafe.Pointer(&pmc)), uintptr(pmc.Cb))
	if r == 0 {
		return 0, false
	}
	return uint64(pmc.WorkingSetSize), true
}
