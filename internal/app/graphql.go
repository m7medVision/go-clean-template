package app

import (
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
)

// mountGraphQL serves GraphQL at /graphql on the HTTP router, and GraphiQL when docs are on.
func mountGraphQL(router chi.Router, docs bool, uc *useCases, jwtManager *jwt.Manager, l *slog.Logger) {
	router.Handle("/graphql", graphql.NewHandler(uc.buses, jwtManager, l))

	if docs {
		router.Handle("/graphql/playground", graphql.NewPlayground("/graphql"))
	}
}
