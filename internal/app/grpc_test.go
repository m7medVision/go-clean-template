package app

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/config"
	pb "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const _bufSize = 1 << 20

// dialGRPC serves the app's gRPC server over b, with metrics on, on an in-memory listener, and
// returns a connection to it.
func dialGRPC(t *testing.T, b cqrs.Buses, tokens *jwt.Manager) *grpc.ClientConn {
	t.Helper()

	lis := bufconn.Listen(_bufSize)
	srv := newGRPCServer(&config.Config{Metrics: config.Metrics{Enabled: true}}, b, tokens, slog.New(slog.DiscardHandler))

	go func() {
		if err := srv.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			t.Errorf("grpc serve: %v", err)
		}
	}()

	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	return conn
}

// Only signing up and signing in are called without a token.
func TestGRPCPublicMethodsAreTheSignInOnes(t *testing.T) {
	t.Parallel()

	conn := dialGRPC(t, newTestBuses(), jwt.New("test-secret", time.Hour))
	users, tasks := pb.NewAuthServiceClient(conn), pb.NewTaskServiceClient(conn)

	_, err := users.Login(t.Context(), &pb.LoginRequest{})
	assert.NotEqual(t, "missing authorization token", status.Convert(err).Message(), "Login is public")

	_, err = users.Register(t.Context(), &pb.RegisterRequest{})
	assert.NotEqual(t, "missing authorization token", status.Convert(err).Message(), "Register is public")

	_, err = users.GetProfile(t.Context(), &pb.GetProfileRequest{})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))

	_, err = tasks.ListTasks(t.Context(), &pb.ListTasksRequest{})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// A panic in a call answers Internal and leaves the server serving.
func TestGRPCRecoversFromAPanic(t *testing.T) {
	t.Parallel()

	b := newTestBuses()
	cqrs.HandleQuery(b.Queries, cqrs.HandlerFunc[query.GetTask, entity.Task](
		func(context.Context, query.GetTask) (entity.Task, error) { panic("boom") }))

	tokens := jwt.New("test-secret", time.Hour)

	token, err := tokens.GenerateToken("user-1", nil)
	require.NoError(t, err)

	tasks := pb.NewTaskServiceClient(dialGRPC(t, b, tokens))
	ctx := metadata.AppendToOutgoingContext(t.Context(), "authorization", token)

	for range 2 {
		_, err = tasks.GetTask(ctx, &pb.GetTaskRequest{Id: "task-1"})
		assert.Equal(t, codes.Internal, status.Code(err))
		assert.Equal(t, "internal server error", status.Convert(err).Message())
	}
}
