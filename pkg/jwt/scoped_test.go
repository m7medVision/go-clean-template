package jwt_test

import (
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScoped_SignAndVerify(t *testing.T) {
	t.Parallel()

	s, err := jwt.NewScoped("test-secret", "webauthn-ceremony", time.Minute)
	require.NoError(t, err)

	token, err := s.Sign([]byte(`{"challenge":"abc"}`))
	require.NoError(t, err)

	payload, err := s.Verify(token)
	require.NoError(t, err)
	assert.JSONEq(t, `{"challenge":"abc"}`, string(payload))
}

func TestScoped_Verify_Expired(t *testing.T) {
	t.Parallel()

	s, err := jwt.NewScoped("test-secret", "webauthn-ceremony", -time.Minute)
	require.NoError(t, err)

	token, err := s.Sign([]byte("payload"))
	require.NoError(t, err)

	_, err = s.Verify(token)
	require.Error(t, err)
}

func TestScoped_Verify_Tampered(t *testing.T) {
	t.Parallel()

	s, err := jwt.NewScoped("test-secret", "webauthn-ceremony", time.Minute)
	require.NoError(t, err)

	token, err := s.Sign([]byte("payload"))
	require.NoError(t, err)

	_, err = s.Verify(token[:len(token)-2] + "xx")
	require.Error(t, err)
}

func TestScoped_Verify_RejectsOtherPurpose(t *testing.T) {
	t.Parallel()

	a, err := jwt.NewScoped("test-secret", "purpose-a", time.Minute)
	require.NoError(t, err)

	b, err := jwt.NewScoped("test-secret", "purpose-b", time.Minute)
	require.NoError(t, err)

	token, err := a.Sign([]byte("payload"))
	require.NoError(t, err)

	_, err = b.Verify(token)
	require.Error(t, err)
}

func TestScoped_Verify_RejectsAuthToken(t *testing.T) {
	t.Parallel()

	auth := jwt.New("test-secret", time.Hour)

	s, err := jwt.NewScoped("test-secret", "webauthn-ceremony", time.Minute)
	require.NoError(t, err)

	token, err := auth.GenerateToken("user-123")
	require.NoError(t, err)

	_, err = s.Verify(token)
	require.Error(t, err)
}

func TestManager_ParseToken_RejectsScopedToken(t *testing.T) {
	t.Parallel()

	auth := jwt.New("test-secret", time.Hour)

	s, err := jwt.NewScoped("test-secret", "webauthn-ceremony", time.Minute)
	require.NoError(t, err)

	token, err := s.Sign([]byte("payload"))
	require.NoError(t, err)

	_, err = auth.ParseToken(token)
	require.Error(t, err)
}
