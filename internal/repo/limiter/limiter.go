package limiter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/netip"
	"strings"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	vk "github.com/valkey-io/valkey-go"
	"github.com/valkey-io/valkey-go/valkeylimiter"
)

const (
	_clientKeyPrefix  = "limit:client"
	_accountKeyPrefix = "limit:account"
	_ipv6ClientBits   = 64
)

type counters struct {
	clients  valkeylimiter.RateLimiterClient
	accounts valkeylimiter.RateLimiterClient
}

func New(client vk.Client, limits entity.Limits) (repo.Limiter, error) {
	build := func(vk.ClientOption) (vk.Client, error) { return client, nil }

	clients, err := valkeylimiter.NewRateLimiter(valkeylimiter.RateLimiterOption{
		ClientBuilder: build,
		KeyPrefix:     _clientKeyPrefix,
		Limit:         limits.ClientAttempts,
		Window:        limits.Window,
	})
	if err != nil {
		return nil, fmt.Errorf("limiter - New - valkeylimiter.NewRateLimiter clients: %w", err)
	}

	accounts, err := valkeylimiter.NewRateLimiter(valkeylimiter.RateLimiterOption{
		ClientBuilder: build,
		KeyPrefix:     _accountKeyPrefix,
		Limit:         limits.AccountFailures,
		Window:        limits.Window,
	})
	if err != nil {
		return nil, fmt.Errorf("limiter - New - valkeylimiter.NewRateLimiter accounts: %w", err)
	}

	return &counters{clients: clients, accounts: accounts}, nil
}

func (c *counters) TakeClient(ctx context.Context, scope string, client netip.Addr) (entity.Allowance, error) {
	res, err := c.clients.Allow(ctx, clientID(scope, client))
	if err != nil {
		return entity.Allowance{}, fmt.Errorf("limiter - TakeClient - Allow: %w", err)
	}

	return allowance(res), nil
}

func (c *counters) CheckAccount(ctx context.Context, scope, account string) (entity.Allowance, error) {
	res, err := c.accounts.Check(ctx, accountID(scope, account))
	if err != nil {
		return entity.Allowance{}, fmt.Errorf("limiter - CheckAccount - Check: %w", err)
	}

	return allowance(res), nil
}

func (c *counters) TakeAccountFailure(ctx context.Context, scope, account string) error {
	if _, err := c.accounts.Allow(ctx, accountID(scope, account)); err != nil {
		return fmt.Errorf("limiter - TakeAccountFailure - Allow: %w", err)
	}

	return nil
}

func (c *counters) ClearAccount(ctx context.Context, scope, account string) error {
	if err := c.accounts.Reset(ctx, accountID(scope, account)); err != nil {
		return fmt.Errorf("limiter - ClearAccount - Reset: %w", err)
	}

	return nil
}

func allowance(res valkeylimiter.Result) entity.Allowance {
	return entity.Allowance{Allowed: res.Allowed, RetryAfter: max(res.RetryAfter, 0)}
}

func clientID(scope string, addr netip.Addr) string {
	addr = addr.Unmap()
	if addr.Is6() {
		return scope + ":" + netip.PrefixFrom(addr, _ipv6ClientBits).Masked().String()
	}

	return scope + ":" + addr.String()
}

func accountID(scope, account string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(account))))

	return scope + ":" + hex.EncodeToString(sum[:])
}
