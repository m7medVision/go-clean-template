package httpmw_test

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/controller/httpmw"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	_proxy     = "172.30.0.10"
	_forwarded = "203.0.113.7"
)

func TestClientAddressResolvesTheClient(t *testing.T) {
	t.Parallel()

	proxy := netip.MustParseAddr(_proxy)

	tests := []struct {
		name       string
		proxy      netip.Addr
		remoteAddr string
		want       netip.Addr
		wantOK     bool
	}{
		{"from the proxy, the client its header names", proxy, _proxy + ":41000", netip.MustParseAddr(_forwarded), true},
		{"a forged header from elsewhere, its own address", proxy, "198.51.100.4:41000", netip.MustParseAddr("198.51.100.4"), true},
		{"no proxy configured, its own address", netip.Addr{}, _proxy + ":41000", proxy, true},
		{"an unparsable remote address, no client", proxy, "pipe", netip.Addr{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var (
				served bool
				got    netip.Addr
				ok     bool
			)

			h := httpmw.ClientAddress(tt.proxy)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				served = true
				got, ok = entity.ClientFromContext(r.Context())
			}))

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", http.NoBody)
			req.RemoteAddr = tt.remoteAddr
			req.Header.Set("X-Forwarded-For", _forwarded)

			h.ServeHTTP(httptest.NewRecorder(), req)

			require.True(t, served)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}
