// Package graphql serves task CRUD over GraphQL, built with gqlgen.
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
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/m7medvision/go-clean-template/pkg/logger"
)

const _playgroundTitle = "Go Clean Template GraphQL"

// NewHandler builds the GraphQL endpoint.
func NewHandler(tk usecase.Task, ex usecase.Exchange, jwtManager *jwt.Manager, l logger.Interface) http.Handler {
	srv := handler.New(generated.NewExecutableSchema(generated.Config{
		Resolvers: &Resolver{task: tk, exchange: ex, l: l},
	}))

	srv.AddTransport(transport.POST{})
	srv.Use(extension.Introspection{})
	srv.SetRecoverFunc(func(_ context.Context, p any) error {
		l.Error(fmt.Sprintf("graphql - panic: %v", p))

		return newError(_codeInternal, _msgInternal)
	})

	return withOptionalCaller(jwtManager, srv)
}

// NewPlayground serves the GraphiQL playground, which sends its queries to endpoint.
func NewPlayground(endpoint string) http.Handler {
	return playground.Handler(_playgroundTitle, endpoint)
}
