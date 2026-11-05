package edit_test

import (
	"testing"

	"github.com/m7medvision/go-clean-template/tools/gen/edit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const taskProto = `syntax = "proto3";

package grpc.v1;

service TaskService {
  rpc CreateTask (CreateTaskRequest) returns (TaskResponse);
  // Every user's tasks. A } in a comment is not the end.
  rpc ListAllTasks (ListTasksRequest) returns (ListTasksResponse);
}

message CreateTaskRequest {
  string title = 1;
}
`

func parseProto(t *testing.T, src string) *edit.Proto {
	t.Helper()

	p, err := edit.ParseProto("task.proto", []byte(src))
	require.NoError(t, err)

	return p
}

func TestInServiceAddsAnRPCAndAtEndItsMessages(t *testing.T) {
	t.Parallel()

	p := parseProto(t, taskProto)

	rpc, err := p.InService("TaskService", "rpc ArchiveTask (ArchiveTaskRequest) returns (ArchiveTaskResponse);")
	require.NoError(t, err)

	msgs := p.AtEnd("message ArchiveTaskRequest {\n  string id = 1;\n}\n\nmessage ArchiveTaskResponse {}")
	got := edit.Apply([]byte(taskProto), []edit.Insertion{rpc, msgs})

	assert.Equal(t, `syntax = "proto3";

package grpc.v1;

service TaskService {
  rpc CreateTask (CreateTaskRequest) returns (TaskResponse);
  // Every user's tasks. A } in a comment is not the end.
  rpc ListAllTasks (ListTasksRequest) returns (ListTasksResponse);
  rpc ArchiveTask (ArchiveTaskRequest) returns (ArchiveTaskResponse);
}

message CreateTaskRequest {
  string title = 1;
}

message ArchiveTaskRequest {
  string id = 1;
}

message ArchiveTaskResponse {}
`, string(got))

	_, err = edit.ParseProto("task.proto", got)
	require.NoError(t, err)
}

func TestInServiceNeedsExactlyOneService(t *testing.T) {
	t.Parallel()

	_, err := parseProto(t, taskProto).InService("WidgetService", "rpc X (A) returns (B);")
	require.ErrorIs(t, err, edit.ErrAnchor, "missing")

	twice := taskProto + "\nservice TaskService {\n}\n"
	_, err = parseProto(t, twice).InService("TaskService", "rpc X (A) returns (B);")
	require.ErrorIs(t, err, edit.ErrAnchor, "twice")
}

func TestProtoListsItsServicesAndMessages(t *testing.T) {
	t.Parallel()

	p := parseProto(t, taskProto)

	assert.Equal(t, []string{"TaskService"}, p.Services())
	assert.Equal(t, []string{"CreateTaskRequest"}, p.Messages())
	assert.Equal(t, []string{"TaskService.CreateTask", "TaskService.ListAllTasks"}, p.RPCs())
}

func TestParseProtoRejectsBrokenSource(t *testing.T) {
	t.Parallel()

	_, err := edit.ParseProto("x.proto", []byte("service {"))
	require.Error(t, err)
}
