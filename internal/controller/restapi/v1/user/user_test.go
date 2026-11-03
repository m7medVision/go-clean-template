package user_test

import (
	"net/http"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/controller/restapi/resttest"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/stretchr/testify/assert"
)

func TestRegisterAndLogin(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		cfg := resttest.Config(t)
		handler := newRouter(cfg, &fakeUser{
			registered: entity.User{ID: "user-1", Username: "johndoe"},
			token:      "token-1",
		})

		resp := resttest.Do(t.Context(), handler, http.MethodPost, "/v1/auth/register", "",
			`{"username":"johndoe","email":"john@example.com","password":"secret123"}`)
		assert.Equal(t, http.StatusCreated, resp.Code)

		resp = resttest.Do(t.Context(), handler, http.MethodPost, "/v1/auth/login", "",
			`{"email":"john@example.com","password":"secret123"}`)
		assert.Equal(t, http.StatusOK, resp.Code)
	})

	t.Run("conflict and bad credentials", func(t *testing.T) {
		t.Parallel()

		cfg := resttest.Config(t)
		handler := newRouter(cfg, &fakeUser{
			registerErr: entity.ErrUserAlreadyExists,
			loginErr:    entity.ErrInvalidCredentials,
		})

		resp := resttest.Do(t.Context(), handler, http.MethodPost, "/v1/auth/register", "",
			`{"username":"johndoe","email":"john@example.com","password":"secret123"}`)
		assert.Equal(t, http.StatusConflict, resp.Code)

		resp = resttest.Do(t.Context(), handler, http.MethodPost, "/v1/auth/login", "",
			`{"email":"john@example.com","password":"secret123"}`)
		assert.Equal(t, http.StatusUnauthorized, resp.Code)
	})
}
