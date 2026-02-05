package services

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache provides caching functionality using Redis
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache creates a new Redis cache client
func NewRedisCache(redisURL string) (*RedisCache, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opt)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	log.Println("Redis connection established")
	return &RedisCache{client: client}, nil
}

// Set stores a value in cache with expiration
func (c *RedisCache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, data, expiration).Err()
}

// Get retrieves a value from cache
func (c *RedisCache) Get(ctx context.Context, key string, dest interface{}) error {
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

// GetOrSet retrieves a value from cache, or calls the callback to fetch and cache it
// The callback is only called if the key doesn't exist in cache
func GetOrSet[T any](c *RedisCache, ctx context.Context, key string, expiration time.Duration, fn func() (T, error)) (T, error) {
	var result T

	// Try to get from cache
	err := c.Get(ctx, key, &result)
	if err == nil {
		return result, nil
	}

	// Cache miss or error - call the callback
	result, err = fn()
	if err != nil {
		return result, err
	}

	// Store in cache (ignore cache set errors)
	_ = c.Set(ctx, key, result, expiration)

	return result, nil
}

// Delete removes a key from cache
func (c *RedisCache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

// Exists checks if a key exists in cache
func (c *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.client.Exists(ctx, key).Result()
	return n > 0, err
}

// SetNX sets a value only if key doesn't exist (useful for locks)
func (c *RedisCache) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return false, err
	}
	return c.client.SetNX(ctx, key, data, expiration).Result()
}

// Increment increments a counter
func (c *RedisCache) Increment(ctx context.Context, key string) (int64, error) {
	return c.client.Incr(ctx, key).Result()
}

// Close closes the Redis connection
func (c *RedisCache) Close() error {
	return c.client.Close()
}

// Client returns the underlying Redis client for advanced operations
func (c *RedisCache) Client() *redis.Client {
	return c.client
}
