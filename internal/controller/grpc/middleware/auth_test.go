package middleware_test

import (
	"context"
	"testing"
	"time"

	pb "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/internal/controller/authn"
	grpcmw "github.com/m7medvision/go-clean-template/internal/controller/grpc/middleware"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const respOK = "ok"

// callerCapture is a unary handler that keeps the caller its context carries.
type callerCapture struct {
	called bool
	caller authn.Caller
	ok     bool
}

func (c *callerCapture) handle(ctx context.Context, _ any) (any, error) {
	c.called = true
	c.caller, c.ok = authn.CallerFrom(ctx)

	return respOK, nil
}

// call runs a call of method with incoming metadata md, unless it is nil, through Auth with
// Register public.
func call(t *testing.T, tokens *jwt.Manager, method string, md metadata.MD) (*callerCapture, error) {
	t.Helper()

	ctx := t.Context()
	if md != nil {
		ctx = metadata.NewIncomingContext(ctx, md)
	}

	capture := &callerCapture{}
	auth := grpcmw.Auth(tokens, pb.AuthService_Register_FullMethodName)

	resp, err := auth(ctx, nil, &grpc.UnaryServerInfo{FullMethod: method}, capture.handle)
	if err == nil {
		assert.Equal(t, respOK, resp)
	}

	return capture, err
}

func TestAuthLetsAPublicMethodThrough(t *testing.T) {
	t.Parallel()

	capture, err := call(t, jwt.New("test-secret", time.Hour), pb.AuthService_Register_FullMethodName, nil)

	require.NoError(t, err)
	assert.True(t, capture.called)
	assert.False(t, capture.ok, "a public method gets no caller")
}

func TestAuthRejects(t *testing.T) {
	t.Parallel()

	tokens := jwt.New("test-secret", time.Hour)

	tests := []struct {
		name string
		md   metadata.MD
	}{
		{"no metadata", nil},
		{"no authorization", metadata.Pairs("other-key", "value")},
		{"invalid token", metadata.Pairs("authorization", "invalid-token")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			capture, err := call(t, tokens, pb.TaskService_GetTask_FullMethodName, tc.md)

			assert.False(t, capture.called)
			assert.Equal(t, codes.Unauthenticated, status.Code(err))
		})
	}
}

func TestAuthRunsTheMethodWithTheTokensCaller(t *testing.T) {
	t.Parallel()

	tokens := jwt.New("test-secret", time.Hour)

	token, err := tokens.GenerateToken("user-42", []string{"admin"})
	require.NoError(t, err)

	capture, err := call(t, tokens, pb.TaskService_ListAllTasks_FullMethodName, metadata.Pairs("authorization", token))
	require.NoError(t, err)

	require.True(t, capture.ok)
	assert.Equal(t, entity.Principal{UserID: "user-42", Roles: []entity.Role{entity.RoleAdmin}}, capture.caller.Principal)
}
