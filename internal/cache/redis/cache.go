package rediscache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache implements flag.Cache using Redis.
type Cache struct {
	client *redis.Client
}

func New(client *redis.Client) *Cache {
	return &Cache{client: client}
}

func (c *Cache) Get(ctx context.Context, key string) (bool, bool, error) {
	val, err := c.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	return val == "1", true, nil
}

func (c *Cache) Set(ctx context.Context, key string, enabled bool, ttl time.Duration) error {
	val := "0"
	if enabled {
		val = "1"
	}
	return c.client.Set(ctx, key, val, ttl).Err()
}

func (c *Cache) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return c.client.Del(ctx, keys...).Err()
}
