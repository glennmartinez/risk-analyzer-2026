package db

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisClient wraps the Redis client with connection pooling
type RedisClient struct {
	client *redis.Client
	config RedisConfig
}

// RedisConfig holds configuration for Redis connection
type RedisConfig struct {
	Host         string
	Port         int
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	MaxRetries   int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// DefaultRedisConfig returns a Redis configuration with sensible defaults
func DefaultRedisConfig() RedisConfig {
	return RedisConfig{
		Host:         "localhost",
		Port:         6379,
		Password:     "",
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 5,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}
}

// NewRedisClient creates a new Redis client with connection pooling
func NewRedisClient(config RedisConfig) (*RedisClient, error) {
	// Apply defaults if not set
	if config.PoolSize == 0 {
		config.PoolSize = 10
	}
	if config.MinIdleConns == 0 {
		config.MinIdleConns = 5
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.DialTimeout == 0 {
		config.DialTimeout = 5 * time.Second
	}
	if config.ReadTimeout == 0 {
		config.ReadTimeout = 3 * time.Second
	}
	if config.WriteTimeout == 0 {
		config.WriteTimeout = 3 * time.Second
	}

	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)

	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     config.Password,
		DB:           config.DB,
		PoolSize:     config.PoolSize,
		MinIdleConns: config.MinIdleConns,
		MaxRetries:   config.MaxRetries,
		DialTimeout:  config.DialTimeout,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
	})

	return &RedisClient{
		client: client,
		config: config,
	}, nil
}

// Ping checks if Redis is alive
func (r *RedisClient) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Get retrieves a value by key
func (r *RedisClient) Get(ctx context.Context, key string) (string, error) {
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("key not found: %s", key)
	}
	return val, err
}

// Set sets a key-value pair with optional expiration
func (r *RedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return r.client.Set(ctx, key, value, expiration).Err()
}

// Del deletes one or more keys
func (r *RedisClient) Del(ctx context.Context, keys ...string) error {
	return r.client.Del(ctx, keys...).Err()
}

// Exists checks if keys exist
func (r *RedisClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	return r.client.Exists(ctx, keys...).Result()
}

// HSet sets field in hash stored at key
func (r *RedisClient) HSet(ctx context.Context, key string, values ...interface{}) error {
	return r.client.HSet(ctx, key, values...).Err()
}

// HGet gets field value from hash
func (r *RedisClient) HGet(ctx context.Context, key, field string) (string, error) {
	val, err := r.client.HGet(ctx, key, field).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("field not found: %s", field)
	}
	return val, err
}

// HGetAll gets all fields and values from hash
func (r *RedisClient) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return r.client.HGetAll(ctx, key).Result()
}

// HDel deletes fields from hash
func (r *RedisClient) HDel(ctx context.Context, key string, fields ...string) error {
	return r.client.HDel(ctx, key, fields...).Err()
}

// HExists checks if field exists in hash
func (r *RedisClient) HExists(ctx context.Context, key, field string) (bool, error) {
	return r.client.HExists(ctx, key, field).Result()
}

// SAdd adds members to set
func (r *RedisClient) SAdd(ctx context.Context, key string, members ...interface{}) error {
	return r.client.SAdd(ctx, key, members...).Err()
}

// SMembers gets all members of set
func (r *RedisClient) SMembers(ctx context.Context, key string) ([]string, error) {
	return r.client.SMembers(ctx, key).Result()
}

// SRem removes members from set
func (r *RedisClient) SRem(ctx context.Context, key string, members ...interface{}) error {
	return r.client.SRem(ctx, key, members...).Err()
}

// SIsMember checks if value is member of set
func (r *RedisClient) SIsMember(ctx context.Context, key string, member interface{}) (bool, error) {
	return r.client.SIsMember(ctx, key, member).Result()
}

// SCard gets the number of members in set
func (r *RedisClient) SCard(ctx context.Context, key string) (int64, error) {
	return r.client.SCard(ctx, key).Result()
}

// Pipeline returns a new pipeline
func (r *RedisClient) Pipeline() redis.Pipeliner {
	return r.client.Pipeline()
}

// TxPipeline returns a new transaction pipeline
func (r *RedisClient) TxPipeline() redis.Pipeliner {
	return r.client.TxPipeline()
}

// Watch watches keys for changes (optimistic locking)
func (r *RedisClient) Watch(ctx context.Context, fn func(*redis.Tx) error, keys ...string) error {
	return r.client.Watch(ctx, fn, keys...)
}

// Keys returns all keys matching pattern
func (r *RedisClient) Keys(ctx context.Context, pattern string) ([]string, error) {
	return r.client.Keys(ctx, pattern).Result()
}

// TTL returns the remaining time to live of a key
func (r *RedisClient) TTL(ctx context.Context, key string) (time.Duration, error) {
	return r.client.TTL(ctx, key).Result()
}

// Expire sets a timeout on key
func (r *RedisClient) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return r.client.Expire(ctx, key, expiration).Err()
}

// PoolStats returns connection pool statistics
func (r *RedisClient) PoolStats() *redis.PoolStats {
	return r.client.PoolStats()
}

// Close closes the Redis client and releases all connections
func (r *RedisClient) Close() error {
	return r.client.Close()
}

// GetClient returns the underlying redis client for advanced operations
func (r *RedisClient) GetClient() *redis.Client {
	return r.client
}
