package task_test

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"testing"
	"time"

	pb "github.com/m7medvision/go-clean-template/docs/proto/v1"
	grpcmw "github.com/m7medvision/go-clean-template/internal/controller/grpc/middleware"
	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/task"
	"github.com/m7medvision/go-clean-template/internal/entity"
	mongotask "github.com/m7medvision/go-clean-template/internal/entity/mongo/task"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/command"
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
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

const _bufSize = 1 << 20

// fakeTask answers ListAllTasks on real buses, and keeps what it was asked.
type fakeTask struct {
	listed    []mongotask.TaskWithAttributes
	stored    mongotask.TaskWithAttributes
	gotPatch  mongotask.AttributesPatch
	err       error
	gotCaller entity.Principal
	gotStatus *entity.TaskStatus
	gotLimit  int
	gotOffset int
}

func (f *fakeTask) buses() cqrs.Buses {
	cmds, qs := cqrs.NewCommands(), cqrs.NewQueries()
	cqrs.HandleQuery(qs, cqrs.HandlerFunc[query.ListAllTasks, entity.List[mongotask.TaskWithAttributes]](
		func(_ context.Context, q query.ListAllTasks) (entity.List[mongotask.TaskWithAttributes], error) {
			f.gotCaller, f.gotStatus, f.gotLimit, f.gotOffset = q.Principal, q.Status, q.Limit, q.Offset

			return entity.List[mongotask.TaskWithAttributes]{Items: f.listed, Total: len(f.listed)}, f.err
		}))
	cqrs.HandleQuery(qs, cqrs.HandlerFunc[query.GetTask, mongotask.TaskWithAttributes](
		func(context.Context, query.GetTask) (mongotask.TaskWithAttributes, error) { return f.stored, f.err }))
	cqrs.HandleCommand(cmds, cqrs.HandlerFunc[command.SetTaskAttributes, entity.Written](
		func(_ context.Context, c command.SetTaskAttributes) (entity.Written, error) {
			f.gotPatch = c.Patch

			return entity.Written{ID: c.TaskID}, f.err
		}))

	return cqrs.Buses{Commands: cmds, Queries: qs}
}

// dial serves the task routes behind the auth interceptor over an in-memory listener.
func dial(t *testing.T, tk *fakeTask) (pb.TaskServiceClient, *jwt.Manager) {
	t.Helper()

	manager := jwt.New("test-secret", time.Hour)
	lis := bufconn.Listen(_bufSize)
	srv := grpc.NewServer(grpc.UnaryInterceptor(grpcmw.Auth(manager)))
	pb.RegisterTaskServiceServer(srv, task.New(tk.buses(), slog.New(slog.DiscardHandler)))

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

	tk := &fakeTask{listed: []mongotask.TaskWithAttributes{
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

func TestUpdateTaskAttributesFollowsTheMask(t *testing.T) {
	t.Parallel()

	high := mongotask.PriorityHigh
	tk := &fakeTask{stored: mongotask.TaskWithAttributes{
		Task: entity.Task{ID: "task-1"}, Attributes: mongotask.Attributes{Priority: &high},
	}}
	client, manager := dial(t, tk)

	resp, err := client.UpdateTaskAttributes(signedIn(t, manager, []string{"user"}), &pb.UpdateTaskAttributesRequest{
		Id:         "task-1",
		Attributes: &pb.TaskAttributes{Priority: new("high"), Labels: []string{"ignored"}},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"priority", "estimate"}},
	})

	require.NoError(t, err)
	assert.Equal(t, mongotask.AttributesPatch{
		Priority: mongotask.SetTo(mongotask.PriorityHigh),
		Estimate: mongotask.Cleared[float64](),
	}, tk.gotPatch)
	assert.Equal(t, "high", resp.GetAttributes().GetPriority())
}

func TestUpdateTaskAttributesRejectsAnUnknownPath(t *testing.T) {
	t.Parallel()

	client, manager := dial(t, &fakeTask{})

	_, err := client.UpdateTaskAttributes(signedIn(t, manager, []string{"user"}), &pb.UpdateTaskAttributesRequest{
		Id: "task-1", UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"title"}},
	})

	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}
