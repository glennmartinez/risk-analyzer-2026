package services

import (
	"context"
	"errors"
	"log"
	"os"
	"testing"

	"risk-analyzer/internal/repositories"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ============================================================================
// Test Setup
// ============================================================================

func setupTestCollectionService(t *testing.T) (*CollectionService, *MockVectorRepository, *MockDocumentRepository) {
	mockVectorRepo := new(MockVectorRepository)
	mockDocRepo := new(MockDocumentRepository)

	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)

	service := NewCollectionService(
		mockVectorRepo,
		mockDocRepo,
		logger,
	)

	return service, mockVectorRepo, mockDocRepo
}

// ============================================================================
// Tests
// ============================================================================

func TestNewCollectionService(t *testing.T) {
	service, _, _ := setupTestCollectionService(t)

	assert.NotNil(t, service)
	assert.NotNil(t, service.vectorRepo)
	assert.NotNil(t, service.docRepo)
	assert.NotNil(t, service.logger)
}

func TestValidateCollectionName(t *testing.T) {
	service, _, _ := setupTestCollectionService(t)

	tests := []struct {
		name        string
		collection  string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid name",
			collection:  "my-collection",
			expectError: false,
		},
		{
			name:        "Valid with underscore",
			collection:  "my_collection_123",
			expectError: false,
		},
		{
			name:        "Empty name",
			collection:  "",
			expectError: true,
			errorMsg:    "collection name is required",
		},
		{
			name:        "Too short",
			collection:  "ab",
			expectError: true,
			errorMsg:    "must be at least 3 characters",
		},
		{
			name:        "Too long",
			collection:  "this_is_a_very_long_collection_name_that_exceeds_the_maximum_allowed_length_of_63_characters",
			expectError: true,
			errorMsg:    "must be at most 63 characters",
		},
		{
			name:        "Invalid characters - space",
			collection:  "my collection",
			expectError: true,
			errorMsg:    "contains invalid character",
		},
		{
			name:        "Invalid characters - special",
			collection:  "my-collection!",
			expectError: true,
			errorMsg:    "contains invalid character",
		},
		{
			name:        "Valid minimum length",
			collection:  "abc",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.validateCollectionName(tt.collection)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateCollection_Success(t *testing.T) {
	service, mockVectorRepo, _ := setupTestCollectionService(t)
	ctx := context.Background()

	req := &CreateCollectionRequest{
		Name: "test-collection",
		Metadata: map[string]interface{}{
			"description": "Test collection",
		},
	}

	// Setup mocks
	mockVectorRepo.On("CollectionExists", ctx, req.Name).Return(false, nil)
	mockVectorRepo.On("CreateCollection", ctx, req.Name, req.Metadata).Return(nil)

	// Execute
	err := service.CreateCollection(ctx, req)

	// Assert
	assert.NoError(t, err)

	mockVectorRepo.AssertExpectations(t)
}

func TestCreateCollection_AlreadyExists(t *testing.T) {
	service, mockVectorRepo, _ := setupTestCollectionService(t)
	ctx := context.Background()

	req := &CreateCollectionRequest{
		Name: "test-collection",
	}

	// Setup mocks
	mockVectorRepo.On("CollectionExists", ctx, req.Name).Return(true, nil)

	// Execute
	err := service.CreateCollection(ctx, req)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	mockVectorRepo.AssertExpectations(t)
}

func TestCreateCollection_InvalidName(t *testing.T) {
	service, _, _ := setupTestCollectionService(t)
	ctx := context.Background()

	req := &CreateCollectionRequest{
		Name: "ab", // Too short
	}

	// Execute
	err := service.CreateCollection(ctx, req)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid collection name")
}

func TestCreateCollection_CheckExistenceFails(t *testing.T) {
	service, mockVectorRepo, _ := setupTestCollectionService(t)
	ctx := context.Background()

	req := &CreateCollectionRequest{
		Name: "test-collection",
	}

	// Setup mocks
	mockVectorRepo.On("CollectionExists", ctx, req.Name).Return(false, errors.New("connection error"))

	// Execute
	err := service.CreateCollection(ctx, req)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check collection existence")

	mockVectorRepo.AssertExpectations(t)
}

func TestCreateCollection_CreateFails(t *testing.T) {
	service, mockVectorRepo, _ := setupTestCollectionService(t)
	ctx := context.Background()

	req := &CreateCollectionRequest{
		Name: "test-collection",
	}

	// Setup mocks
	mockVectorRepo.On("CollectionExists", ctx, req.Name).Return(false, nil)
	mockVectorRepo.On("CreateCollection", ctx, req.Name, req.Metadata).Return(errors.New("creation failed"))

	// Execute
	err := service.CreateCollection(ctx, req)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create collection")

	mockVectorRepo.AssertExpectations(t)
}

func TestDeleteCollection_Success(t *testing.T) {
	service, mockVectorRepo, mockDocRepo := setupTestCollectionService(t)
	ctx := context.Background()
	collectionName := "test-collection"

	docs := []*repositories.Document{
		{ID: "doc1", Collection: collectionName},
		{ID: "doc2", Collection: collectionName},
	}

	// Setup mocks
	mockVectorRepo.On("CollectionExists", ctx, collectionName).Return(true, nil)
	mockDocRepo.On("ListByCollection", ctx, collectionName).Return(docs, nil)
	mockVectorRepo.On("DeleteCollection", ctx, collectionName).Return(nil)
	mockDocRepo.On("Update", ctx, "doc1", mock.AnythingOfType("map[string]interface {}")).Return(nil)
	mockDocRepo.On("Update", ctx, "doc2", mock.AnythingOfType("map[string]interface {}")).Return(nil)

	// Execute
	resp, err := service.DeleteCollection(ctx, collectionName)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, collectionName, resp.CollectionName)
	assert.Equal(t, 2, resp.DocumentsCount)
	assert.Equal(t, 2, resp.DeletedDocs)
	assert.True(t, resp.Success)

	mockVectorRepo.AssertExpectations(t)
	mockDocRepo.AssertExpectations(t)
}

func TestDeleteCollection_NotFound(t *testing.T) {
	service, mockVectorRepo, _ := setupTestCollectionService(t)
	ctx := context.Background()
	collectionName := "non-existent"

	// Setup mocks
	mockVectorRepo.On("CollectionExists", ctx, collectionName).Return(false, nil)

	// Execute
	resp, err := service.DeleteCollection(ctx, collectionName)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "collection not found")

	mockVectorRepo.AssertExpectations(t)
}

func TestDeleteCollection_InvalidName(t *testing.T) {
	service, _, _ := setupTestCollectionService(t)
	ctx := context.Background()

	// Execute
	resp, err := service.DeleteCollection(ctx, "ab")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "invalid collection name")
}

func TestDeleteCollection_WithDocUpdateFailures(t *testing.T) {
	service, mockVectorRepo, mockDocRepo := setupTestCollectionService(t)
	ctx := context.Background()
	collectionName := "test-collection"

	docs := []*repositories.Document{
		{ID: "doc1", Collection: collectionName},
		{ID: "doc2", Collection: collectionName},
	}

	// Setup mocks - one document update will fail
	mockVectorRepo.On("CollectionExists", ctx, collectionName).Return(true, nil)
	mockDocRepo.On("ListByCollection", ctx, collectionName).Return(docs, nil)
	mockVectorRepo.On("DeleteCollection", ctx, collectionName).Return(nil)
	mockDocRepo.On("Update", ctx, "doc1", mock.AnythingOfType("map[string]interface {}")).Return(nil)
	mockDocRepo.On("Update", ctx, "doc2", mock.AnythingOfType("map[string]interface {}")).Return(errors.New("update failed"))

	// Execute
	resp, err := service.DeleteCollection(ctx, collectionName)

	// Assert - should still succeed even if doc updates fail
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 2, resp.DocumentsCount)
	assert.Equal(t, 1, resp.DeletedDocs) // Only one update succeeded

	mockVectorRepo.AssertExpectations(t)
	mockDocRepo.AssertExpectations(t)
}

func TestListCollections_Success(t *testing.T) {
	service, mockVectorRepo, _ := setupTestCollectionService(t)
	ctx := context.Background()

	expectedCollections := []string{"collection1", "collection2", "collection3"}

	// Setup mocks
	mockVectorRepo.On("ListCollections", ctx).Return(expectedCollections, nil)

	// Execute
	collections, err := service.ListCollections(ctx)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, collections)
	assert.Len(t, collections, 3)
	assert.Equal(t, expectedCollections, collections)

	mockVectorRepo.AssertExpectations(t)
}

func TestListCollections_Fails(t *testing.T) {
	service, mockVectorRepo, _ := setupTestCollectionService(t)
	ctx := context.Background()

	// Setup mocks
	mockVectorRepo.On("ListCollections", ctx).Return(nil, errors.New("list failed"))

	// Execute
	collections, err := service.ListCollections(ctx)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, collections)
	assert.Contains(t, err.Error(), "failed to list collections")

	mockVectorRepo.AssertExpectations(t)
}

func TestGetCollectionInfo_Success(t *testing.T) {
	service, mockVectorRepo, mockDocRepo := setupTestCollectionService(t)
	ctx := context.Background()
	collectionName := "test-collection"

	stats := &repositories.CollectionStats{
		Name:       collectionName,
		ChunkCount: 150,
		Metadata: map[string]interface{}{
			"created_at": "2024-01-01",
		},
	}

	// Setup mocks
	mockVectorRepo.On("CollectionExists", ctx, collectionName).Return(true, nil)
	mockVectorRepo.On("GetCollectionStats", ctx, collectionName).Return(stats, nil)
	mockDocRepo.On("CountByCollection", ctx, collectionName).Return(10, nil)

	// Execute
	info, err := service.GetCollectionInfo(ctx, collectionName)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, info)
	assert.Equal(t, collectionName, info.Name)
	assert.Equal(t, 10, info.DocumentCount)
	assert.Equal(t, 150, info.ChunkCount)
	assert.NotNil(t, info.Metadata)

	mockVectorRepo.AssertExpectations(t)
	mockDocRepo.AssertExpectations(t)
}

func TestGetCollectionInfo_NotFound(t *testing.T) {
	service, mockVectorRepo, _ := setupTestCollectionService(t)
	ctx := context.Background()
	collectionName := "non-existent"

	// Setup mocks
	mockVectorRepo.On("CollectionExists", ctx, collectionName).Return(false, nil)

	// Execute
	info, err := service.GetCollectionInfo(ctx, collectionName)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, info)
	assert.Contains(t, err.Error(), "collection not found")

	mockVectorRepo.AssertExpectations(t)
}

func TestGetCollectionStats_Success(t *testing.T) {
	service, mockVectorRepo, _ := setupTestCollectionService(t)
	ctx := context.Background()
	collectionName := "test-collection"

	stats := &repositories.CollectionStats{
		Name:          collectionName,
		DocumentCount: 5,
		ChunkCount:    50,
	}

	// Setup mocks
	mockVectorRepo.On("CollectionExists", ctx, collectionName).Return(true, nil)
	mockVectorRepo.On("GetCollectionStats", ctx, collectionName).Return(stats, nil)

	// Execute
	result, err := service.GetCollectionStats(ctx, collectionName)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, stats, result)

	mockVectorRepo.AssertExpectations(t)
}

func TestCollectionExists_Success(t *testing.T) {
	service, mockVectorRepo, _ := setupTestCollectionService(t)
	ctx := context.Background()
	collectionName := "test-collection"

	// Setup mocks
	mockVectorRepo.On("CollectionExists", ctx, collectionName).Return(true, nil)

	// Execute
	exists, err := service.CollectionExists(ctx, collectionName)

	// Assert
	assert.NoError(t, err)
	assert.True(t, exists)

	mockVectorRepo.AssertExpectations(t)
}

func TestCollectionExists_NotFound(t *testing.T) {
	service, mockVectorRepo, _ := setupTestCollectionService(t)
	ctx := context.Background()
	collectionName := "non-existent"

	// Setup mocks
	mockVectorRepo.On("CollectionExists", ctx, collectionName).Return(false, nil)

	// Execute
	exists, err := service.CollectionExists(ctx, collectionName)

	// Assert
	assert.NoError(t, err)
	assert.False(t, exists)

	mockVectorRepo.AssertExpectations(t)
}

func TestCollectionExists_InvalidName(t *testing.T) {
	service, _, _ := setupTestCollectionService(t)
	ctx := context.Background()

	// Execute
	exists, err := service.CollectionExists(ctx, "ab")

	// Assert
	assert.Error(t, err)
	assert.False(t, exists)
	assert.Contains(t, err.Error(), "invalid collection name")
}
