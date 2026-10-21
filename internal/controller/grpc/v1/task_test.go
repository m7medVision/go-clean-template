package v1_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	pb "github.com/m7medvision/go-clean-template/docs/proto/v1"
	grpcmw "github.com/m7medvision/go-clean-template/internal/controller/grpc/middleware"
	v1 "github.com/m7medvision/go-clean-template/internal/controller/grpc/v1"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase"
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

type noopLogger struct{}

func (noopLogger) Debug(any, ...any)   {}
func (noopLogger) Info(string, ...any) {}
func (noopLogger) Warn(string, ...any) {}
func (noopLogger) Error(any, ...any)   {}
func (noopLogger) Fatal(any, ...any)   {}

type fakeTask struct {
	usecase.Task

	listed    []entity.Task
	err       error
	gotCaller entity.Principal
	gotStatus *entity.TaskStatus
	gotLimit  int
	gotOffset int
}

func (f *fakeTask) ListAll(
	_ context.Context, caller entity.Principal, taskStatus *entity.TaskStatus, limit, offset int,
) ([]entity.Task, int, error) {
	f.gotCaller, f.gotStatus, f.gotLimit, f.gotOffset = caller, taskStatus, limit, offset

	return f.listed, len(f.listed), f.err
}

// dial serves the task routes behind the auth interceptor over an in-memory listener.
func dial(t *testing.T, tk usecase.Task) (pb.TaskServiceClient, *jwt.Manager) {
	t.Helper()

	manager := jwt.New("test-secret", time.Hour)
	lis := bufconn.Listen(_bufSize)
	srv := grpc.NewServer(grpc.UnaryInterceptor(grpcmw.AuthInterceptor(manager)))
	v1.NewTaskRoutes(srv, tk, noopLogger{})

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

	return pb.NewTaskServiceClient(conn), manager
}

func signedIn(t *testing.T, manager *jwt.Manager, roles []string) context.Context {
	t.Helper()

	token, err := manager.GenerateToken("caller-1", roles)
	require.NoError(t, err)

	return metadata.AppendToOutgoingContext(t.Context(), "authorization", token)
}

func TestListAllTasks(t *testing.T) {
	t.Parallel()

	tk := &fakeTask{listed: []entity.Task{
		{ID: "task-1", UserID: "alice", Status: entity.TaskStatusDone},
		{ID: "task-2", UserID: "bob", Status: entity.TaskStatusDone},
	}}
	client, manager := dial(t, tk)

	resp, err := client.ListAllTasks(signedIn(t, manager, []string{"admin"}), &pb.ListTasksRequest{
		Status: "done", Limit: 5, Offset: 10,
	})

	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.GetTotal())
	require.Len(t, resp.GetTasks(), 2)
	assert.Equal(t, "bob", resp.GetTasks()[1].GetUserId())
	assert.Equal(t, entity.Principal{UserID: "caller-1", Roles: []entity.Role{entity.RoleAdmin}}, tk.gotCaller)
	require.NotNil(t, tk.gotStatus)
	assert.Equal(t, entity.TaskStatusDone, *tk.gotStatus)
	assert.Equal(t, 5, tk.gotLimit)
	assert.Equal(t, 10, tk.gotOffset)
}

func TestListAllTasksAdminRequired(t *testing.T) {
	t.Parallel()

	client, manager := dial(t, &fakeTask{err: entity.ErrAdminRequired})

	_, err := client.ListAllTasks(signedIn(t, manager, []string{"user"}), &pb.ListTasksRequest{Limit: 10})

	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestListAllTasksInvalidStatus(t *testing.T) {
	t.Parallel()

	client, manager := dial(t, &fakeTask{})

	_, err := client.ListAllTasks(signedIn(t, manager, []string{"admin"}), &pb.ListTasksRequest{Status: "archived"})

	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}
