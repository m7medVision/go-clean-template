package middleware_test

import (
	"context"
	"net"
	"net/netip"
	"testing"

	grpcmw "github.com/m7medvision/go-clean-template/internal/controller/grpc/middleware"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

const (
	_proxy     = "172.30.0.10"
	_forwarded = "203.0.113.7"
)

func tcp(addr string) net.Addr { return net.TCPAddrFromAddrPort(netip.MustParseAddrPort(addr)) }

func forwardedBy(t *testing.T, from net.Addr) context.Context {
	t.Helper()

	ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs("x-forwarded-for", _forwarded))
	if from == nil {
		return ctx
	}

	return peer.NewContext(ctx, &peer.Peer{Addr: from})
}

type contextStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s contextStream) Context() context.Context { return s.ctx }

func TestClientAddressResolvesTheClient(t *testing.T) {
	t.Parallel()

	proxy := netip.MustParseAddr(_proxy)

	tests := []struct {
		name   string
		proxy  netip.Addr
		peer   net.Addr
		want   netip.Addr
		wantOK bool
	}{
		{"from the proxy, the client its metadata names", proxy, tcp(_proxy + ":41000"), netip.MustParseAddr(_forwarded), true},
		{"forged metadata from elsewhere, its own address", proxy, tcp("198.51.100.4:41000"), netip.MustParseAddr("198.51.100.4"), true},
		{"no proxy configured, its own address", netip.Addr{}, tcp(_proxy + ":41000"), proxy, true},
		{"a peer with no IP address, no client", proxy, &net.UnixAddr{Name: "@grpc", Net: "unix"}, netip.Addr{}, false},
		{"no peer, no client", proxy, nil, netip.Addr{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var (
				got netip.Addr
				ok  bool
			)

			resp, err := grpcmw.ClientAddress(tt.proxy)(forwardedBy(t, tt.peer), nil, unaryInfo(), func(ctx context.Context, _ any) (any, error) {
				got, ok = entity.ClientFromContext(ctx)

				return respOK, nil
			})

			require.NoError(t, err)
			assert.Equal(t, respOK, resp)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStreamClientAddressSetsTheClientOnTheStream(t *testing.T) {
	t.Parallel()

	var (
		got netip.Addr
		ok  bool
	)

	err := grpcmw.StreamClientAddress(netip.MustParseAddr(_proxy))(nil, contextStream{ctx: forwardedBy(t, tcp(_proxy+":41000"))},
		&grpc.StreamServerInfo{FullMethod: "/grpc.reflection.v1.ServerReflection/ServerReflectionInfo"},
		func(_ any, ss grpc.ServerStream) error {
			got, ok = entity.ClientFromContext(ss.Context())

			return nil
		})

	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, netip.MustParseAddr(_forwarded), got)
}
