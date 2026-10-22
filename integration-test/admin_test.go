package integration_test

import (
	"context"
	"net/http"
	"testing"
)

// A password sign-in grants only the user role, so the admin-only operations refuse it on every
// transport.
// The admin side runs against Keycloak in oidc_test.go. // scaffold:sso
func TestAdminTasksRefuseLocalUser(t *testing.T) {
	token := registerAndLogin(t)

	t.Run("REST", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), requestTimeout)
		defer cancel()

		resp, err := doAuthenticatedRequest(ctx, http.MethodGet, basePathV1+"/admin/tasks", http.NoBody, token)
		if err != nil {
			t.Fatalf("GET /v1/admin/tasks: %v", err)
		}

		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("GET /v1/admin/tasks: expected 403, got %d", resp.StatusCode)
		}
	})

	// scaffold:graphql:begin
	t.Run("GraphQL", func(t *testing.T) {
		resp := graphqlDo(t, token, `query { allTasks { total } }`, nil)

		if len(resp.Errors) == 0 || resp.Errors[0].Extensions["code"] != "FORBIDDEN" {
			t.Fatalf("allTasks: expected a FORBIDDEN error, got %+v", resp.Errors)
		}
	})
	// scaffold:graphql:end

	// scaffold:grpc:begin
	t.Run("gRPC", func(t *testing.T) {
		requireListAllTasksDeniedGRPC(t, token)
	})
	// scaffold:grpc:end
}
