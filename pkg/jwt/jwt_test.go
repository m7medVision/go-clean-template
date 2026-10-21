package jwt_test

import (
	"testing"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWT_GenerateAndParse(t *testing.T) {
	t.Parallel()

	j := jwt.New("test-secret", time.Hour)

	token, err := j.GenerateToken("user-123", nil)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	userID, err := j.ParseToken(token)
	require.NoError(t, err)
	assert.Equal(t, "user-123", userID)
}

func TestJWT_ParseToken_Invalid(t *testing.T) {
	t.Parallel()

	j := jwt.New("test-secret", time.Hour)

	_, err := j.ParseToken("invalid-token")
	require.Error(t, err)
}

func TestJWT_ParseToken_WrongSecret(t *testing.T) {
	t.Parallel()

	j1 := jwt.New("secret-1", time.Hour)
	j2 := jwt.New("secret-2", time.Hour)

	token, err := j1.GenerateToken("user-123", nil)
	require.NoError(t, err)

	_, err = j2.ParseToken(token)
	require.Error(t, err)
}

func TestJWT_ParseToken_Expired(t *testing.T) {
	t.Parallel()

	j := jwt.New("test-secret", -time.Hour)

	token, err := j.GenerateToken("user-123", nil)
	require.NoError(t, err)

	_, err = j.ParseToken(token)
	require.Error(t, err)
}

func TestJWT_Parse_IssuedAt(t *testing.T) {
	t.Parallel()

	j := jwt.New("test-secret", time.Hour)

	before := time.Now().Truncate(time.Second)

	token, err := j.GenerateToken("user-123", nil)
	require.NoError(t, err)

	claims, err := j.Parse(token)
	require.NoError(t, err)
	assert.Equal(t, "user-123", claims.UserID)
	assert.False(t, claims.IssuedAt.Before(before), "issued at %s, before %s", claims.IssuedAt, before)
	assert.False(t, claims.IssuedAt.After(time.Now()), "issued in the future: %s", claims.IssuedAt)
}

func TestJWT_Parse_NoIssuedAt(t *testing.T) {
	t.Parallel()

	// A token minted before tokens carried iat parses, with a zero issue time.
	token, err := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, jwtlib.RegisteredClaims{
		Subject:   "user-123",
		ExpiresAt: jwtlib.NewNumericDate(time.Now().Add(time.Hour)),
	}).SignedString([]byte("test-secret"))
	require.NoError(t, err)

	claims, err := jwt.New("test-secret", time.Hour).Parse(token)
	require.NoError(t, err)
	assert.Equal(t, "user-123", claims.UserID)
	assert.True(t, claims.IssuedAt.IsZero())
	assert.Empty(t, claims.Roles)
}

func TestJWT_Parse_Roles(t *testing.T) {
	t.Parallel()

	j := jwt.New("test-secret", time.Hour)

	token, err := j.GenerateToken("user-123", []string{"admin", "user"})
	require.NoError(t, err)

	claims, err := j.Parse(token)
	require.NoError(t, err)
	assert.Equal(t, []string{"admin", "user"}, claims.Roles)
}

func TestJWT_Parse_Invalid(t *testing.T) {
	t.Parallel()

	_, err := jwt.New("test-secret", time.Hour).Parse("invalid-token")
	require.Error(t, err)
}
