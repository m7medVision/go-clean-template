package restapi

import (
	"encoding/json"
	"fmt"

	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	v1 "github.com/evrone/go-clean-template/internal/controller/restapi/v1"
	"github.com/go-chi/chi/v5"
)

// OpenAPI returns the REST OpenAPI document without starting a server. Registering
// operations never calls the use cases, so nil dependencies are safe here.
func OpenAPI(version string) ([]byte, error) {
	api := humachi.New(chi.NewRouter(), newAPIConfig(version))
	v1.NewRoutes(api, nil, nil, nil, nil)

	b, err := json.MarshalIndent(api.OpenAPI(), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("restapi - OpenAPI - json.MarshalIndent: %w", err)
	}

	return b, nil
}
