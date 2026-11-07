package integration_test

import (
	"testing"

	protov1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// gRPC: AuthService Register and Login.
func TestGRPCAuthRegisterLoginV1(t *testing.T) {
	name := sanitizeTestName(t)
	email := name + "@test.com"
	password := testPassword

	grpcConn, err := grpc.NewClient(grpcURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}

	defer func() {
		if cerr := grpcConn.Close(); cerr != nil {
			t.Fatalf("grpcConn.Close: %v", cerr)
		}
	}()

	authClient := protov1.NewAuthServiceClient(grpcConn)

	regResp, err := authClient.Register(t.Context(), &protov1.RegisterRequest{
		Username: name,
		Email:    email,
		Password: password,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if regResp.GetId() == "" {
		t.Error("Expected non-empty Id from Register")
	}

	if regResp.GetUsername() != name {
		t.Errorf("Expected username %q, got %q", name, regResp.GetUsername())
	}

	loginResp, err := authClient.Login(t.Context(), &protov1.LoginRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	if loginResp.GetToken() == "" {
		t.Error("Expected non-empty Token from Login")
	}
}

// gRPC: Register checks the same rules REST does, though gRPC has no schema to check them first.
func TestGRPCRegisterRefusesInvalidInputV1(t *testing.T) {
	grpcConn, err := grpc.NewClient(grpcURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}

	defer func() {
		if cerr := grpcConn.Close(); cerr != nil {
			t.Fatalf("grpcConn.Close: %v", cerr)
		}
	}()

	authClient := protov1.NewAuthServiceClient(grpcConn)
	name := sanitizeTestName(t)

	for _, req := range []*protov1.RegisterRequest{
		{Username: name, Email: "", Password: testPassword},
		{Username: name, Email: name + "@test.com", Password: ""},
		{Username: "ab", Email: name + "@test.com", Password: testPassword},
	} {
		_, err := authClient.Register(t.Context(), req)
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("Register(%v): want InvalidArgument, got %v", req, err)
		}
	}
}

// gRPC: AuthService GetProfile.
func TestGRPCAuthProfileV1(t *testing.T) {
	token := registerAndLoginGRPC(t)

	grpcConn, err := grpc.NewClient(grpcURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}

	defer func() {
		if cerr := grpcConn.Close(); cerr != nil {
			t.Fatalf("grpcConn.Close: %v", cerr)
		}
	}()

	authClient := protov1.NewAuthServiceClient(grpcConn)

	profileResp, err := authClient.GetProfile(grpcAuthCtx(t, token), &protov1.GetProfileRequest{})
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}

	if profileResp.GetUsername() == "" {
		t.Error("Expected non-empty username")
	}

	if profileResp.GetEmail() == "" {
		t.Error("Expected non-empty email")
	}
}
