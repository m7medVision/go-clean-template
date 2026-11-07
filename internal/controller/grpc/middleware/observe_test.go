package middleware_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"testing"

	grpcmw "github.com/m7medvision/go-clean-template/internal/controller/grpc/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

const _method = "/grpc.v1.TaskService/GetTask"

func unaryInfo() *grpc.UnaryServerInfo { return &grpc.UnaryServerInfo{FullMethod: _method} }

func fromClient(ctx context.Context) context.Context {
	return peer.NewContext(ctx, &peer.Peer{Addr: &net.TCPAddr{IP: net.IPv4(10, 0, 0, 7), Port: 51000}})
}

func panics(context.Context, any) (any, error) { panic("boom") }

func notFound(context.Context, any) (any, error) {
	return nil, status.Error(codes.NotFound, "task not found")
}

// onlyRecord decodes the one JSON log record in logs.
func onlyRecord(t *testing.T, logs *bytes.Buffer) map[string]any {
	t.Helper()

	var records []map[string]any

	dec := json.NewDecoder(logs)
	for dec.More() {
		var r map[string]any
		require.NoError(t, dec.Decode(&r))

		records = append(records, r)
	}

	require.Len(t, records, 1)

	return records[0]
}

// callCount reads grpc_requests_total for method and code from reg.
func callCount(t *testing.T, reg *prometheus.Registry, method, code string) float64 {
	t.Helper()

	families, err := reg.Gather()
	require.NoError(t, err)

	for _, f := range families {
		if f.GetName() != "grpc_requests_total" {
			continue
		}

		for _, m := range f.GetMetric() {
			labels := map[string]string{}
			for _, l := range m.GetLabel() {
				labels[l.GetName()] = l.GetValue()
			}

			if labels["method"] == method && labels["code"] == code {
				return m.GetCounter().GetValue()
			}
		}
	}

	return 0
}

func TestRecoveryTurnsAPanicIntoInternal(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer

	resp, err := grpcmw.Recovery(slog.New(slog.NewJSONHandler(&logs, nil)))(fromClient(t.Context()), nil, unaryInfo(), panics)

	assert.Nil(t, resp)
	assert.Equal(t, codes.Internal, status.Code(err))
	assert.Equal(t, "internal server error", status.Convert(err).Message(), "the panic stays out of the answer")

	r := onlyRecord(t, &logs)
	assert.Equal(t, slog.LevelError.String(), r["level"])
	assert.Equal(t, "boom", r["panic"])
	assert.Equal(t, "10.0.0.7", r["client"])
	assert.Equal(t, _method, r["method"])
	assert.NotEmpty(t, r["stack"])
}

func TestRecoveryPassesAnAnswerThrough(t *testing.T) {
	t.Parallel()

	_, err := grpcmw.Recovery(slog.New(slog.DiscardHandler))(t.Context(), nil, unaryInfo(), notFound)

	assert.Equal(t, codes.NotFound, status.Code(err))
}

type panickingStream struct{ grpc.ServerStream }

func (panickingStream) Context() context.Context { return context.Background() }

func TestStreamRecoveryTurnsAPanicIntoInternal(t *testing.T) {
	t.Parallel()

	err := grpcmw.StreamRecovery(slog.New(slog.DiscardHandler))(nil, panickingStream{},
		&grpc.StreamServerInfo{FullMethod: "/grpc.reflection.v1.ServerReflection/ServerReflectionInfo"},
		func(any, grpc.ServerStream) error { panic("boom") })

	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestLoggerLogsEveryCall(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer

	_, err := grpcmw.Logger(slog.New(slog.NewJSONHandler(&logs, nil)))(fromClient(t.Context()), nil, unaryInfo(), notFound)

	assert.Equal(t, codes.NotFound, status.Code(err))

	r := onlyRecord(t, &logs)
	assert.Equal(t, "10.0.0.7", r["client"])
	assert.Equal(t, _method, r["method"])
	assert.Equal(t, codes.NotFound.String(), r["code"])
}

func TestMetricsCountAndTimeEveryCall(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	m := grpcmw.NewMetrics(reg)

	for range 2 {
		_, err := m.Interceptor(t.Context(), nil, unaryInfo(), notFound)
		assert.Equal(t, codes.NotFound, status.Code(err))
	}

	assert.Equal(t, 2.0, callCount(t, reg, _method, codes.NotFound.String()))

	count, err := testutil.GatherAndCount(reg, "grpc_request_duration_seconds")
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}
