package scaffold

import "testing"

func TestMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		pattern, name string
		want          bool
	}{
		{"web/**", "web/src/main.ts", true},
		{"web/**", "web", true},
		{"web/**", "webx/a", false},
		{"internal/**/tracing.go", "internal/usecase/task/tracing.go", true},
		{"integration-test/grpc_*", "integration-test/grpc_task_test.go", true},
		{"integration-test/grpc_*", "integration-test/task_test.go", false},
		{"internal/repo/mailtmpl/templates/welcome.*", "internal/repo/mailtmpl/templates/welcome.html.tmpl", true},
		{"config/grpc.go", "config/grpc.go", true},
		{"config/grpc.go", "config/grpc.go.bak", false},
		{"docs/**", "docs/proto/v1/task.proto", true},
	}

	for _, tt := range tests {
		if got := match(tt.pattern, tt.name); got != tt.want {
			t.Errorf("match(%q, %q) = %v, want %v", tt.pattern, tt.name, got, tt.want)
		}
	}
}
