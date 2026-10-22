package integration_test

import (
	"testing"

	protov1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// gRPC: create and verify task.
func TestGRPCTaskCreateV1(t *testing.T) {
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

	taskClient := protov1.NewTaskServiceClient(grpcConn)
	authCtx := grpcAuthCtx(t, token)

	createResp, err := taskClient.CreateTask(authCtx, &protov1.CreateTaskRequest{
		Title:       "grpc task",
		Description: "grpc description",
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	if createResp.GetId() == "" {
		t.Fatal("expected non-empty id")
	}

	if createResp.GetStatus() != statusTodo {
		t.Errorf("expected status 'todo', got %q", createResp.GetStatus())
	}
}

// gRPC: get and list tasks.
func TestGRPCTaskGetListV1(t *testing.T) {
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

	taskClient := protov1.NewTaskServiceClient(grpcConn)
	authCtx := grpcAuthCtx(t, token)

	createResp, err := taskClient.CreateTask(authCtx, &protov1.CreateTaskRequest{
		Title:       "grpc get-list task",
		Description: "grpc description",
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	taskID := createResp.GetId()

	getResp, err := taskClient.GetTask(authCtx, &protov1.GetTaskRequest{Id: taskID})
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}

	if getResp.GetId() != taskID {
		t.Errorf("expected id %q, got %q", taskID, getResp.GetId())
	}

	listResp, err := taskClient.ListTasks(authCtx, &protov1.ListTasksRequest{
		Status: "",
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}

	if listResp.GetTotal() < 1 {
		t.Errorf("expected total >= 1, got %d", listResp.GetTotal())
	}
}

// gRPC: update, transition, and delete task.
func TestGRPCTaskUpdateTransitionDeleteV1(t *testing.T) {
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

	taskClient := protov1.NewTaskServiceClient(grpcConn)
	authCtx := grpcAuthCtx(t, token)

	createResp, err := taskClient.CreateTask(authCtx, &protov1.CreateTaskRequest{
		Title:       "grpc update task",
		Description: "grpc description",
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	taskID := createResp.GetId()

	updateResp, err := taskClient.UpdateTask(authCtx, &protov1.UpdateTaskRequest{
		Id:          taskID,
		Title:       "updated grpc task",
		Description: "updated grpc description",
	})
	if err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}

	if updateResp.GetTitle() != "updated grpc task" {
		t.Errorf("expected title 'updated grpc task', got %q", updateResp.GetTitle())
	}

	transResp, err := taskClient.TransitionTask(authCtx, &protov1.TransitionTaskRequest{
		Id:     taskID,
		Status: statusInProgress,
	})
	if err != nil {
		t.Fatalf("TransitionTask: %v", err)
	}

	if transResp.GetStatus() != statusInProgress {
		t.Errorf("expected status 'in_progress', got %q", transResp.GetStatus())
	}

	_, err = taskClient.DeleteTask(authCtx, &protov1.DeleteTaskRequest{Id: taskID})
	if err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
}
