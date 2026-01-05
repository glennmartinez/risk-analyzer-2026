package db

import (
	"context"
	"testing"
	"time"
)

// TestNewRedisClient tests client initialization
func TestNewRedisClient(t *testing.T) {
	tests := []struct {
		name       string
		config     RedisConfig
		wantError  bool
		checkField func(*RedisClient) error
	}{
		{
			name: "default config",
			config: RedisConfig{
				Host: "localhost",
				Port: 6379,
			},
			wantError: false,
		},
		{
			name: "custom config with all fields",
			config: RedisConfig{
				Host:         "redis.example.com",
				Port:         6380,
				Password:     "secret",
				DB:           1,
				PoolSize:     20,
				MinIdleConns: 10,
				MaxRetries:   5,
				DialTimeout:  10 * time.Second,
				ReadTimeout:  5 * time.Second,
				WriteTimeout: 5 * time.Second,
			},
			wantError: false,
		},
		{
			name:      "empty config uses defaults",
			config:    RedisConfig{},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewRedisClient(tt.config)

			if (err != nil) != tt.wantError {
				t.Errorf("NewRedisClient() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if client == nil {
				t.Fatal("Expected non-nil client")
			}

			if client.client == nil {
				t.Error("Expected non-nil underlying Redis client")
			}

			// Verify defaults are applied
			if client.config.PoolSize == 0 {
				t.Error("Expected PoolSize to be set")
			}
			if client.config.MinIdleConns == 0 {
				t.Error("Expected MinIdleConns to be set")
			}
		})
	}
}

// TestDefaultRedisConfig tests default configuration
func TestDefaultRedisConfig(t *testing.T) {
	config := DefaultRedisConfig()

	if config.Host != "localhost" {
		t.Errorf("Expected default host 'localhost', got %s", config.Host)
	}
	if config.Port != 6379 {
		t.Errorf("Expected default port 6379, got %d", config.Port)
	}
	if config.PoolSize != 10 {
		t.Errorf("Expected default pool size 10, got %d", config.PoolSize)
	}
	if config.MinIdleConns != 5 {
		t.Errorf("Expected default min idle conns 5, got %d", config.MinIdleConns)
	}
	if config.MaxRetries != 3 {
		t.Errorf("Expected default max retries 3, got %d", config.MaxRetries)
	}

	t.Log("✅ Default config has correct values")
}

// TestRedisClient_Ping tests ping functionality
func TestRedisClient_Ping(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	client, err := NewRedisClient(DefaultRedisConfig())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.Ping(ctx)
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	t.Log("✅ Ping successful")
}

// TestRedisClient_SetGet tests basic set/get operations
func TestRedisClient_SetGet(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	client, err := NewRedisClient(DefaultRedisConfig())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testKey := "test:setget:key"
	testValue := "test-value-123"

	// Set
	err = client.Set(ctx, testKey, testValue, 10*time.Second)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	t.Log("✅ Set successful")

	// Get
	val, err := client.Get(ctx, testKey)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if val != testValue {
		t.Errorf("Expected value %s, got %s", testValue, val)
	}
	t.Logf("✅ Get successful: %s", val)

	// Cleanup
	client.Del(ctx, testKey)
}

// TestRedisClient_Del tests delete operation
func TestRedisClient_Del(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	client, err := NewRedisClient(DefaultRedisConfig())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testKey := "test:del:key"

	// Set a key
	client.Set(ctx, testKey, "value", 10*time.Second)

	// Delete it
	err = client.Del(ctx, testKey)
	if err != nil {
		t.Fatalf("Del failed: %v", err)
	}
	t.Log("✅ Delete successful")

	// Verify it's gone
	_, err = client.Get(ctx, testKey)
	if err == nil {
		t.Error("Expected error when getting deleted key")
	}
	t.Log("✅ Verified key was deleted")
}

// TestRedisClient_Exists tests exists check
func TestRedisClient_Exists(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	client, err := NewRedisClient(DefaultRedisConfig())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testKey := "test:exists:key"

	// Should not exist initially
	count, err := client.Exists(ctx, testKey)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected count 0, got %d", count)
	}

	// Set the key
	client.Set(ctx, testKey, "value", 10*time.Second)
	defer client.Del(ctx, testKey)

	// Should exist now
	count, err = client.Exists(ctx, testKey)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected count 1, got %d", count)
	}

	t.Log("✅ Exists check works correctly")
}

// TestRedisClient_HashOperations tests hash operations (critical for document registry)
func TestRedisClient_HashOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	client, err := NewRedisClient(DefaultRedisConfig())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testKey := "test:hash:doc123"

	// HSet - set multiple fields
	err = client.HSet(ctx, testKey,
		"document_id", "doc123",
		"filename", "test.pdf",
		"chunk_count", "42",
		"collection", "test_collection",
	)
	if err != nil {
		t.Fatalf("HSet failed: %v", err)
	}
	t.Log("✅ HSet successful")

	// HGet - get single field
	filename, err := client.HGet(ctx, testKey, "filename")
	if err != nil {
		t.Fatalf("HGet failed: %v", err)
	}
	if filename != "test.pdf" {
		t.Errorf("Expected filename 'test.pdf', got %s", filename)
	}
	t.Logf("✅ HGet successful: %s", filename)

	// HGetAll - get all fields
	all, err := client.HGetAll(ctx, testKey)
	if err != nil {
		t.Fatalf("HGetAll failed: %v", err)
	}
	if len(all) != 4 {
		t.Errorf("Expected 4 fields, got %d", len(all))
	}
	if all["document_id"] != "doc123" {
		t.Errorf("Expected document_id 'doc123', got %s", all["document_id"])
	}
	t.Logf("✅ HGetAll successful: %d fields", len(all))

	// HExists - check field exists
	exists, err := client.HExists(ctx, testKey, "filename")
	if err != nil {
		t.Fatalf("HExists failed: %v", err)
	}
	if !exists {
		t.Error("Expected field 'filename' to exist")
	}
	t.Log("✅ HExists successful")

	// HDel - delete field
	err = client.HDel(ctx, testKey, "chunk_count")
	if err != nil {
		t.Fatalf("HDel failed: %v", err)
	}

	// Verify field was deleted
	exists, _ = client.HExists(ctx, testKey, "chunk_count")
	if exists {
		t.Error("Expected field 'chunk_count' to be deleted")
	}
	t.Log("✅ HDel successful")

	// Cleanup
	client.Del(ctx, testKey)
}

// TestRedisClient_SetOperations tests set operations (critical for document ID tracking)
func TestRedisClient_SetOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	client, err := NewRedisClient(DefaultRedisConfig())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testKey := "test:set:docs:all"

	// SAdd - add members
	err = client.SAdd(ctx, testKey, "doc1", "doc2", "doc3")
	if err != nil {
		t.Fatalf("SAdd failed: %v", err)
	}
	t.Log("✅ SAdd successful")

	// SMembers - get all members
	members, err := client.SMembers(ctx, testKey)
	if err != nil {
		t.Fatalf("SMembers failed: %v", err)
	}
	if len(members) != 3 {
		t.Errorf("Expected 3 members, got %d", len(members))
	}
	t.Logf("✅ SMembers successful: %d members", len(members))

	// SIsMember - check membership
	isMember, err := client.SIsMember(ctx, testKey, "doc2")
	if err != nil {
		t.Fatalf("SIsMember failed: %v", err)
	}
	if !isMember {
		t.Error("Expected 'doc2' to be a member")
	}
	t.Log("✅ SIsMember successful")

	// SCard - get count
	count, err := client.SCard(ctx, testKey)
	if err != nil {
		t.Fatalf("SCard failed: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected count 3, got %d", count)
	}
	t.Logf("✅ SCard successful: %d", count)

	// SRem - remove member
	err = client.SRem(ctx, testKey, "doc2")
	if err != nil {
		t.Fatalf("SRem failed: %v", err)
	}

	// Verify removal
	count, _ = client.SCard(ctx, testKey)
	if count != 2 {
		t.Errorf("Expected count 2 after removal, got %d", count)
	}
	t.Log("✅ SRem successful")

	// Cleanup
	client.Del(ctx, testKey)
}

// TestRedisClient_TTL tests TTL and expiration
func TestRedisClient_TTL(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	client, err := NewRedisClient(DefaultRedisConfig())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testKey := "test:ttl:key"

	// Set with expiration
	err = client.Set(ctx, testKey, "value", 5*time.Second)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Check TTL
	ttl, err := client.TTL(ctx, testKey)
	if err != nil {
		t.Fatalf("TTL failed: %v", err)
	}
	if ttl <= 0 || ttl > 5*time.Second {
		t.Errorf("Expected TTL around 5 seconds, got %v", ttl)
	}
	t.Logf("✅ TTL check successful: %v", ttl)

	// Cleanup
	client.Del(ctx, testKey)
}

// TestRedisClient_Expire tests setting expiration
func TestRedisClient_Expire(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	client, err := NewRedisClient(DefaultRedisConfig())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testKey := "test:expire:key"

	// Set without expiration
	client.Set(ctx, testKey, "value", 0)
	defer client.Del(ctx, testKey)

	// Set expiration
	err = client.Expire(ctx, testKey, 10*time.Second)
	if err != nil {
		t.Fatalf("Expire failed: %v", err)
	}

	// Verify expiration was set
	ttl, err := client.TTL(ctx, testKey)
	if err != nil {
		t.Fatalf("TTL failed: %v", err)
	}
	if ttl <= 0 {
		t.Error("Expected positive TTL")
	}
	t.Logf("✅ Expire successful, TTL: %v", ttl)
}

// TestRedisClient_Keys tests pattern matching
func TestRedisClient_Keys(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	client, err := NewRedisClient(DefaultRedisConfig())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Set test keys
	testKeys := []string{
		"test:keys:doc1",
		"test:keys:doc2",
		"test:keys:doc3",
		"test:other:key",
	}

	for _, key := range testKeys {
		client.Set(ctx, key, "value", 10*time.Second)
		defer client.Del(ctx, key)
	}

	// Get keys matching pattern
	keys, err := client.Keys(ctx, "test:keys:*")
	if err != nil {
		t.Fatalf("Keys failed: %v", err)
	}

	if len(keys) != 3 {
		t.Errorf("Expected 3 matching keys, got %d", len(keys))
	}
	t.Logf("✅ Keys pattern matching successful: %d keys", len(keys))
}

// TestRedisClient_PoolStats tests connection pool statistics
func TestRedisClient_PoolStats(t *testing.T) {
	client, err := NewRedisClient(DefaultRedisConfig())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	stats := client.PoolStats()
	if stats == nil {
		t.Fatal("Expected non-nil pool stats")
	}

	t.Logf("✅ Pool stats: TotalConns=%d, IdleConns=%d, StaleConns=%d",
		stats.TotalConns, stats.IdleConns, stats.StaleConns)
}

// TestRedisClient_Pipeline tests pipelined operations
func TestRedisClient_Pipeline(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	client, err := NewRedisClient(DefaultRedisConfig())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pipe := client.Pipeline()

	// Queue multiple operations
	testKeys := []string{"test:pipe:1", "test:pipe:2", "test:pipe:3"}
	for _, key := range testKeys {
		pipe.Set(ctx, key, "value", 10*time.Second)
	}

	// Execute pipeline
	_, err = pipe.Exec(ctx)
	if err != nil {
		t.Fatalf("Pipeline execution failed: %v", err)
	}
	t.Log("✅ Pipeline execution successful")

	// Verify keys were set
	for _, key := range testKeys {
		exists, _ := client.Exists(ctx, key)
		if exists != 1 {
			t.Errorf("Expected key %s to exist", key)
		}
		client.Del(ctx, key)
	}
	t.Log("✅ Verified all pipeline operations completed")
}

// TestRedisClient_Close tests client cleanup
func TestRedisClient_Close(t *testing.T) {
	client, err := NewRedisClient(DefaultRedisConfig())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	err = client.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
	t.Log("✅ Client closed successfully")
}

// TestRedisClient_ContextCancellation tests context cancellation handling
func TestRedisClient_ContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	client, err := NewRedisClient(DefaultRedisConfig())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err = client.Set(ctx, "test:key", "value", 0)
	if err == nil {
		t.Error("Expected error with cancelled context")
	}
	t.Logf("✅ Correctly handled cancelled context: %v", err)
}
