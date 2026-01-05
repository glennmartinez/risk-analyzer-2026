package repositories

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestRedis creates a test Redis client
func setupTestRedis(t *testing.T) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // Use separate DB for testing
	})

	// Ping to ensure connection
	ctx := context.Background()
	err := client.Ping(ctx).Err()
	require.NoError(t, err, "Redis must be running for tests")

	// Flush test database
	err = client.FlushDB(ctx).Err()
	require.NoError(t, err)

	return client
}

func TestNewRedisDocumentRepository(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	repo := NewRedisDocumentRepository(client)
	assert.NotNil(t, repo)
	assert.Equal(t, client, repo.client)
}

func TestRedisDocumentRepository_Register(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisDocumentRepository(client)
	ctx := context.Background()

	t.Run("successful registration", func(t *testing.T) {
		doc := &Document{
			ID:               "doc-1",
			Filename:         "test.pdf",
			Collection:       "test-collection",
			ChunkCount:       10,
			FileSize:         1024,
			Status:           DocumentStatusPending,
			StoredInVectorDB: false,
			Metadata:         map[string]interface{}{"key": "value"},
		}

		err := repo.Register(ctx, doc)
		require.NoError(t, err)

		// Verify document was stored
		retrieved, err := repo.Get(ctx, "doc-1")
		require.NoError(t, err)
		assert.Equal(t, doc.ID, retrieved.ID)
		assert.Equal(t, doc.Filename, retrieved.Filename)
		assert.Equal(t, doc.Collection, retrieved.Collection)
		assert.Equal(t, doc.ChunkCount, retrieved.ChunkCount)
		assert.Equal(t, doc.Status, retrieved.Status)
		assert.NotZero(t, retrieved.CreatedAt)
		assert.NotZero(t, retrieved.UpdatedAt)
	})

	t.Run("duplicate registration fails", func(t *testing.T) {
		doc := &Document{
			ID:         "doc-duplicate",
			Filename:   "dup.pdf",
			Collection: "test-collection",
			Status:     DocumentStatusPending,
		}

		err := repo.Register(ctx, doc)
		require.NoError(t, err)

		// Try to register again
		err = repo.Register(ctx, doc)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("invalid document fails validation", func(t *testing.T) {
		doc := &Document{
			ID:         "", // Invalid: empty ID
			Filename:   "test.pdf",
			Collection: "test-collection",
		}

		err := repo.Register(ctx, doc)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "required")
	})
}

func TestRedisDocumentRepository_Get(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisDocumentRepository(client)
	ctx := context.Background()

	t.Run("get existing document", func(t *testing.T) {
		doc := &Document{
			ID:         "doc-get-1",
			Filename:   "get.pdf",
			Collection: "test-collection",
			ChunkCount: 5,
			Status:     DocumentStatusCompleted,
		}

		err := repo.Register(ctx, doc)
		require.NoError(t, err)

		retrieved, err := repo.Get(ctx, "doc-get-1")
		require.NoError(t, err)
		assert.Equal(t, doc.ID, retrieved.ID)
		assert.Equal(t, doc.Filename, retrieved.Filename)
		assert.Equal(t, doc.ChunkCount, retrieved.ChunkCount)
	})

	t.Run("get non-existent document", func(t *testing.T) {
		_, err := repo.Get(ctx, "non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestRedisDocumentRepository_List(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisDocumentRepository(client)
	ctx := context.Background()

	t.Run("list all documents", func(t *testing.T) {
		// Register multiple documents
		docs := []*Document{
			{ID: "doc-list-1", Filename: "list1.pdf", Collection: "coll1", Status: DocumentStatusPending},
			{ID: "doc-list-2", Filename: "list2.pdf", Collection: "coll2", Status: DocumentStatusCompleted},
			{ID: "doc-list-3", Filename: "list3.pdf", Collection: "coll1", Status: DocumentStatusFailed},
		}

		for _, doc := range docs {
			err := repo.Register(ctx, doc)
			require.NoError(t, err)
		}

		// List all
		allDocs, err := repo.List(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(allDocs), 3)

		// Verify our docs are in the list
		foundIDs := make(map[string]bool)
		for _, doc := range allDocs {
			foundIDs[doc.ID] = true
		}
		assert.True(t, foundIDs["doc-list-1"])
		assert.True(t, foundIDs["doc-list-2"])
		assert.True(t, foundIDs["doc-list-3"])
	})

	t.Run("list returns empty on no documents", func(t *testing.T) {
		client2 := setupTestRedis(t)
		defer client2.Close()
		repo2 := NewRedisDocumentRepository(client2)

		docs, err := repo2.List(ctx)
		require.NoError(t, err)
		assert.Empty(t, docs)
	})
}

func TestRedisDocumentRepository_Delete(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisDocumentRepository(client)
	ctx := context.Background()

	t.Run("delete existing document", func(t *testing.T) {
		doc := &Document{
			ID:         "doc-delete-1",
			Filename:   "delete.pdf",
			Collection: "test-collection",
			Status:     DocumentStatusCompleted,
		}

		err := repo.Register(ctx, doc)
		require.NoError(t, err)

		// Delete
		err = repo.Delete(ctx, "doc-delete-1")
		require.NoError(t, err)

		// Verify it's gone
		_, err = repo.Get(ctx, "doc-delete-1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")

		// Verify it's removed from collection index
		collDocs, err := repo.ListByCollection(ctx, "test-collection")
		require.NoError(t, err)
		for _, d := range collDocs {
			assert.NotEqual(t, "doc-delete-1", d.ID)
		}
	})

	t.Run("delete non-existent document", func(t *testing.T) {
		err := repo.Delete(ctx, "non-existent")
		assert.Error(t, err)
	})
}

func TestRedisDocumentRepository_Update(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisDocumentRepository(client)
	ctx := context.Background()

	t.Run("update document fields", func(t *testing.T) {
		doc := &Document{
			ID:         "doc-update-1",
			Filename:   "update.pdf",
			Collection: "coll1",
			ChunkCount: 5,
			Status:     DocumentStatusPending,
		}

		err := repo.Register(ctx, doc)
		require.NoError(t, err)

		// Update fields
		updates := map[string]interface{}{
			"chunk_count": 10,
			"status":      DocumentStatusCompleted,
		}

		err = repo.Update(ctx, "doc-update-1", updates)
		require.NoError(t, err)

		// Verify updates
		updated, err := repo.Get(ctx, "doc-update-1")
		require.NoError(t, err)
		assert.Equal(t, 10, updated.ChunkCount)
		assert.Equal(t, DocumentStatusCompleted, updated.Status)
	})

	t.Run("update changes indexes", func(t *testing.T) {
		doc := &Document{
			ID:         "doc-update-2",
			Filename:   "update2.pdf",
			Collection: "old-coll",
			Status:     DocumentStatusPending,
		}

		err := repo.Register(ctx, doc)
		require.NoError(t, err)

		// Update collection and status
		updates := map[string]interface{}{
			"collection": "new-coll",
			"status":     DocumentStatusCompleted,
		}

		err = repo.Update(ctx, "doc-update-2", updates)
		require.NoError(t, err)

		// Verify old collection index updated
		oldDocs, err := repo.ListByCollection(ctx, "old-coll")
		require.NoError(t, err)
		for _, d := range oldDocs {
			assert.NotEqual(t, "doc-update-2", d.ID)
		}

		// Verify new collection index updated
		newDocs, err := repo.ListByCollection(ctx, "new-coll")
		require.NoError(t, err)
		found := false
		for _, d := range newDocs {
			if d.ID == "doc-update-2" {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestRedisDocumentRepository_Exists(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisDocumentRepository(client)
	ctx := context.Background()

	doc := &Document{
		ID:         "doc-exists-1",
		Filename:   "exists.pdf",
		Collection: "test-collection",
		Status:     DocumentStatusPending,
	}

	// Check before registration
	exists, err := repo.Exists(ctx, "doc-exists-1")
	require.NoError(t, err)
	assert.False(t, exists)

	// Register
	err = repo.Register(ctx, doc)
	require.NoError(t, err)

	// Check after registration
	exists, err = repo.Exists(ctx, "doc-exists-1")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestRedisDocumentRepository_RegisterBatch(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisDocumentRepository(client)
	ctx := context.Background()

	docs := []*Document{
		{ID: "batch-1", Filename: "batch1.pdf", Collection: "batch-coll", Status: DocumentStatusPending},
		{ID: "batch-2", Filename: "batch2.pdf", Collection: "batch-coll", Status: DocumentStatusPending},
		{ID: "batch-3", Filename: "batch3.pdf", Collection: "batch-coll", Status: DocumentStatusPending},
	}

	err := repo.RegisterBatch(ctx, docs)
	require.NoError(t, err)

	// Verify all registered
	for _, doc := range docs {
		retrieved, err := repo.Get(ctx, doc.ID)
		require.NoError(t, err)
		assert.Equal(t, doc.ID, retrieved.ID)
		assert.Equal(t, doc.Filename, retrieved.Filename)
	}
}

func TestRedisDocumentRepository_GetBatch(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisDocumentRepository(client)
	ctx := context.Background()

	// Register test documents
	docs := []*Document{
		{ID: "get-batch-1", Filename: "gb1.pdf", Collection: "coll", Status: DocumentStatusPending},
		{ID: "get-batch-2", Filename: "gb2.pdf", Collection: "coll", Status: DocumentStatusPending},
		{ID: "get-batch-3", Filename: "gb3.pdf", Collection: "coll", Status: DocumentStatusPending},
	}

	for _, doc := range docs {
		err := repo.Register(ctx, doc)
		require.NoError(t, err)
	}

	// Get batch
	retrieved, err := repo.GetBatch(ctx, []string{"get-batch-1", "get-batch-2", "get-batch-3"})
	require.NoError(t, err)
	assert.Len(t, retrieved, 3)

	// Get batch with some missing
	retrieved, err = repo.GetBatch(ctx, []string{"get-batch-1", "non-existent", "get-batch-2"})
	require.NoError(t, err)
	assert.Len(t, retrieved, 2) // Only existing ones
}

func TestRedisDocumentRepository_DeleteBatch(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisDocumentRepository(client)
	ctx := context.Background()

	// Register test documents
	docs := []*Document{
		{ID: "del-batch-1", Filename: "db1.pdf", Collection: "coll", Status: DocumentStatusPending},
		{ID: "del-batch-2", Filename: "db2.pdf", Collection: "coll", Status: DocumentStatusPending},
		{ID: "del-batch-3", Filename: "db3.pdf", Collection: "coll", Status: DocumentStatusPending},
	}

	for _, doc := range docs {
		err := repo.Register(ctx, doc)
		require.NoError(t, err)
	}

	// Delete batch
	err := repo.DeleteBatch(ctx, []string{"del-batch-1", "del-batch-2"})
	require.NoError(t, err)

	// Verify deletion
	_, err = repo.Get(ctx, "del-batch-1")
	assert.Error(t, err)

	_, err = repo.Get(ctx, "del-batch-2")
	assert.Error(t, err)

	// Verify del-batch-3 still exists
	_, err = repo.Get(ctx, "del-batch-3")
	assert.NoError(t, err)
}

func TestRedisDocumentRepository_ListByCollection(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisDocumentRepository(client)
	ctx := context.Background()

	// Register documents in different collections
	docs := []*Document{
		{ID: "coll-a-1", Filename: "a1.pdf", Collection: "collection-a", Status: DocumentStatusPending},
		{ID: "coll-a-2", Filename: "a2.pdf", Collection: "collection-a", Status: DocumentStatusPending},
		{ID: "coll-b-1", Filename: "b1.pdf", Collection: "collection-b", Status: DocumentStatusPending},
	}

	for _, doc := range docs {
		err := repo.Register(ctx, doc)
		require.NoError(t, err)
	}

	// List collection-a
	collADocs, err := repo.ListByCollection(ctx, "collection-a")
	require.NoError(t, err)
	assert.Len(t, collADocs, 2)

	// List collection-b
	collBDocs, err := repo.ListByCollection(ctx, "collection-b")
	require.NoError(t, err)
	assert.Len(t, collBDocs, 1)

	// List non-existent collection
	noneDocs, err := repo.ListByCollection(ctx, "non-existent")
	require.NoError(t, err)
	assert.Empty(t, noneDocs)
}

func TestRedisDocumentRepository_ListByStatus(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisDocumentRepository(client)
	ctx := context.Background()

	// Register documents with different statuses
	docs := []*Document{
		{ID: "status-1", Filename: "s1.pdf", Collection: "coll", Status: DocumentStatusPending},
		{ID: "status-2", Filename: "s2.pdf", Collection: "coll", Status: DocumentStatusPending},
		{ID: "status-3", Filename: "s3.pdf", Collection: "coll", Status: DocumentStatusCompleted},
	}

	for _, doc := range docs {
		err := repo.Register(ctx, doc)
		require.NoError(t, err)
	}

	// List pending
	pending, err := repo.ListByStatus(ctx, DocumentStatusPending)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(pending), 2)

	// List completed
	completed, err := repo.ListByStatus(ctx, DocumentStatusCompleted)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(completed), 1)
}

func TestRedisDocumentRepository_CountByCollection(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisDocumentRepository(client)
	ctx := context.Background()

	docs := []*Document{
		{ID: "count-1", Filename: "c1.pdf", Collection: "count-coll", Status: DocumentStatusPending},
		{ID: "count-2", Filename: "c2.pdf", Collection: "count-coll", Status: DocumentStatusPending},
		{ID: "count-3", Filename: "c3.pdf", Collection: "other-coll", Status: DocumentStatusPending},
	}

	for _, doc := range docs {
		err := repo.Register(ctx, doc)
		require.NoError(t, err)
	}

	count, err := repo.CountByCollection(ctx, "count-coll")
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	count, err = repo.CountByCollection(ctx, "other-coll")
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestRedisDocumentRepository_CountTotal(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisDocumentRepository(client)
	ctx := context.Background()

	initialCount, err := repo.CountTotal(ctx)
	require.NoError(t, err)

	// Register documents
	docs := []*Document{
		{ID: "total-1", Filename: "t1.pdf", Collection: "coll", Status: DocumentStatusPending},
		{ID: "total-2", Filename: "t2.pdf", Collection: "coll", Status: DocumentStatusPending},
	}

	for _, doc := range docs {
		err := repo.Register(ctx, doc)
		require.NoError(t, err)
	}

	count, err := repo.CountTotal(ctx)
	require.NoError(t, err)
	assert.Equal(t, initialCount+2, count)
}

func TestRedisDocumentRepository_FindByFilename(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisDocumentRepository(client)
	ctx := context.Background()

	doc := &Document{
		ID:         "filename-1",
		Filename:   "unique-file.pdf",
		Collection: "coll",
		Status:     DocumentStatusPending,
	}

	err := repo.Register(ctx, doc)
	require.NoError(t, err)

	// Find by filename
	found, err := repo.FindByFilename(ctx, "unique-file.pdf")
	require.NoError(t, err)
	assert.Equal(t, "filename-1", found.ID)
	assert.Equal(t, "unique-file.pdf", found.Filename)

	// Try non-existent filename
	_, err = repo.FindByFilename(ctx, "non-existent.pdf")
	assert.Error(t, err)
}

func TestRedisDocumentRepository_FilterByMetadata(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisDocumentRepository(client)
	ctx := context.Background()

	docs := []*Document{
		{
			ID:         "meta-1",
			Filename:   "m1.pdf",
			Collection: "coll",
			Status:     DocumentStatusPending,
			Metadata:   map[string]interface{}{"type": "invoice", "year": 2024},
		},
		{
			ID:         "meta-2",
			Filename:   "m2.pdf",
			Collection: "coll",
			Status:     DocumentStatusPending,
			Metadata:   map[string]interface{}{"type": "report", "year": 2024},
		},
		{
			ID:         "meta-3",
			Filename:   "m3.pdf",
			Collection: "coll",
			Status:     DocumentStatusPending,
			Metadata:   map[string]interface{}{"type": "invoice", "year": 2023},
		},
	}

	for _, doc := range docs {
		err := repo.Register(ctx, doc)
		require.NoError(t, err)
	}

	// Filter by type
	filtered, err := repo.FilterByMetadata(ctx, map[string]interface{}{"type": "invoice"})
	require.NoError(t, err)
	assert.Len(t, filtered, 2)

	// Filter by multiple fields
	filtered, err = repo.FilterByMetadata(ctx, map[string]interface{}{
		"type": "invoice",
		"year": 2024,
	})
	require.NoError(t, err)
	assert.Len(t, filtered, 1)
	assert.Equal(t, "meta-1", filtered[0].ID)
}

func TestRedisDocumentRepository_Ping(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisDocumentRepository(client)
	ctx := context.Background()

	err := repo.Ping(ctx)
	assert.NoError(t, err)
}

func TestRedisDocumentRepository_Cleanup(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisDocumentRepository(client)
	ctx := context.Background()

	// Create old deleted document
	oldDoc := &Document{
		ID:         "old-doc",
		Filename:   "old.pdf",
		Collection: "coll",
		Status:     DocumentStatusDeleted,
	}

	err := repo.Register(ctx, oldDoc)
	require.NoError(t, err)

	// Manually set old created timestamp
	oldDoc.CreatedAt = time.Now().Add(-48 * time.Hour)
	docJSON, _ := json.Marshal(oldDoc)
	err = client.Set(ctx, documentKeyPrefix+oldDoc.ID, docJSON, 0).Err()
	require.NoError(t, err)

	// Create recent document
	recentDoc := &Document{
		ID:         "recent-doc",
		Filename:   "recent.pdf",
		Collection: "coll",
		Status:     DocumentStatusCompleted,
	}
	err = repo.Register(ctx, recentDoc)
	require.NoError(t, err)

	// Cleanup documents older than 24 hours
	count, err := repo.Cleanup(ctx, 24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify old doc is gone
	_, err = repo.Get(ctx, "old-doc")
	assert.Error(t, err)

	// Verify recent doc still exists
	_, err = repo.Get(ctx, "recent-doc")
	assert.NoError(t, err)
}

func TestRedisDocumentRepository_GetStats(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisDocumentRepository(client)
	ctx := context.Background()

	docs := []*Document{
		{ID: "stats-1", Filename: "s1.pdf", Collection: "coll-a", ChunkCount: 10, FileSize: 1000, Status: DocumentStatusPending},
		{ID: "stats-2", Filename: "s2.pdf", Collection: "coll-a", ChunkCount: 20, FileSize: 2000, Status: DocumentStatusCompleted},
		{ID: "stats-3", Filename: "s3.pdf", Collection: "coll-b", ChunkCount: 30, FileSize: 3000, Status: DocumentStatusCompleted},
	}

	for _, doc := range docs {
		err := repo.Register(ctx, doc)
		require.NoError(t, err)
	}

	stats, err := repo.GetStats(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, stats.TotalDocuments, 3)
	assert.GreaterOrEqual(t, stats.TotalChunks, 60)
	assert.GreaterOrEqual(t, stats.TotalSize, int64(6000))
	assert.Greater(t, stats.AverageChunkCount, 0.0)
}

func TestRedisDocumentRepository_ListCollections(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisDocumentRepository(client)
	ctx := context.Background()

	docs := []*Document{
		{ID: "lc-1", Filename: "lc1.pdf", Collection: "collection-x", Status: DocumentStatusPending},
		{ID: "lc-2", Filename: "lc2.pdf", Collection: "collection-y", Status: DocumentStatusPending},
		{ID: "lc-3", Filename: "lc3.pdf", Collection: "collection-x", Status: DocumentStatusPending},
	}

	for _, doc := range docs {
		err := repo.Register(ctx, doc)
		require.NoError(t, err)
	}

	collections, err := repo.ListCollections(ctx)
	require.NoError(t, err)
	assert.Contains(t, collections, "collection-x")
	assert.Contains(t, collections, "collection-y")
}

func TestRedisDocumentRepository_ClearCollection(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisDocumentRepository(client)
	ctx := context.Background()

	docs := []*Document{
		{ID: "clear-1", Filename: "c1.pdf", Collection: "clear-me", Status: DocumentStatusPending},
		{ID: "clear-2", Filename: "c2.pdf", Collection: "clear-me", Status: DocumentStatusPending},
		{ID: "keep-1", Filename: "k1.pdf", Collection: "keep-me", Status: DocumentStatusPending},
	}

	for _, doc := range docs {
		err := repo.Register(ctx, doc)
		require.NoError(t, err)
	}

	// Clear collection
	err := repo.ClearCollection(ctx, "clear-me")
	require.NoError(t, err)

	// Verify cleared
	clearedDocs, err := repo.ListByCollection(ctx, "clear-me")
	require.NoError(t, err)
	assert.Empty(t, clearedDocs)

	// Verify other collection untouched
	keptDocs, err := repo.ListByCollection(ctx, "keep-me")
	require.NoError(t, err)
	assert.Len(t, keptDocs, 1)
}

func TestRedisDocumentRepository_GetCollectionStats(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisDocumentRepository(client)
	ctx := context.Background()

	docs := []*Document{
		{ID: "cs-1", Filename: "cs1.pdf", Collection: "stats-coll", ChunkCount: 10, FileSize: 1000, Status: DocumentStatusPending},
		{ID: "cs-2", Filename: "cs2.pdf", Collection: "stats-coll", ChunkCount: 20, FileSize: 2000, Status: DocumentStatusCompleted},
	}

	for _, doc := range docs {
		err := repo.Register(ctx, doc)
		require.NoError(t, err)
	}

	stats, err := repo.GetCollectionStats(ctx, "stats-coll")
	require.NoError(t, err)
	assert.Equal(t, 2, stats["total_documents"])
	assert.Equal(t, 30, stats["total_chunks"])
	assert.Equal(t, int64(3000), stats["total_size"])
	assert.Equal(t, 15.0, stats["average_chunks"])
}
