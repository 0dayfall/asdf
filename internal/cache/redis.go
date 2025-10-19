package cache

import (
	"asdf/internal/types"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cache interface {
	GetWebFingerRecord(ctx context.Context, subject string) (*types.JRD, error)
	SetWebFingerRecord(ctx context.Context, subject string, jrd *types.JRD, expiry time.Duration) error
	DeleteWebFingerRecord(ctx context.Context, subject string) error
	GetSearchResults(ctx context.Context, query string) ([]string, error)
	SetSearchResults(ctx context.Context, query string, results []string, expiry time.Duration) error
	InvalidateUserCache(ctx context.Context, subject string) error
	GetStats(ctx context.Context) (*CacheStats, error)
	Close() error
}

type RedisCache struct {
	client *redis.Client
	prefix string
}

type CacheStats struct {
	HitCount    int64 `json:"hit_count"`
	MissCount   int64 `json:"miss_count"`
	KeyCount    int64 `json:"key_count"`
	MemoryUsage int64 `json:"memory_usage"`
}

// NewRedisCache creates a new Redis cache instance
func NewRedisCache(redisURL, password string, db int) (*RedisCache, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	if password != "" {
		opt.Password = password
	}
	if db != 0 {
		opt.DB = db
	}

	client := redis.NewClient(opt)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisCache{
		client: client,
		prefix: "asdf:webfinger:",
	}, nil
}

// GetWebFingerRecord retrieves a WebFinger record from cache
func (c *RedisCache) GetWebFingerRecord(ctx context.Context, subject string) (*types.JRD, error) {
	key := c.webfingerKey(subject)

	data, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss
		}
		return nil, fmt.Errorf("failed to get WebFinger record from cache: %w", err)
	}

	var jrd types.JRD
	if err := json.Unmarshal([]byte(data), &jrd); err != nil {
		// If we can't unmarshal, delete the corrupted cache entry
		c.client.Del(ctx, key)
		return nil, fmt.Errorf("failed to unmarshal cached WebFinger record: %w", err)
	}

	// Increment hit counter
	c.client.Incr(ctx, c.prefix+"stats:hits")

	return &jrd, nil
}

// SetWebFingerRecord stores a WebFinger record in cache
func (c *RedisCache) SetWebFingerRecord(ctx context.Context, subject string, jrd *types.JRD, expiry time.Duration) error {
	key := c.webfingerKey(subject)

	data, err := json.Marshal(jrd)
	if err != nil {
		return fmt.Errorf("failed to marshal WebFinger record: %w", err)
	}

	if err := c.client.Set(ctx, key, data, expiry).Err(); err != nil {
		return fmt.Errorf("failed to set WebFinger record in cache: %w", err)
	}

	return nil
}

// DeleteWebFingerRecord removes a WebFinger record from cache
func (c *RedisCache) DeleteWebFingerRecord(ctx context.Context, subject string) error {
	key := c.webfingerKey(subject)

	if err := c.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete WebFinger record from cache: %w", err)
	}

	return nil
}

// GetSearchResults retrieves search results from cache
func (c *RedisCache) GetSearchResults(ctx context.Context, query string) ([]string, error) {
	key := c.searchKey(query)

	results, err := c.client.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss
		}
		return nil, fmt.Errorf("failed to get search results from cache: %w", err)
	}

	if len(results) == 0 {
		return nil, nil // Empty cache entry
	}

	// Increment hit counter
	c.client.Incr(ctx, c.prefix+"stats:hits")

	return results, nil
}

// SetSearchResults stores search results in cache
func (c *RedisCache) SetSearchResults(ctx context.Context, query string, results []string, expiry time.Duration) error {
	key := c.searchKey(query)

	// Use pipeline for atomic operation
	pipe := c.client.Pipeline()
	pipe.Del(ctx, key) // Clear existing list

	if len(results) > 0 {
		// Convert to interface{} slice for Redis
		args := make([]interface{}, len(results))
		for i, result := range results {
			args[i] = result
		}
		pipe.LPush(ctx, key, args...)
	} else {
		// Store empty marker to avoid repeated database queries
		pipe.LPush(ctx, key, "__EMPTY__")
	}

	pipe.Expire(ctx, key, expiry)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to set search results in cache: %w", err)
	}

	return nil
}

// InvalidateUserCache removes all cache entries for a user
func (c *RedisCache) InvalidateUserCache(ctx context.Context, subject string) error {
	// Delete WebFinger record
	if err := c.DeleteWebFingerRecord(ctx, subject); err != nil {
		return err
	}

	// Delete search results that might contain this user
	// Note: This is a simple approach. In production, you might want
	// to use Redis SCAN to find and delete matching search cache keys
	searchPattern := c.prefix + "search:*"
	keys, err := c.client.Keys(ctx, searchPattern).Result()
	if err != nil {
		return fmt.Errorf("failed to find search cache keys: %w", err)
	}

	if len(keys) > 0 {
		if err := c.client.Del(ctx, keys...).Err(); err != nil {
			return fmt.Errorf("failed to delete search cache entries: %w", err)
		}
	}

	return nil
}

// GetStats returns cache statistics
func (c *RedisCache) GetStats(ctx context.Context) (*CacheStats, error) {
	pipe := c.client.Pipeline()

	hitCmd := pipe.Get(ctx, c.prefix+"stats:hits")
	missCmd := pipe.Get(ctx, c.prefix+"stats:misses")

	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to get cache stats: %w", err)
	}

	stats := &CacheStats{}

	if hitCmd.Err() == nil {
		hits, _ := hitCmd.Int64()
		stats.HitCount = hits
	}

	if missCmd.Err() == nil {
		misses, _ := missCmd.Int64()
		stats.MissCount = misses
	}

	// Get key count for WebFinger records
	webfingerPattern := c.prefix + "webfinger:*"
	keys, err := c.client.Keys(ctx, webfingerPattern).Result()
	if err == nil {
		stats.KeyCount = int64(len(keys))
	}

	// Get memory usage (Redis INFO command)
	_, err = c.client.Info(ctx, "memory").Result()
	if err == nil {
		// Parse memory usage from INFO output (simplified)
		// In production, you'd want more robust parsing
		stats.MemoryUsage = 0 // Placeholder
	}

	return stats, nil
}

// Close closes the Redis connection
func (c *RedisCache) Close() error {
	return c.client.Close()
}

// RecordMiss increments the cache miss counter
func (c *RedisCache) RecordMiss(ctx context.Context) {
	c.client.Incr(ctx, c.prefix+"stats:misses")
}

// webfingerKey generates a cache key for WebFinger records
func (c *RedisCache) webfingerKey(subject string) string {
	return c.prefix + "webfinger:" + subject
}

// searchKey generates a cache key for search results
func (c *RedisCache) searchKey(query string) string {
	return c.prefix + "search:" + query
}

// Cleanup removes expired and unused cache entries
func (c *RedisCache) Cleanup(ctx context.Context) error {
	// Redis handles TTL automatically, but we can do additional cleanup

	// Remove empty search result markers older than 1 hour
	searchPattern := c.prefix + "search:*"
	keys, err := c.client.Keys(ctx, searchPattern).Result()
	if err != nil {
		return fmt.Errorf("failed to scan search keys: %w", err)
	}

	for _, key := range keys {
		// Check if the list contains only the empty marker
		results, err := c.client.LRange(ctx, key, 0, -1).Result()
		if err != nil {
			continue
		}

		if len(results) == 1 && results[0] == "__EMPTY__" {
			// Check TTL and delete if older than 1 hour
			ttl, err := c.client.TTL(ctx, key).Result()
			if err == nil && ttl > 0 && ttl < 23*time.Hour {
				c.client.Del(ctx, key)
			}
		}
	}

	return nil
}
