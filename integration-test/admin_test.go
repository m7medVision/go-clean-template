package integration_test

import (
	"context"
	"net/http"
	"testing"

	protov1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

const _allTasksQuery = `query { allTasks { total } }`

// A password sign-in grants only the user role, so the admin-only operations refuse it on every
// transport. The admin side runs against Keycloak in oidc_test.go.
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

	t.Run("GraphQL", func(t *testing.T) {
		resp := graphqlDo(t, token, _allTasksQuery, nil)

		if len(resp.Errors) == 0 || resp.Errors[0].Extensions["code"] != "FORBIDDEN" {
			t.Fatalf("allTasks: expected a FORBIDDEN error, got %+v", resp.Errors)
		}
	})

	t.Run("gRPC", func(t *testing.T) {
		conn, err := grpc.NewClient(grpcURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			t.Fatalf("grpc.NewClient: %v", err)
		}

		defer conn.Close()

		_, err = protov1.NewTaskServiceClient(conn).ListAllTasks(grpcAuthCtx(t, token), &protov1.ListTasksRequest{Limit: 10})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("ListAllTasks: expected PermissionDenied, got %v", err)
		}
	})
}
