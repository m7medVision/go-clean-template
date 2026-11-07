// Package graphql serves the GraphQL API, built with gqlgen. Each domain's resolvers live in their
// own folder (task/, exchange/); this package joins them into the root resolver and serves it.
package graphql

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	gqlgen "github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/generated"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/gqlerr"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/middleware"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
)

const _playgroundTitle = "Go Clean Template GraphQL"

func NewHandler(b cqrs.Buses, jwtManager *jwt.Manager, l *slog.Logger) http.Handler {
	srv := handler.New(generated.NewExecutableSchema(generated.Config{
		Resolvers: newResolver(b, l),
	}))

	srv.AddTransport(transport.POST{})
	srv.Use(extension.Introspection{})
	srv.SetRecoverFunc(func(ctx context.Context, p any) error {
		attrs := []any{"panic", fmt.Sprint(p), "stack", string(debug.Stack())}
		if fc := gqlgen.GetFieldContext(ctx); fc != nil {
			attrs = append(attrs, "path", fc.Path().String())
		}

		l.ErrorContext(ctx, "graphql panic", attrs...)

		return gqlerr.Internal()
	})

	return middleware.WithOptionalCaller(jwtManager, srv)
}

// NewPlayground serves the GraphiQL playground, which sends its queries to endpoint.
func NewPlayground(endpoint string) http.Handler {
	return playground.Handler(_playgroundTitle, endpoint)
}
