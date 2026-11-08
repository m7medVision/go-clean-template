package entity

import (
	"context"
	"net/netip"
)

type clientKey struct{}

func ContextWithClient(ctx context.Context, client netip.Addr) context.Context {
	return context.WithValue(ctx, clientKey{}, client)
}

func ClientFromContext(ctx context.Context) (netip.Addr, bool) {
	client, ok := ctx.Value(clientKey{}).(netip.Addr)

	return client, ok && client.IsValid()
}
