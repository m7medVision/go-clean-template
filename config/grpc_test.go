package config_test

import (
	"testing"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GRPC_PORT has a default so that only the gRPC transport's own code knows about it.
//
//nolint:paralleltest // t.Setenv (in setRequired) cannot run in parallel.
func TestNewConfig_GRPCPortDefault(t *testing.T) {
	setRequired(t)

	cfg, err := config.NewConfig()
	require.NoError(t, err)

	assert.Equal(t, "8081", cfg.GRPC.Port)
}
