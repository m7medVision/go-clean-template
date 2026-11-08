package httpmw_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/controller/httpmw"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestLoggerLogsTheClient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		resolved bool
		want     string
	}{
		{"the client resolved behind the proxy", true, _forwarded},
		{"the remote address when none was resolved", false, _proxy},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var logs bytes.Buffer

			h := httpmw.Logger(slog.New(slog.NewJSONHandler(&logs, nil)))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}))

			if tt.resolved {
				h = httpmw.ClientAddress(netip.MustParseAddr(_proxy))(h)
			}

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/tasks?limit=5", http.NoBody)
			req.RemoteAddr = _proxy + ":41000"
			req.Header.Set("X-Forwarded-For", _forwarded)

			h.ServeHTTP(httptest.NewRecorder(), req)

			r := onlyRecord(t, &logs)
			assert.Equal(t, tt.want, r["client"])
			assert.Equal(t, http.MethodGet, r["method"])
			assert.Equal(t, "/v1/tasks?limit=5", r["uri"])
			assert.EqualValues(t, http.StatusNoContent, r["status"])
		})
	}
}
