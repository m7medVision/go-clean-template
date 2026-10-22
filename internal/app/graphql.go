package app

import (
	"github.com/go-chi/chi/v5"
	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/m7medvision/go-clean-template/pkg/logger"
)

// mountGraphQL serves GraphQL at /graphql on the HTTP router, and GraphiQL when docs are on.
func mountGraphQL(router chi.Router, cfg *config.Config, uc *useCases, jwtManager *jwt.Manager, l logger.Interface) {
	router.Handle("/graphql", graphql.NewHandler(uc.task, uc.exchange, jwtManager, l))

	if cfg.Docs.Enabled {
		router.Handle("/graphql/playground", graphql.NewPlayground("/graphql"))
	}
}
