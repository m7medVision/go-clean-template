package integration_test

import (
	"testing"

	protov1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// gRPC: the task CRUD flow, every write read back at once.
func TestGRPCTaskV1(t *testing.T) {
	token := registerAndLoginGRPC(t)

	conn, err := grpc.NewClient(grpcURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	t.Cleanup(func() { assert.NoError(t, conn.Close()) })

	client := protov1.NewTaskServiceClient(conn)
	ctx := grpcAuthCtx(t, token)

	created, err := client.CreateTask(ctx, &protov1.CreateTaskRequest{Title: "grpc task", Description: "grpc description"})
	require.NoError(t, err)
	require.NotEmpty(t, created.GetId())
	assert.Equal(t, statusTodo, created.GetStatus())

	id := created.GetId()

	got, err := client.GetTask(ctx, &protov1.GetTaskRequest{Id: id})
	require.NoError(t, err)
	assert.Equal(t, id, got.GetId())

	listed, err := client.ListTasks(ctx, &protov1.ListTasksRequest{Limit: 10})
	require.NoError(t, err)

	ids := make([]string, 0, len(listed.GetTasks()))
	for _, task := range listed.GetTasks() {
		ids = append(ids, task.GetId())
	}

	assert.Contains(t, ids, id)

	updated, err := client.UpdateTask(ctx, &protov1.UpdateTaskRequest{
		Id: id, Title: "updated grpc task", Description: "updated grpc description",
	})
	require.NoError(t, err)
	assert.Equal(t, "updated grpc task", updated.GetTitle())

	moved, err := client.TransitionTask(ctx, &protov1.TransitionTaskRequest{Id: id, Status: statusInProgress})
	require.NoError(t, err)
	assert.Equal(t, statusInProgress, moved.GetStatus())

	_, err = client.DeleteTask(ctx, &protov1.DeleteTaskRequest{Id: id})
	require.NoError(t, err)
}
