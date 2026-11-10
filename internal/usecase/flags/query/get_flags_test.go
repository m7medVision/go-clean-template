package query_test

import (
	"context"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/flags/query"
	"github.com/m7medvision/go-clean-template/pkg/flags/flagstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		values map[string]bool
		want   entity.FlagValues
	}{
		{"defaults", nil, entity.FlagValues{TaskAttributes: entity.FlagTaskAttributes().Default}},
		{"off", map[string]bool{entity.FlagTaskAttributes().Key: false}, entity.FlagValues{}},
		{"on", map[string]bool{entity.FlagTaskAttributes().Key: true}, entity.FlagValues{TaskAttributes: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := query.GetFlagsHandler{Flags: flagstest.New(t, tt.values)}.
				Handle(context.Background(), query.GetFlags{UserID: "u1"})

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
