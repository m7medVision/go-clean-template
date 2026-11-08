package valkey

import (
	"context"
	"fmt"

	vk "github.com/valkey-io/valkey-go"
	"github.com/valkey-io/valkey-go/valkeyotel"
)

func New(ctx context.Context, url string) (vk.Client, error) {
	opt, err := vk.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("valkey - New - vk.ParseURL: %w", err)
	}

	client, err := valkeyotel.NewClient(opt)
	if err != nil {
		return nil, fmt.Errorf("valkey - New - valkeyotel.NewClient: %w", err)
	}

	if err = client.Do(ctx, client.B().Ping().Build()).Error(); err != nil {
		client.Close()

		return nil, fmt.Errorf("valkey - New - ping: %w", err)
	}

	return client, nil
}
