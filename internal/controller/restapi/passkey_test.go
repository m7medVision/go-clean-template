package restapi_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakePasskey struct {
	usecase.Passkey

	options  []byte
	token    string
	stored   entity.Passkey
	passkeys []entity.Passkey
	err      error

	// gotToken, gotName and gotBody record what FinishRegistration was called with; gotUser and
	// gotID what Rename and Delete were called with (Rename's name goes in gotName).
	gotToken, gotName string
	gotBody           []byte
	gotUser, gotID    string
	// gotAuthTime is the auth time BeginRegistration was called with.
	gotAuthTime time.Time
}

func (f *fakePasskey) BeginRegistration(_ context.Context, _ string, authTime time.Time) (options []byte, token string, err error) {
	f.gotAuthTime = authTime

	return f.options, f.token, f.err
}

func (f *fakePasskey) FinishRegistration(_ context.Context, _, token, name string, body []byte) (entity.Passkey, error) {
	f.gotToken, f.gotName, f.gotBody = token, name, body

	return f.stored, f.err
}

func (f *fakePasskey) List(context.Context, string) ([]entity.Passkey, error) {
	return f.passkeys, f.err
}

func (f *fakePasskey) Rename(_ context.Context, userID, id, name string) (entity.Passkey, error) {
	f.gotUser, f.gotID, f.gotName = userID, id, name

	return f.stored, f.err
}

func (f *fakePasskey) Delete(_ context.Context, userID, id string) error {
	f.gotUser, f.gotID = userID, id

	return f.err
}

func (f *fakePasskey) BeginLogin(context.Context) (options []byte, token string, err error) {
	return f.options, f.token, f.err
}

func (f *fakePasskey) FinishLogin(_ context.Context, token string, body []byte) (string, error) {
	f.gotToken, f.gotBody = token, body

	return f.token, f.err
}

// _ceremonyHeader carries the ceremony token on both finish endpoints.
const _ceremonyHeader = "X-Ceremony-Token"

// doCeremonyRequest is doRequest with the ceremony token in its header; an empty one is left out.
func doCeremonyRequest(
	ctx context.Context,
	handler http.Handler,
	target, token, ceremony, body string,
) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, target, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	if ceremony != "" {
		req.Header.Set(_ceremonyHeader, ceremony)
	}

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	return resp
}

func newPasskeyRouter(t *testing.T, pk usecase.Passkey) http.Handler {
	t.Helper()

	return newRouterWithPasskey(newTestConfig(t), &fakeExchange{}, &fakeUser{}, &fakeTask{}, pk)
}

func TestPasskeyRoutesNeedAToken(t *testing.T) {
	t.Parallel()

	handler := newPasskeyRouter(t, &fakePasskey{})

	for _, tt := range []struct{ method, target string }{
		{http.MethodPost, "/v1/passkeys/register/begin"},
		{http.MethodPost, "/v1/passkeys/register/finish"},
		{http.MethodGet, "/v1/passkeys"},
		{http.MethodPatch, "/v1/passkeys/AQIDBA"},
		{http.MethodDelete, "/v1/passkeys/AQIDBA"},
	} {
		resp := doRequest(t.Context(), handler, tt.method, tt.target, "", "{}")
		assert.Equal(t, http.StatusUnauthorized, resp.Code, tt.target)
	}
}

func TestBeginPasskeyRegistration(t *testing.T) {
	t.Parallel()

	t.Run("returns the options as an object and the ceremony token", func(t *testing.T) {
		t.Parallel()

		handler := newPasskeyRouter(t, &fakePasskey{options: []byte(`{"challenge":"abc","rp":{"id":"localhost"}}`), token: "tok"})

		resp := doRequest(t.Context(), handler, http.MethodPost, "/v1/passkeys/register/begin", newToken(t), "")
		require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
		assert.JSONEq(t, `{"options":{"challenge":"abc","rp":{"id":"localhost"}},"ceremony_token":"tok"}`,
			resp.Body.String())
	})

	t.Run("passes when the caller's token was issued", func(t *testing.T) {
		t.Parallel()

		pk := &fakePasskey{}
		handler := newPasskeyRouter(t, pk)
		before := time.Now().Truncate(time.Second)

		resp := doRequest(t.Context(), handler, http.MethodPost, "/v1/passkeys/register/begin", newToken(t), "")
		require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
		assert.False(t, pk.gotAuthTime.Before(before), "auth time %s, before %s", pk.gotAuthTime, before)
		assert.False(t, pk.gotAuthTime.After(time.Now()), "auth time in the future: %s", pk.gotAuthTime)
	})

	t.Run("a stale sign-in is a 401 asking to reauthenticate", func(t *testing.T) {
		t.Parallel()

		handler := newPasskeyRouter(t, &fakePasskey{err: entity.ErrPasskeyReauthRequired})

		resp := doRequest(t.Context(), handler, http.MethodPost, "/v1/passkeys/register/begin", newToken(t), "")
		require.Equal(t, http.StatusUnauthorized, resp.Code)

		var problem struct {
			Status int    `json:"status"`
			Detail string `json:"detail"`
		}

		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &problem))
		assert.Equal(t, http.StatusUnauthorized, problem.Status)
		assert.Equal(t, "reauthentication required", problem.Detail)
	})

	for _, tt := range []struct {
		name string
		err  error
		code int
	}{
		{"unknown user maps to 404", entity.ErrUserNotFound, http.StatusNotFound},
		{"other failure maps to 500", io.EOF, http.StatusInternalServerError},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := newPasskeyRouter(t, &fakePasskey{err: tt.err})

			resp := doRequest(t.Context(), handler, http.MethodPost, "/v1/passkeys/register/begin", newToken(t), "")
			assert.Equal(t, tt.code, resp.Code)
		})
	}
}

func TestFinishPasskeyRegistration(t *testing.T) {
	t.Parallel()

	const attestation = `{"id":"AQIDBA","rawId":"AQIDBA","type":"public-key","response":{"clientDataJSON":"e30"}}`

	t.Run("passes the token, name and raw credential and answers 201", func(t *testing.T) {
		t.Parallel()

		pk := &fakePasskey{stored: entity.Passkey{
			ID: entity.PasskeyID{1, 2, 3, 4}, UserID: "user-1", Name: "Laptop",
			Credential: []byte(`{"secret":true}`), CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		}}
		handler := newPasskeyRouter(t, pk)

		resp := doCeremonyRequest(t.Context(), handler,
			"/v1/passkeys/register/finish?name=Laptop", newToken(t), "tok", attestation)
		require.Equal(t, http.StatusCreated, resp.Code, resp.Body.String())
		assert.JSONEq(t,
			`{"id":"AQIDBA","name":"Laptop","created_at":"2026-01-01T00:00:00Z","last_used_at":null}`,
			resp.Body.String())
		assert.Equal(t, "tok", pk.gotToken)
		assert.Equal(t, "Laptop", pk.gotName)
		assert.JSONEq(t, attestation, string(pk.gotBody))
	})

	for _, tt := range []struct {
		name string
		err  error
		code int
	}{
		{"invalid ceremony maps to 400", entity.ErrPasskeyCeremonyInvalid, http.StatusBadRequest},
		{"duplicate passkey maps to 409", entity.ErrPasskeyExists, http.StatusConflict},
		{"other failure maps to 500", io.EOF, http.StatusInternalServerError},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := newPasskeyRouter(t, &fakePasskey{err: tt.err})

			resp := doCeremonyRequest(t.Context(), handler, "/v1/passkeys/register/finish", newToken(t), "t", attestation)
			assert.Equal(t, tt.code, resp.Code)
		})
	}
}

func TestFinishPasskeyRegistration_Validation(t *testing.T) {
	t.Parallel()

	const attestation = `{"id":"AQIDBA","rawId":"AQIDBA","type":"public-key","response":{"clientDataJSON":"e30"}}`

	for _, tt := range []struct {
		name, target, ceremony, body string
		code                         int
	}{
		{"missing ceremony token is rejected", "/v1/passkeys/register/finish", "", attestation, http.StatusUnprocessableEntity},
		{
			"a ceremony token in the query string is not read", "/v1/passkeys/register/finish?ceremony_token=t", "",
			attestation, http.StatusUnprocessableEntity,
		},
		{
			"a body that is not an object is rejected", "/v1/passkeys/register/finish", "t", `[]`,
			http.StatusUnprocessableEntity,
		},
		{"invalid json is rejected", "/v1/passkeys/register/finish", "t", `{bad`, http.StatusBadRequest},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := newPasskeyRouter(t, &fakePasskey{})

			resp := doCeremonyRequest(t.Context(), handler, tt.target, newToken(t), tt.ceremony, tt.body)
			assert.Equal(t, tt.code, resp.Code, resp.Body.String())
		})
	}
}

func TestListPasskeys(t *testing.T) {
	t.Parallel()

	t.Run("no passkeys is an empty list", func(t *testing.T) {
		t.Parallel()

		handler := newPasskeyRouter(t, &fakePasskey{})

		resp := doRequest(t.Context(), handler, http.MethodGet, "/v1/passkeys", newToken(t), "")
		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{"passkeys":[]}`, resp.Body.String())
	})

	t.Run("failure maps to 500", func(t *testing.T) {
		t.Parallel()

		handler := newPasskeyRouter(t, &fakePasskey{err: io.EOF})

		resp := doRequest(t.Context(), handler, http.MethodGet, "/v1/passkeys", newToken(t), "")
		assert.Equal(t, http.StatusInternalServerError, resp.Code)
	})
}

func TestRenamePasskey(t *testing.T) {
	t.Parallel()

	t.Run("passes the caller, id and name and returns the passkey", func(t *testing.T) {
		t.Parallel()

		pk := &fakePasskey{stored: entity.Passkey{
			ID: entity.PasskeyID{1, 2, 3, 4}, UserID: "user-1", Name: "Phone",
			CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		}}
		handler := newPasskeyRouter(t, pk)

		resp := doRequest(t.Context(), handler, http.MethodPatch, "/v1/passkeys/AQIDBA", newToken(t), `{"name":"Phone"}`)
		require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
		assert.JSONEq(t,
			`{"id":"AQIDBA","name":"Phone","created_at":"2026-01-01T00:00:00Z","last_used_at":null}`,
			resp.Body.String())
		assert.Equal(t, "user-1", pk.gotUser)
		assert.Equal(t, "AQIDBA", pk.gotID)
		assert.Equal(t, "Phone", pk.gotName)
	})

	for _, tt := range []struct {
		name, body string
		code       int
	}{
		{"an empty name is rejected", `{"name":""}`, http.StatusUnprocessableEntity},
		{"a missing name is rejected", `{}`, http.StatusUnprocessableEntity},
		{"a name over 64 characters is rejected", `{"name":"` + strings.Repeat("a", 65) + `"}`, http.StatusUnprocessableEntity},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pk := &fakePasskey{}
			handler := newPasskeyRouter(t, pk)

			resp := doRequest(t.Context(), handler, http.MethodPatch, "/v1/passkeys/AQIDBA", newToken(t), tt.body)
			assert.Equal(t, tt.code, resp.Code, resp.Body.String())
			assert.Empty(t, pk.gotID, "the use case must not be called")
		})
	}

	for _, tt := range []struct {
		name string
		err  error
		code int
	}{
		{"unknown or another user's passkey maps to 404", entity.ErrPasskeyNotFound, http.StatusNotFound},
		{"other failure maps to 500", io.EOF, http.StatusInternalServerError},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := newPasskeyRouter(t, &fakePasskey{err: tt.err})

			resp := doRequest(t.Context(), handler, http.MethodPatch, "/v1/passkeys/AQIDBA", newToken(t), `{"name":"Phone"}`)
			assert.Equal(t, tt.code, resp.Code)
		})
	}
}

func TestDeletePasskey(t *testing.T) {
	t.Parallel()

	t.Run("passes the caller and id and answers 204", func(t *testing.T) {
		t.Parallel()

		pk := &fakePasskey{}
		handler := newPasskeyRouter(t, pk)

		resp := doRequest(t.Context(), handler, http.MethodDelete, "/v1/passkeys/AQIDBA", newToken(t), "")
		require.Equal(t, http.StatusNoContent, resp.Code, resp.Body.String())
		assert.Empty(t, resp.Body.String())
		assert.Equal(t, "user-1", pk.gotUser)
		assert.Equal(t, "AQIDBA", pk.gotID)
	})

	for _, tt := range []struct {
		name string
		err  error
		code int
	}{
		{"unknown or another user's passkey maps to 404", entity.ErrPasskeyNotFound, http.StatusNotFound},
		{"other failure maps to 500", io.EOF, http.StatusInternalServerError},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := newPasskeyRouter(t, &fakePasskey{err: tt.err})

			resp := doRequest(t.Context(), handler, http.MethodDelete, "/v1/passkeys/AQIDBA", newToken(t), "")
			assert.Equal(t, tt.code, resp.Code)
		})
	}
}

func TestBeginPasskeyLogin(t *testing.T) {
	t.Parallel()

	t.Run("is public and returns the options and the ceremony token", func(t *testing.T) {
		t.Parallel()

		handler := newPasskeyRouter(t, &fakePasskey{options: []byte(`{"challenge":"abc","rpId":"localhost"}`), token: "tok"})

		resp := doRequest(t.Context(), handler, http.MethodPost, "/v1/auth/passkey/begin", "", "")
		require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
		assert.JSONEq(t, `{"options":{"challenge":"abc","rpId":"localhost"},"ceremony_token":"tok"}`,
			resp.Body.String())
	})

	t.Run("failure maps to 500", func(t *testing.T) {
		t.Parallel()

		handler := newPasskeyRouter(t, &fakePasskey{err: io.EOF})

		resp := doRequest(t.Context(), handler, http.MethodPost, "/v1/auth/passkey/begin", "", "")
		assert.Equal(t, http.StatusInternalServerError, resp.Code)
	})
}

func TestFinishPasskeyLogin(t *testing.T) {
	t.Parallel()

	const assertion = `{"id":"AQIDBA","rawId":"AQIDBA","type":"public-key","response":{"clientDataJSON":"e30"}}`

	t.Run("is public, passes the token and raw assertion and returns the JWT", func(t *testing.T) {
		t.Parallel()

		pk := &fakePasskey{token: "jwt"}
		handler := newPasskeyRouter(t, pk)

		resp := doCeremonyRequest(t.Context(), handler, "/v1/auth/passkey/finish", "", "tok", assertion)
		require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
		assert.JSONEq(t, `{"token":"jwt"}`, resp.Body.String())
		assert.Equal(t, "tok", pk.gotToken)
		assert.JSONEq(t, assertion, string(pk.gotBody))
	})

	for _, tt := range []struct{ name, target string }{
		{"missing ceremony token is rejected", "/v1/auth/passkey/finish"},
		{"a ceremony token in the query string is not read", "/v1/auth/passkey/finish?ceremony_token=t"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pk := &fakePasskey{}
			handler := newPasskeyRouter(t, pk)

			resp := doCeremonyRequest(t.Context(), handler, tt.target, "", "", assertion)
			assert.Equal(t, http.StatusUnprocessableEntity, resp.Code, resp.Body.String())
			assert.Nil(t, pk.gotBody, "the use case must not be called")
		})
	}

	for _, tt := range []struct {
		name string
		err  error
		code int
	}{
		{"invalid ceremony maps to 401", entity.ErrPasskeyCeremonyInvalid, http.StatusUnauthorized},
		{"replayed login maps to 409", entity.ErrPasskeyChallengeUsed, http.StatusConflict},
		{"other failure maps to 500", io.EOF, http.StatusInternalServerError},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := newPasskeyRouter(t, &fakePasskey{err: tt.err})

			resp := doCeremonyRequest(t.Context(), handler, "/v1/auth/passkey/finish", "", "t", assertion)
			assert.Equal(t, tt.code, resp.Code)
		})
	}
}
