package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/google/uuid"
	protov1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	mongotask "github.com/m7medvision/go-clean-template/internal/entity/mongo/task"
	"github.com/m7medvision/go-clean-template/internal/repo"
	mongoTaskRepo "github.com/m7medvision/go-clean-template/internal/repo/mongo/task"
	"github.com/m7medvision/go-clean-template/pkg/mongo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

type attributesResponse struct {
	Priority *string  `json:"priority"`
	Estimate *float64 `json:"estimate"`
	Labels   []string `json:"labels"`
}

type taskWithAttributes struct {
	ID         string             `json:"id"`
	Attributes attributesResponse `json:"attributes"`
}

func attributesStore(t *testing.T) repo.TaskAttributesStore {
	t.Helper()

	mongoURL := os.Getenv("MONGO_URL")
	require.NotEmpty(t, mongoURL, "MONGO_URL is not set")

	ctx, cancel := context.WithTimeout(t.Context(), connectWait)
	defer cancel()

	db, disconnect, err := mongo.New(ctx, mongoURL)
	require.NoError(t, err)

	t.Cleanup(func() { assert.NoError(t, disconnect(context.WithoutCancel(t.Context()))) })

	return mongoTaskRepo.NewAttributesStore(db)
}

func TestTaskAttributesStorePatchesGetsAndDeletes(t *testing.T) {
	store := attributesStore(t)
	ctx := t.Context()
	id, other := "_test-"+uuid.NewString(), "_test-"+uuid.NewString()

	require.NoError(t, store.Patch(ctx, id, mongotask.AttributesPatch{
		Priority: mongotask.SetTo(mongotask.PriorityHigh),
		Estimate: mongotask.SetTo(2.5),
		Labels:   mongotask.SetTo([]string{"a", "b"}),
	}))
	require.NoError(t, store.Patch(ctx, id, mongotask.AttributesPatch{Estimate: mongotask.Cleared[float64]()}))

	got, err := store.Get(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, got.Priority)
	assert.Equal(t, mongotask.PriorityHigh, *got.Priority)
	assert.Nil(t, got.Estimate)
	assert.Equal(t, []string{"a", "b"}, got.Labels)

	many, err := store.GetMany(ctx, []string{id, other})
	require.NoError(t, err)
	assert.Len(t, many, 1)
	assert.Contains(t, many, id)

	require.NoError(t, store.Delete(ctx, id))
	require.NoError(t, store.Delete(ctx, id))

	got, err = store.Get(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, mongotask.Attributes{}, got)
}

func TestHTTPTaskAttributes(t *testing.T) {
	token := registerAndLogin(t)
	created := httpCreateTask(t, token, "attributes task", "")

	patched := httpSend[taskWithAttributes](t, token, http.MethodPatch, "/tasks/"+created.ID+"/attributes",
		`{"priority":"high","estimate":3,"labels":["backend"]}`, http.StatusOK)
	require.NotNil(t, patched.Attributes.Priority)
	assert.Equal(t, "high", *patched.Attributes.Priority)

	kept := httpSend[taskWithAttributes](t, token, http.MethodPatch, "/tasks/"+created.ID+"/attributes",
		`{"estimate":null}`, http.StatusOK)
	assert.Nil(t, kept.Attributes.Estimate)
	assert.Equal(t, []string{"backend"}, kept.Attributes.Labels)

	got := httpGetJSON[taskWithAttributes](t, token, "/tasks/"+created.ID)
	require.NotNil(t, got.Attributes.Priority)
	assert.Equal(t, "high", *got.Attributes.Priority)

	httpSend[json.RawMessage](t, token, http.MethodPatch, "/tasks/"+created.ID+"/attributes",
		`{"priority":"urgent"}`, http.StatusUnprocessableEntity)

	t.Run("other", func(t *testing.T) {
		httpSend[json.RawMessage](t, registerAndLogin(t), http.MethodPatch, "/tasks/"+created.ID+"/attributes",
			`{"priority":"low"}`, http.StatusForbidden)
	})

	httpSend[json.RawMessage](t, token, http.MethodDelete, "/tasks/"+created.ID, "", http.StatusNoContent)

	left, err := attributesStore(t).Get(t.Context(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, mongotask.Attributes{}, left)
}

func TestGRPCTaskAttributes(t *testing.T) {
	token := registerAndLoginGRPC(t)

	conn, err := grpc.NewClient(grpcURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	t.Cleanup(func() { assert.NoError(t, conn.Close()) })

	client := protov1.NewTaskServiceClient(conn)
	ctx := grpcAuthCtx(t, token)

	created, err := client.CreateTask(ctx, &protov1.CreateTaskRequest{Title: "grpc attributes"})
	require.NoError(t, err)

	resp, err := client.UpdateTaskAttributes(ctx, &protov1.UpdateTaskAttributesRequest{
		Id:         created.GetId(),
		Attributes: &protov1.TaskAttributes{Priority: new("medium"), Labels: []string{"x"}},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"priority", "labels"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "medium", resp.GetAttributes().GetPriority())

	got, err := client.GetTask(ctx, &protov1.GetTaskRequest{Id: created.GetId()})
	require.NoError(t, err)
	assert.Equal(t, []string{"x"}, got.GetAttributes().GetLabels())
}

func TestGraphQLTaskAttributes(t *testing.T) {
	token := registerAndLogin(t)
	created := httpCreateTask(t, token, "graphql attributes", "")

	body, err := json.Marshal(map[string]any{
		"query":     `mutation($id: ID!) { updateTaskAttributes(id: $id, input: {priority: LOW, labels: ["ui"]}) { id attributes { priority labels } } }`,
		"variables": map[string]any{"id": created.ID},
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), requestTimeout)
	defer cancel()

	resp, err := doAuthenticatedRequest(ctx, http.MethodPost, graphqlURL, bytes.NewReader(body), token)
	require.NoError(t, err)

	defer resp.Body.Close()

	out := parseJSON[struct {
		Data struct {
			UpdateTaskAttributes struct {
				Attributes struct {
					Priority string   `json:"priority"`
					Labels   []string `json:"labels"`
				} `json:"attributes"`
			} `json:"updateTaskAttributes"`
		} `json:"data"`
	}](t, resp)

	assert.Equal(t, "LOW", out.Data.UpdateTaskAttributes.Attributes.Priority)
	assert.Equal(t, []string{"ui"}, out.Data.UpdateTaskAttributes.Attributes.Labels)
}
