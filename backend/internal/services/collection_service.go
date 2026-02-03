package services

import (
	"context"
	"fmt"
	"log"

	"risk-analyzer/internal/repositories"
)

// CollectionService manages vector database collections
type CollectionService struct {
	vectorRepo repositories.VectorRepository
	docRepo    repositories.DocumentRepository
	logger     *log.Logger
}

// NewCollectionService creates a new collection service
func NewCollectionService(
	vectorRepo repositories.VectorRepository,
	docRepo repositories.DocumentRepository,
	logger *log.Logger,
) *CollectionService {
	return &CollectionService{
		vectorRepo: vectorRepo,
		docRepo:    docRepo,
		logger:     logger,
	}
}

// CollectionInfo represents collection metadata
type CollectionInfo struct {
	Name          string                 `json:"name"`
	DocumentCount int                    `json:"document_count"`
	ChunkCount    int                    `json:"chunk_count"`
	DocumentIDs   []string               `json:"document_ids,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// CreateCollectionRequest represents a request to create a collection
type CreateCollectionRequest struct {
	Name     string                 `json:"name"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// CreateCollection creates a new vector collection
func (s *CollectionService) CreateCollection(ctx context.Context, req *CreateCollectionRequest) error {
	s.logger.Printf("Creating collection: %s", req.Name)

	// Validate request
	if err := s.validateCollectionName(req.Name); err != nil {
		return fmt.Errorf("invalid collection name: %w", err)
	}

	// Check if collection already exists
	exists, err := s.vectorRepo.CollectionExists(ctx, req.Name)
	if err != nil {
		return fmt.Errorf("failed to check collection existence: %w", err)
	}

	if exists {
		return fmt.Errorf("collection already exists: %s", req.Name)
	}

	// Create collection
	if err := s.vectorRepo.CreateCollection(ctx, req.Name, req.Metadata); err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	s.logger.Printf("Collection created successfully: %s", req.Name)
	return nil
}

// DeleteCollection deletes a collection and all its documents
func (s *CollectionService) DeleteCollection(ctx context.Context, name string) (*DeleteCollectionResponse, error) {
	s.logger.Printf("Deleting collection: %s", name)

	// Validate collection name
	if err := s.validateCollectionName(name); err != nil {
		return nil, fmt.Errorf("invalid collection name: %w", err)
	}

	// Check if collection exists
	exists, err := s.vectorRepo.CollectionExists(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to check collection existence: %w", err)
	}

	if !exists {
		return nil, fmt.Errorf("collection not found: %s", name)
	}

	// Get document count before deletion
	docs, err := s.docRepo.ListByCollection(ctx, name)
	if err != nil {
		s.logger.Printf("Failed to list documents (non-critical): %v", err)
		docs = []*repositories.Document{}
	}
	documentCount := len(docs)

	// Delete from vector database
	if err := s.vectorRepo.DeleteCollection(ctx, name); err != nil {
		return nil, fmt.Errorf("failed to delete collection: %w", err)
	}

	// Delete documents from registry (Redis) - must be 1:1 with ChromaDB
	deletedDocs := 0
	for _, doc := range docs {
		if err := s.docRepo.Delete(ctx, doc.ID); err != nil {
			s.logger.Printf("Failed to delete document %s from registry: %v", doc.ID, err)
		} else {
			deletedDocs++
		}
	}

	s.logger.Printf("Collection deleted: %s (documents: %d)", name, documentCount)

	return &DeleteCollectionResponse{
		CollectionName: name,
		DocumentsCount: documentCount,
		DeletedDocs:    deletedDocs,
		Success:        true,
	}, nil
}

// DeleteCollectionResponse represents the response from deleting a collection
type DeleteCollectionResponse struct {
	CollectionName string `json:"collection_name"`
	DocumentsCount int    `json:"documents_count"`
	DeletedDocs    int    `json:"deleted_docs"`
	Success        bool   `json:"success"`
}

// ListCollections lists all available collections
func (s *CollectionService) ListCollections(ctx context.Context) ([]string, error) {
	s.logger.Printf("Listing collections")

	collections, err := s.vectorRepo.ListCollections(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}

	s.logger.Printf("Found %d collections", len(collections))
	return collections, nil
}

// GetCollectionInfo retrieves detailed information about a collection
func (s *CollectionService) GetCollectionInfo(ctx context.Context, name string) (*CollectionInfo, error) {
	s.logger.Printf("Getting collection info: %s", name)

	// Validate collection name
	if err := s.validateCollectionName(name); err != nil {
		return nil, fmt.Errorf("invalid collection name: %w", err)
	}

	// Check if collection exists
	exists, err := s.vectorRepo.CollectionExists(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to check collection existence: %w", err)
	}

	if !exists {
		return nil, fmt.Errorf("collection not found: %s", name)
	}

	// Get collection stats from vector DB
	stats, err := s.vectorRepo.GetCollectionStats(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection stats: %w", err)
	}

	// Get documents from registry (Redis)
	docs, err := s.docRepo.ListByCollection(ctx, name)
	if err != nil {
		s.logger.Printf("Failed to list documents from registry (non-critical): %v", err)
		docs = nil
	}

	// Build document IDs list from Redis
	var documentIDs []string
	if len(docs) > 0 {
		documentIDs = make([]string, len(docs))
		for i, doc := range docs {
			documentIDs[i] = doc.ID
		}
	} else {
		// Fall back to ChromaDB if Redis has no documents
		vectorDocs, err := s.vectorRepo.ListDocuments(ctx, name)
		if err == nil && len(vectorDocs) > 0 {
			documentIDs = make([]string, len(vectorDocs))
			for i, doc := range vectorDocs {
				documentIDs[i] = doc.DocumentID
			}
		}
	}

	docCount := len(documentIDs)
	if docCount == 0 {
		docCount = stats.DocumentCount
	}

	info := &CollectionInfo{
		Name:          stats.Name,
		DocumentCount: docCount,
		ChunkCount:    stats.ChunkCount,
		DocumentIDs:   documentIDs,
		Metadata:      stats.Metadata,
	}

	return info, nil
}

// GetCollectionStats retrieves statistics for a collection
func (s *CollectionService) GetCollectionStats(ctx context.Context, name string) (*repositories.CollectionStats, error) {
	s.logger.Printf("Getting collection stats: %s", name)

	// Validate collection name
	if err := s.validateCollectionName(name); err != nil {
		return nil, fmt.Errorf("invalid collection name: %w", err)
	}

	// Check if collection exists
	exists, err := s.vectorRepo.CollectionExists(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to check collection existence: %w", err)
	}

	if !exists {
		return nil, fmt.Errorf("collection not found: %s", name)
	}

	// Get stats
	stats, err := s.vectorRepo.GetCollectionStats(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection stats: %w", err)
	}

	return stats, nil
}

// CollectionExists checks if a collection exists
func (s *CollectionService) CollectionExists(ctx context.Context, name string) (bool, error) {
	if err := s.validateCollectionName(name); err != nil {
		return false, fmt.Errorf("invalid collection name: %w", err)
	}

	exists, err := s.vectorRepo.CollectionExists(ctx, name)
	if err != nil {
		return false, fmt.Errorf("failed to check collection existence: %w", err)
	}

	return exists, nil
}

// validateCollectionName validates a collection name
func (s *CollectionService) validateCollectionName(name string) error {
	if name == "" {
		return fmt.Errorf("collection name is required")
	}

	if len(name) < 3 {
		return fmt.Errorf("collection name must be at least 3 characters")
	}

	if len(name) > 63 {
		return fmt.Errorf("collection name must be at most 63 characters")
	}

	// Check for valid characters (alphanumeric, dash, underscore)
	for _, ch := range name {
		if !((ch >= 'a' && ch <= 'z') ||
			(ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') ||
			ch == '-' || ch == '_') {
			return fmt.Errorf("collection name contains invalid character: %c", ch)
		}
	}

	return nil
}
