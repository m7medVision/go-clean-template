package httpmw

import (
	"net/http"
	"net/netip"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/pkg/clientip"
)

func ClientAddress(proxy netip.Addr) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			peer, err := netip.ParseAddrPort(r.RemoteAddr)
			if err != nil {
				next.ServeHTTP(w, r)

				return
			}

			client := clientip.Resolve(peer.Addr(), r.Header.Values("X-Forwarded-For"), proxy)
			next.ServeHTTP(w, r.WithContext(entity.ContextWithClient(r.Context(), client)))
		})
	}
}
