package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type flagValues struct {
	TaskAttributes bool `json:"taskAttributes"`
}

// The dev flags file turns task-attributes on, and both transports answer what the handlers use.
func TestFlagsAnswerTheSameOverRESTAndGraphQL(t *testing.T) {
	token := registerAndLogin(t)

	ctx, cancel := context.WithTimeout(t.Context(), requestTimeout)
	defer cancel()

	resp, err := doAuthenticatedRequest(ctx, http.MethodGet, basePathV1+"/flags", http.NoBody, token)
	require.NoError(t, err)

	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	rest := parseJSON[flagValues](t, resp)

	body, err := json.Marshal(map[string]any{"query": `query { flags { taskAttributes } }`})
	require.NoError(t, err)

	gqlResp, err := doAuthenticatedRequest(ctx, http.MethodPost, graphqlURL, bytes.NewReader(body), token)
	require.NoError(t, err)

	defer gqlResp.Body.Close()

	gql := parseJSON[struct {
		Data struct {
			Flags flagValues `json:"flags"`
		} `json:"data"`
	}](t, gqlResp)

	assert.True(t, rest.TaskAttributes)
	assert.Equal(t, rest, gql.Data.Flags)
}
