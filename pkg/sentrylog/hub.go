package sentrylog

import (
	"context"
	"net/http"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/getsentry/sentry-go"
	"google.golang.org/grpc"
)

// Fork gives ctx a hub of its own, cloned from the one it carries or the root, so its breadcrumbs
// belong to this unit of work only.
func Fork(ctx context.Context) context.Context {
	parent := sentry.GetHubFromContext(ctx)
	if parent == nil {
		parent = sentry.CurrentHub()
	}

	return sentry.SetHubOnContext(ctx, parent.Clone())
}

func HTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(Fork(r.Context())))
	})
}

func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
		return h(Fork(ctx), req)
	}
}

func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, h grpc.StreamHandler) error {
		return h(srv, &stream{ServerStream: ss, ctx: Fork(ss.Context())})
	}
}

func Watermill(h message.HandlerFunc) message.HandlerFunc {
	return func(msg *message.Message) ([]*message.Message, error) {
		msg.SetContext(Fork(msg.Context()))

		return h(msg)
	}
}

func SetUser(ctx context.Context, id string) {
	if hub := sentry.GetHubFromContext(ctx); hub != nil {
		hub.Scope().SetUser(sentry.User{ID: id})
	}
}

type stream struct {
	grpc.ServerStream

	ctx context.Context //nolint:containedctx // a wrapped stream must answer its own context
}

func (s *stream) Context() context.Context { return s.ctx }
