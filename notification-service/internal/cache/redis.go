package cache

import (
	"context"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cache struct {
	client *redis.Client
}

func NewCache() *Cache {
	return NewCacheWithAddr(os.Getenv("REDIS_ADDR"))
}

func NewCacheWithAddr(addr string) *Cache {
	client := redis.NewClient(&redis.Options{Addr: addr})
	return &Cache{client: client}
}

func (c *Cache) Get(key string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return c.client.Get(ctx, key).Result()
}

func (c *Cache) Set(key, value string, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return c.client.Set(ctx, key, value, ttl).Err()
}
