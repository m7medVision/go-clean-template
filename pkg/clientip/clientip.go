package clientip

import (
	"net/netip"
	"strings"
)

func Resolve(peer netip.Addr, forwardedFor []string, proxy netip.Addr) netip.Addr {
	peer = peer.Unmap().WithZone("")
	if !proxy.IsValid() || peer != proxy.Unmap().WithZone("") {
		return peer
	}

	hops := strings.Split(strings.Join(forwardedFor, ","), ",")

	client, err := netip.ParseAddr(strings.TrimSpace(hops[len(hops)-1]))
	if err != nil {
		return peer
	}

	return client.Unmap().WithZone("")
}
