package main

import (
	"net"
	"sort"
	"strings"
)

// lanAddresses lists this machine's private IPv4 addresses, best first: the one on the
// hub's subnet, then 192.168.x.x, then the rest. VPN-looking adapters sort last.
func (a *app) lanAddresses() []string {
	type cand struct {
		ip    string
		score int
	}
	hubIP := net.ParseIP(a.hub.Host())
	var cands []cand
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		name := strings.ToLower(ifc.Name)
		vpn := strings.Contains(name, "vpn") || strings.Contains(name, "wireguard") || strings.Contains(name, "tailscale") || strings.Contains(name, "virtual") || strings.Contains(name, "vethernet") || strings.Contains(name, "vmware")
		addrs, _ := ifc.Addrs()
		for _, ad := range addrs {
			ipn, ok := ad.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipn.IP.To4()
			if ip4 == nil || !ip4.IsPrivate() {
				continue
			}
			score := 0
			if hubIP != nil && ipn.Contains(hubIP) {
				score += 100
			}
			if ip4[0] == 192 {
				score += 10
			}
			if vpn {
				score -= 50
			}
			cands = append(cands, cand{ip4.String(), score})
		}
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].score > cands[j].score })
	out := make([]string, 0, len(cands))
	for _, c := range cands {
		out = append(out, c.ip)
	}
	return out
}
