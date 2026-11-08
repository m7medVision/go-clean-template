package valkey

import (
	"context"
	"fmt"
	"time"

	vk "github.com/valkey-io/valkey-go"
)

type Cache struct {
	client vk.Client
}

func NewCache(client vk.Client) *Cache {
	return &Cache{client: client}
}

func (c *Cache) Get(ctx context.Context, key string) (value []byte, found bool, err error) {
	value, err = c.client.Do(ctx, c.client.B().Get().Key(key).Build()).AsBytes()
	if vk.IsValkeyNil(err) {
		return nil, false, nil
	}

	if err != nil {
		return nil, false, fmt.Errorf("valkey - Cache.Get: %w", err)
	}

	return value, true, nil
}

func (c *Cache) Set(ctx context.Context, key string, value []byte, expiresAt time.Time) error {
	cmd := c.client.B().Set().Key(key).Value(vk.BinaryString(value)).Pxat(expiresAt).Build()

	if err := c.client.Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("valkey - Cache.Set: %w", err)
	}

	return nil
}
