package omniq

import (
	"context"
	"time"

	"github.com/not-empty/omniq-go"
	"github.com/redis/go-redis/v9"
)

type Client struct {
	rdb         *redis.Client
	OmniqClient *omniq.Client
}

func NewClient(redisURL string) (*Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}

	rdb := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	omniqClient, err := omniq.NewClient(omniq.ClientOpts{
		RedisURL: redisURL,
	})
	if err != nil {
		rdb.Close()
		return nil, err
	}

	return &Client{rdb: rdb, OmniqClient: omniqClient}, nil
}

func (c *Client) Close() error {
	return c.rdb.Close()
}
