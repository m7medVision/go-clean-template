// Package graphql serves the GraphQL API, built with gqlgen. Each domain's resolvers live in their
// own folder (task/, exchange/); this package joins them into the root resolver and serves it.
package graphql

import (
	"context"
	"fmt"
	"net/http"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/generated"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/gqlerr"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/middleware"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/m7medvision/go-clean-template/pkg/logger"
)

const _playgroundTitle = "Go Clean Template GraphQL"

// NewHandler builds the GraphQL endpoint.
func NewHandler(b cqrs.Buses, jwtManager *jwt.Manager, l logger.Interface) http.Handler {
	srv := handler.New(generated.NewExecutableSchema(generated.Config{
		Resolvers: newResolver(b, l),
	}))

	srv.AddTransport(transport.POST{})
	srv.Use(extension.Introspection{})
	srv.SetRecoverFunc(func(_ context.Context, p any) error {
		l.Error(fmt.Sprintf("graphql - panic: %v", p))

		return gqlerr.Internal()
	})

	return middleware.WithOptionalCaller(jwtManager, srv)
}

// NewPlayground serves the GraphiQL playground, which sends its queries to endpoint.
func NewPlayground(endpoint string) http.Handler {
	return playground.Handler(_playgroundTitle, endpoint)
}
