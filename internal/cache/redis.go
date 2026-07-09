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
	return c.client.Get(context.Background(), key).Result()
}

func (c *Cache) Set(key, value string, ttl time.Duration) error {
	return c.client.Set(context.Background(), key, value, ttl).Err()
}
