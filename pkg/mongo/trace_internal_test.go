package mongo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func TestServer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   string
		want []attribute.KeyValue
	}{
		{"host and port", "mongo:27017[-12]", []attribute.KeyValue{semconv.ServerAddress("mongo"), semconv.ServerPort(27017)}},
		{"ipv6", "[::1]:27017[-3]", []attribute.KeyValue{semconv.ServerAddress("::1"), semconv.ServerPort(27017)}},
		{"no connection number", "mongo:27017", []attribute.KeyValue{semconv.ServerAddress("mongo"), semconv.ServerPort(27017)}},
		{"no port", "mongo[-1]", []attribute.KeyValue{semconv.ServerAddress("mongo")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, server(tt.id))
		})
	}
}
