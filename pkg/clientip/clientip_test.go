package clientip_test

import (
	"net/netip"
	"testing"

	"github.com/m7medvision/go-clean-template/pkg/clientip"
	"github.com/stretchr/testify/assert"
)

func TestResolve(t *testing.T) {
	t.Parallel()

	ip := netip.MustParseAddr
	proxy := ip("172.30.0.10")
	direct := ip("192.0.2.1")
	client := ip("203.0.113.7")

	tests := []struct {
		name         string
		peer         netip.Addr
		forwardedFor []string
		proxy        netip.Addr
		want         netip.Addr
	}{
		{name: "no proxy configured", peer: proxy, forwardedFor: []string{"203.0.113.7"}, want: proxy},
		{name: "a direct client with no header", peer: direct, proxy: proxy, want: direct},
		{name: "a forged header from a non-proxy", peer: direct, forwardedFor: []string{"203.0.113.7"}, proxy: proxy, want: direct},
		{name: "through the proxy", peer: proxy, forwardedFor: []string{"203.0.113.7"}, proxy: proxy, want: client},
		{
			name:         "several header lines and comma lists",
			peer:         proxy,
			forwardedFor: []string{"198.51.100.1, 198.51.100.2", "198.51.100.3 , 203.0.113.7 "},
			proxy:        proxy,
			want:         client,
		},
		{name: "an unparsable rightmost entry", peer: proxy, forwardedFor: []string{"203.0.113.7, unknown"}, proxy: proxy, want: proxy},
		{name: "the proxy with no header", peer: proxy, proxy: proxy, want: proxy},
		{name: "an IPv4-mapped peer is the proxy", peer: ip("::ffff:172.30.0.10"), forwardedFor: []string{"203.0.113.7"}, proxy: proxy, want: client},
		{name: "an IPv4-mapped entry comes back unmapped", peer: proxy, forwardedFor: []string{"::ffff:203.0.113.7"}, proxy: proxy, want: client},
		{name: "an IPv6 client through the proxy", peer: proxy, forwardedFor: []string{"2001:db8::1"}, proxy: proxy, want: ip("2001:db8::1")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, clientip.Resolve(tt.peer, tt.forwardedFor, tt.proxy))
		})
	}
}
