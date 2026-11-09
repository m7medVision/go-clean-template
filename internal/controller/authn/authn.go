// Package authn reads who sent a request from its token, for every transport. REST, gRPC and
// GraphQL each read their own header, turn the token into a Caller with ReadToken and store it with
// WithCaller, under the one context key their handlers read it from. It imports no transport, so
// each of them may import it without importing another.
package authn

import (
	"context"
	"fmt"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/m7medvision/go-clean-template/pkg/sentrylog"
)

// Caller is who sent a request, as their token says.
type Caller struct {
	entity.Principal

	// IssuedAt is when their token was issued: when they last signed in. It is zero for a token
	// without an iat claim, which an operation that needs a recent sign-in treats as too old.
	IssuedAt time.Time
}

type callerKey struct{}

// ReadToken checks that tokens signed token and that it has not expired, and returns its caller.
// Role names the app does not know are dropped.
func ReadToken(tokens *jwt.Manager, token string) (Caller, error) {
	claims, err := tokens.Parse(token)
	if err != nil {
		return Caller{}, fmt.Errorf("authn - ReadToken - tokens.Parse: %w", err)
	}

	return Caller{
		UserID: claims.UserID, Roles: entity.ParseRoles(claims.Roles),
		IssuedAt: claims.IssuedAt,
	}, nil
}

func WithCaller(ctx context.Context, c Caller) context.Context {
	sentrylog.SetUser(ctx, c.UserID)

	return context.WithValue(ctx, callerKey{}, c)
}

// CallerFrom returns the caller ctx carries, and whether a token was read for it.
func CallerFrom(ctx context.Context) (Caller, bool) {
	c, ok := ctx.Value(callerKey{}).(Caller)

	return c, ok
}

// MustCaller returns the caller ctx carries, for a handler its transport only calls with one: a REST
// operation that declares Security, a gRPC method its server does not list as public. Those
// transports answer 401 or Unauthenticated before the handler runs, so a missing caller is a
// programming error (an operation that reads the caller without declaring that it needs a token),
// and MustCaller panics: the transport recovers, logs the stack and answers 500 or Internal.
func MustCaller(ctx context.Context) Caller {
	c, ok := CallerFrom(ctx)
	if !ok {
		panic("authn - MustCaller: no caller in the context: the operation does not declare that it needs a token")
	}

	return c
}
