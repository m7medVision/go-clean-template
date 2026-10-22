package integration_test

import (
	"context"
	"testing"

	protov1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const grpcURL = host + ":8081"

// registerAndLoginGRPC creates a unique user via gRPC and returns the JWT token.
func registerAndLoginGRPC(t *testing.T) string {
	t.Helper()

	name := sanitizeTestName(t)
	email := name + "@test.com"
	password := testPassword

	grpcConn, err := grpc.NewClient(grpcURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("registerAndLoginGRPC: grpc.NewClient: %v", err)
	}

	defer func() {
		if cerr := grpcConn.Close(); cerr != nil {
			t.Fatalf("registerAndLoginGRPC: grpcConn.Close: %v", cerr)
		}
	}()

	authClient := protov1.NewAuthServiceClient(grpcConn)

	_, err = authClient.Register(t.Context(), &protov1.RegisterRequest{
		Username: name,
		Email:    email,
		Password: password,
	})
	if err != nil {
		t.Fatalf("registerAndLoginGRPC: Register: %v", err)
	}

	loginResp, err := authClient.Login(t.Context(), &protov1.LoginRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		t.Fatalf("registerAndLoginGRPC: Login: %v", err)
	}

	return loginResp.Token
}

// grpcAuthCtx returns a context with gRPC authorization metadata.
func grpcAuthCtx(t *testing.T, token string) context.Context {
	t.Helper()

	return metadata.AppendToOutgoingContext(t.Context(), "authorization", token)
}

// requireListAllTasksDeniedGRPC fails the test unless the admin-only ListAllTasks refuses token with
// PermissionDenied.
func requireListAllTasksDeniedGRPC(t *testing.T, token string) {
	t.Helper()

	conn, err := grpc.NewClient(grpcURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}

	defer conn.Close()

	_, err = protov1.NewTaskServiceClient(conn).ListAllTasks(grpcAuthCtx(t, token), &protov1.ListTasksRequest{Limit: 10})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("ListAllTasks: expected PermissionDenied, got %v", err)
	}
}
