// Package discover finds a neoHub on the local network. The hub does not answer the
// Heatmiser app's "hubseek" UDP broadcast on current firmware, so discovery is a TCP
// sweep of each local /24 for port 4242, confirmed with a FIRMWARE query.
package discover

import (
	"context"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/bs40/neohub-dashboard/internal/hub"
)

// Result is a hub that answered.
type Result struct {
	IP       string
	Firmware string
}

// Find tries lastIP first, then sweeps the local subnets. It returns the first hub found
// or an error when ctx ends.
func Find(ctx context.Context, lastIP string, port int) (Result, error) {
	if lastIP != "" {
		if fw, ok := hub.Probe(lastIP, port, 1500*time.Millisecond); ok {
			return Result{IP: lastIP, Firmware: fw}, nil
		}
	}
	for _, subnet := range localSubnets() {
		if r, ok := sweep(ctx, subnet, port); ok {
			return r, nil
		}
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
	}
	return Result{}, errNotFound
}

type notFound struct{}

func (notFound) Error() string { return "no neoHub found on the local network" }

var errNotFound = notFound{}

// localSubnets lists the /24 networks of this machine's private IPv4 addresses,
// most likely first (the one containing the default-looking 192.168.x.x addresses).
func localSubnets() []*net.IPNet {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var out []*net.IPNet
	seen := map[string]bool{}
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipn.IP.To4()
			if ip4 == nil || !ip4.IsPrivate() {
				continue
			}
			// Always sweep the /24 the host sits in, even on a wider mask.
			n := &net.IPNet{IP: ip4.Mask(net.CIDRMask(24, 32)), Mask: net.CIDRMask(24, 32)}
			if !seen[n.String()] {
				seen[n.String()] = true
				out = append(out, n)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].IP[0] == 192 && out[j].IP[0] != 192 })
	return out
}

func sweep(ctx context.Context, subnet *net.IPNet, port int) (Result, bool) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ips := make(chan string, 254)
	base := subnet.IP.To4()
	for i := 1; i <= 254; i++ {
		ips <- net.IPv4(base[0], base[1], base[2], byte(i)).String()
	}
	close(ips)
	results := make(chan Result, 1)
	var wg sync.WaitGroup
	for w := 0; w < 64; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ip := range ips {
				if ctx.Err() != nil {
					return
				}
				if fw, ok := hub.Probe(ip, port, 800*time.Millisecond); ok {
					select {
					case results <- Result{IP: ip, Firmware: fw}:
					default:
					}
					cancel()
					return
				}
			}
		}()
	}
	wg.Wait()
	select {
	case r := <-results:
		return r, true
	default:
		return Result{}, false
	}
}
