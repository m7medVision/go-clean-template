package middleware_test

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"strings"
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

func TestRecoveryTurnsAPanicIntoInternal(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer

	resp, err := grpcmw.Recovery(slog.New(slog.NewTextHandler(&logs, nil)))(fromClient(t.Context()), nil, unaryInfo(), panics)

	assert.Nil(t, resp)
	assert.Equal(t, codes.Internal, status.Code(err))
	assert.Equal(t, "internal server error", status.Convert(err).Message(), "the panic stays out of the answer")
	assert.Contains(t, logs.String(), "grpc panic")
	assert.Contains(t, logs.String(), "panic=boom")
	assert.Contains(t, logs.String(), "client=10.0.0.7")
	assert.Contains(t, logs.String(), "stack=")
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

	_, err := grpcmw.Logger(slog.New(slog.NewTextHandler(&logs, nil)))(fromClient(t.Context()), nil, unaryInfo(), notFound)

	assert.Equal(t, codes.NotFound, status.Code(err))
	assert.Contains(t, logs.String(), `msg="grpc request" client=10.0.0.7 method=`+_method+" code=NotFound")
}

func TestMetricsCountAndTimeEveryCall(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	m := grpcmw.NewMetrics(reg)

	_, err := m.Interceptor(t.Context(), nil, unaryInfo(), notFound)
	assert.Equal(t, codes.NotFound, status.Code(err))

	// Registering again, as a second server in the same process does, reuses the collectors.
	again := grpcmw.NewMetrics(reg)
	_, err = again.Interceptor(t.Context(), nil, unaryInfo(), notFound)
	assert.Equal(t, codes.NotFound, status.Code(err))

	require.NoError(t, testutil.GatherAndCompare(reg, strings.NewReader(`
# HELP grpc_requests_total Total number of gRPC calls served.
# TYPE grpc_requests_total counter
grpc_requests_total{code="NotFound",method="`+_method+`"} 2
`), "grpc_requests_total"))

	count, err := testutil.GatherAndCount(reg, "grpc_request_duration_seconds")
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}
