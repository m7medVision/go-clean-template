package limiter_test

import (
	"crypto/sha256"
	"encoding/hex"
	"net/netip"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/limiter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valkey-io/valkey-go/valkeylimiter"
)

func TestClientID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		scope string
		addr  string
		want  string
	}{
		{name: "IPv4", scope: "sign-in", addr: "192.0.2.1", want: "sign-in:192.0.2.1"},
		{name: "IPv4-mapped IPv6", scope: "sign-in", addr: "::ffff:192.0.2.1", want: "sign-in:192.0.2.1"},
		{name: "IPv6", scope: "sign-in", addr: "2001:db8:1:2:aaaa:bbbb:cccc:dddd", want: "sign-in:2001:db8:1:2::/64"},
		{name: "IPv6 with a zone", scope: "sign-in", addr: "fe80::1%eth0", want: "sign-in:fe80::/64"},
		{name: "another scope", scope: "sign-up", addr: "192.0.2.1", want: "sign-up:192.0.2.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, limiter.ClientID(tt.scope, netip.MustParseAddr(tt.addr)))
		})
	}
}

func TestClientIDKeysAnIPv6ClientByItsSlash64(t *testing.T) {
	t.Parallel()

	id := func(addr string) string { return limiter.ClientID("sign-in", netip.MustParseAddr(addr)) }

	assert.Equal(t, id("2001:db8:1:2::1"), id("2001:db8:1:2:ffff:ffff:ffff:ffff"))
	assert.NotEqual(t, id("2001:db8:1:2::1"), id("2001:db8:1:3::1"))
}

func TestAccountID(t *testing.T) {
	t.Parallel()

	sum := sha256.Sum256([]byte("ann@example.com"))
	want := "sign-in:" + hex.EncodeToString(sum[:])

	tests := []struct {
		name    string
		account string
	}{
		{name: "as written", account: "ann@example.com"},
		{name: "upper case", account: "ANN@Example.COM"},
		{name: "surrounding space", account: " \tann@example.com\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := limiter.AccountID("sign-in", tt.account)

			assert.Equal(t, want, got)
			assert.NotContains(t, got, "@")
		})
	}
}

func TestAccountIDSeparatesAccountsAndScopes(t *testing.T) {
	t.Parallel()

	ann := limiter.AccountID("sign-in", "ann@example.com")

	assert.NotEqual(t, ann, limiter.AccountID("sign-in", "bob@example.com"))
	assert.NotEqual(t, ann, limiter.AccountID("sign-up", "ann@example.com"))
}

func TestNew(t *testing.T) {
	t.Parallel()

	const (
		attempts = 20
		failures = 5
		window   = 15 * time.Minute
	)

	tests := []struct {
		name    string
		limits  entity.Limits
		wantErr error
	}{
		{name: "valid", limits: entity.Limits{ClientAttempts: attempts, AccountFailures: failures, Window: window}},
		{
			name:    "no client attempts",
			limits:  entity.Limits{AccountFailures: failures, Window: window},
			wantErr: valkeylimiter.ErrInvalidLimit,
		},
		{
			name:    "no account failures",
			limits:  entity.Limits{ClientAttempts: attempts, Window: window},
			wantErr: valkeylimiter.ErrInvalidLimit,
		},
		{
			name:    "negative account failures",
			limits:  entity.Limits{ClientAttempts: attempts, AccountFailures: -1, Window: window},
			wantErr: valkeylimiter.ErrInvalidLimit,
		},
		{
			name:    "no window",
			limits:  entity.Limits{ClientAttempts: attempts, AccountFailures: failures},
			wantErr: valkeylimiter.ErrInvalidWindow,
		},
		{
			name:    "negative window",
			limits:  entity.Limits{ClientAttempts: attempts, AccountFailures: failures, Window: -time.Second},
			wantErr: valkeylimiter.ErrInvalidWindow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := limiter.New(nil, tt.limits)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, got)

				return
			}

			require.NoError(t, err)
			assert.NotNil(t, got)
		})
	}
}
