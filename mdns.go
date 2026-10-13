package main

import (
	"net"
	"time"

	"github.com/grandcat/zeroconf"
)

// advertise announces the dashboard as <name>.local over mDNS (Bonjour), so phones, Macs,
// Windows 10+ and Linux machines can open http://neohub.local:8484/ without knowing the IP.
// It re-registers every few minutes so a changed address is picked up. Set "mdnsName": ""
// in config.json to turn it off (for example when two machines on one network run neoHub).
func (a *app) advertise(name string, port int) {
	if name == "" {
		return
	}
	var cur *zeroconf.Server
	register := func() {
		ips := a.lanAddresses()
		if len(ips) == 0 {
			return
		}
		if cur != nil {
			cur.Shutdown()
			cur = nil
		}
		ifaces := upInterfaces()
		s, err := zeroconf.RegisterProxy("neoHub", "_http._tcp", "local.", port, name, ips, []string{"path=/"}, ifaces)
		if err != nil {
			a.log.Printf("mdns: %v", err)
			return
		}
		cur = s
		a.log.Printf("mdns: advertising http://%s.local:%d/ on %v", name, port, ips)
	}
	register()
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()
	for range t.C {
		if cur == nil || joinIPs(a.lanAddresses()) != a.mdnsIPs {
			a.mdnsIPs = joinIPs(a.lanAddresses())
			register()
		}
	}
}

func joinIPs(ips []string) string {
	out := ""
	for _, ip := range ips {
		out += ip + ","
	}
	return out
}

func upInterfaces() []net.Interface {
	all, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var out []net.Interface
	for _, ifc := range all {
		if ifc.Flags&net.FlagUp != 0 && ifc.Flags&net.FlagLoopback == 0 && ifc.Flags&net.FlagMulticast != 0 {
			out = append(out, ifc)
		}
	}
	return out
}
