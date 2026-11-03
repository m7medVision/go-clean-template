package restapi

import (
	"encoding/json"
	"fmt"

	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	v1 "github.com/m7medvision/go-clean-template/internal/controller/restapi/v1"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// OpenAPI returns the REST OpenAPI document without starting a server. Registering
// operations never calls the use cases, so nil dependencies are safe here.
func OpenAPI(version string) ([]byte, error) {
	api := humachi.New(chi.NewRouter(), newAPIConfig(version))
	v1.NewRoutes(api, cqrs.Buses{}, nil, nil)

	for _, opt := range []Option{
		WithPasskeys(nil),
		WithOIDC(nil),
	} {
		opt(api, nil)
	}

	b, err := json.MarshalIndent(api.OpenAPI(), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("restapi - OpenAPI - json.MarshalIndent: %w", err)
	}

	return b, nil
}
