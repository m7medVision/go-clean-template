package middleware

import (
	"context"
	"net/netip"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/pkg/clientip"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

func ClientAddress(proxy netip.Addr) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		return handler(withClient(ctx, proxy), req)
	}
}

func StreamClientAddress(proxy netip.Addr) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		return handler(srv, clientStream{ServerStream: ss, ctx: withClient(ss.Context(), proxy)})
	}
}

type clientStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s clientStream) Context() context.Context { return s.ctx }

func withClient(ctx context.Context, proxy netip.Addr) context.Context {
	p, ok := peer.FromContext(ctx)
	if !ok || p.Addr == nil {
		return ctx
	}

	addr, err := netip.ParseAddrPort(p.Addr.String())
	if err != nil {
		return ctx
	}

	md, _ := metadata.FromIncomingContext(ctx)

	return entity.ContextWithClient(ctx, clientip.Resolve(addr.Addr(), md.Get("x-forwarded-for"), proxy))
}
