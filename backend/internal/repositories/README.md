# Repository Layer

This package defines repository interfaces for data access abstraction. Repositories provide a clean separation between business logic and data persistence.

## Overview

The repository layer provides:
- **VectorRepository**: Interface for ChromaDB vector database operations
- **DocumentRepository**: Interface for Redis document registry operations
- Clean abstraction for testing (easy to mock)
- Proper error handling with custom error types
- Type-safe domain models

## Architecture

```
Services (Business Logic)
    ↓
Repositories (Interfaces - this package)
    ↓
Database Clients (internal/db)
    ↓
ChromaDB / Redis / MySQL
```

## Files

### `vector_repository.go`
Defines the interface for vector database operations.

**Key Methods:**
- **Collection Management**: Create, Delete, Get, List collections
- **Document Operations**: Store, Search, Delete chunks
- **Batch Operations**: Bulk store/delete for performance
- **Health**: Ping, Close

**Domain Models:**
- `Chunk` - Text chunk with embedding and metadata
- `SearchResult` - Vector similarity search result
- `VectorDocument` - Document metadata from vector DB
- `CollectionInfo` - Collection metadata
- `CollectionStats` - Collection statistics

**Usage:**
```go
type VectorRepository interface {
    CreateCollection(ctx, name, metadata) error
    StoreChunks(ctx, collection, chunks) error
    SearchChunks(ctx, collection, embedding, topK, filter) ([]*SearchResult, error)
    DeleteDocument(ctx, collection, docID) (int, error)
    ListCollections(ctx) ([]string, error)
    // ... more methods
}
```

### `document_repository.go`
Defines the interface for document registry operations (Redis-backed).

**Key Methods:**
- **Registry Operations**: Register, Get, List, Update, Delete documents
- **Bulk Operations**: Batch register/get/delete
- **Query Operations**: List by collection, status, filter by metadata
- **Search**: Find by filename, filter by criteria
- **Cleanup**: Remove old documents

**Domain Models:**
- `Document` - Document metadata and processing info
- `DocumentStatus` - Status enum (pending, processing, completed, failed, deleted)
- `DocumentFilter` - Filter criteria for queries
- `DocumentStats` - Statistics aggregation

**Usage:**
```go
type DocumentRepository interface {
    Register(ctx, doc) error
    Get(ctx, docID) (*Document, error)
    List(ctx) ([]*Document, error)
    Update(ctx, docID, updates) error
    Delete(ctx, docID) error
    ListByCollection(ctx, collection) ([]*Document, error)
    // ... more methods
}
```

## Domain Models

### Vector Repository Models

#### Chunk
```go
type Chunk struct {
    ID         string                 // Unique chunk ID
    DocumentID string                 // Parent document ID
    Text       string                 // Chunk text content
    Embedding  []float32              // Vector embedding
    Metadata   map[string]interface{} // Additional metadata
    ChunkIndex int                    // Order in document
    PageNumber *int                   // Optional page number
    TokenCount *int                   // Optional token count
}
```

#### SearchResult
```go
type SearchResult struct {
    ChunkID    string                 // Matched chunk ID
    DocumentID string                 // Parent document ID
    Text       string                 // Chunk text
    Score      float32                // Similarity score (0-1, higher better)
    Distance   float32                // Vector distance
    Metadata   map[string]interface{} // Chunk metadata
}
```

#### VectorDocument
```go
type VectorDocument struct {
    DocumentID string // Document ID
    Filename   string // Original filename
    Title      string // Document title
    ChunkCount int    // Number of chunks
    Collection string // Collection name
}
```

### Document Repository Models

#### Document
```go
type Document struct {
    ID               string                 // Document UUID
    Filename         string                 // Original filename
    Collection       string                 // Target collection
    ChunkCount       int                    // Number of chunks
    FileSize         int64                  // File size in bytes
    Status           DocumentStatus         // Processing status
    StoredInVectorDB bool                   // Stored in ChromaDB?
    CreatedAt        time.Time              // Creation timestamp
    UpdatedAt        time.Time              // Last update timestamp
    Metadata         map[string]interface{} // Extra metadata
    
    // Processing config
    ChunkingStrategy string
    ChunkSize        int
    ChunkOverlap     int
    ExtractMetadata  bool
    NumQuestions     int
    MaxPages         int
    
    // LLM config
    LLMProvider string
    LLMModel    string
}
```

#### DocumentStatus
```go
type DocumentStatus string

const (
    DocumentStatusPending    DocumentStatus = "pending"
    DocumentStatusProcessing DocumentStatus = "processing"
    DocumentStatusCompleted  DocumentStatus = "completed"
    DocumentStatusFailed     DocumentStatus = "failed"
    DocumentStatusDeleted    DocumentStatus = "deleted"
)
```

## Error Handling

### Vector Repository Errors

Custom error type with operation context:
```go
type VectorRepositoryError struct {
    Operation string
    Err       error
    Message   string
}
```

**Common Errors:**
- `CollectionNotFoundError(name)` - Collection doesn't exist
- `CollectionAlreadyExistsError(name)` - Collection already exists
- `DocumentNotFoundError(docID)` - Document not found
- `ChunkNotFoundError(chunkID)` - Chunk not found

**Usage:**
```go
collection, err := repo.GetCollection(ctx, "my-collection")
if err != nil {
    var repoErr *VectorRepositoryError
    if errors.As(err, &repoErr) {
        if repoErr.Operation == "get_collection" {
            // Handle collection not found
        }
    }
}
```

### Document Repository Errors

Custom error type with document context:
```go
type DocumentRepositoryError struct {
    Operation  string
    DocumentID string
    Err        error
    Message    string
}
```

**Common Errors:**
- `DocumentNotFoundError(docID)` - Document doesn't exist
- `DocumentAlreadyExistsError(docID)` - Document already registered
- `InvalidDocumentError(docID, reason)` - Validation failed

## Validation

### Document Validation
```go
doc := &Document{
    ID:         "doc-123",
    Filename:   "test.pdf",
    Collection: "documents",
    ChunkCount: 10,
}

if err := doc.Validate(); err != nil {
    // Handle validation error
}
```

### Status Validation
```go
status := DocumentStatusCompleted
if !status.IsValid() {
    // Invalid status
}
```

## Implementation Guide

### Implementing VectorRepository

```go
type ChromaVectorRepository struct {
    client *db.ChromaDBClient
}

func NewChromaVectorRepository(client *db.ChromaDBClient) VectorRepository {
    return &ChromaVectorRepository{client: client}
}

func (r *ChromaVectorRepository) CreateCollection(ctx context.Context, name string, metadata map[string]interface{}) error {
    _, err := r.client.CreateCollection(ctx, name, metadata)
    if err != nil {
        return NewVectorRepositoryError("create_collection", err, "")
    }
    return nil
}

// Implement other interface methods...
```

### Implementing DocumentRepository

```go
type RedisDocumentRepository struct {
    client *db.RedisClient
}

func NewRedisDocumentRepository(client *db.RedisClient) DocumentRepository {
    return &RedisDocumentRepository{client: client}
}

func (r *RedisDocumentRepository) Register(ctx context.Context, doc *Document) error {
    if err := doc.Validate(); err != nil {
        return err
    }
    
    // Convert to Redis hash
    err := r.client.HSet(ctx, "doc:"+doc.ID,
        "document_id", doc.ID,
        "filename", doc.Filename,
        // ... other fields
    )
    if err != nil {
        return NewDocumentRepositoryError("register", doc.ID, err, "")
    }
    
    // Add to set of all docs
    return r.client.SAdd(ctx, "docs:all", doc.ID)
}

// Implement other interface methods...
```

## Testing

### Mocking Repositories

Repositories are interfaces, making them easy to mock:

```go
type MockVectorRepository struct {
    CreateCollectionFunc func(ctx context.Context, name string, metadata map[string]interface{}) error
    StoreChunksFunc      func(ctx context.Context, collection string, chunks []*Chunk) error
    SearchChunksFunc     func(ctx context.Context, collection string, embedding []float32, topK int, filter map[string]interface{}) ([]*SearchResult, error)
    // ... other methods
}

func (m *MockVectorRepository) CreateCollection(ctx context.Context, name string, metadata map[string]interface{}) error {
    if m.CreateCollectionFunc != nil {
        return m.CreateCollectionFunc(ctx, name, metadata)
    }
    return nil
}

// Usage in tests
func TestDocumentService(t *testing.T) {
    mockRepo := &MockVectorRepository{
        StoreChunksFunc: func(ctx context.Context, collection string, chunks []*Chunk) error {
            return nil // Success
        },
    }
    
    service := NewDocumentService(mockRepo, ...)
    // Test service logic
}
```

## Best Practices

### 1. Always Use Context
```go
// Good ✅
doc, err := repo.Get(ctx, "doc-123")

// Bad ❌
doc, err := repo.Get("doc-123") // No context
```

### 2. Handle Errors Properly
```go
// Good ✅
if err := repo.Register(ctx, doc); err != nil {
    var docErr *DocumentRepositoryError
    if errors.As(err, &docErr) {
        log.Error("Failed to register", "op", docErr.Operation, "doc", docErr.DocumentID)
    }
    return err
}

// Bad ❌
repo.Register(ctx, doc) // Ignoring error
```

### 3. Validate Before Persisting
```go
// Good ✅
if err := doc.Validate(); err != nil {
    return err
}
err := repo.Register(ctx, doc)

// Bad ❌
err := repo.Register(ctx, doc) // No validation
```

### 4. Use Batch Operations When Possible
```go
// Good ✅ - Single operation
err := repo.RegisterBatch(ctx, documents)

// Less efficient ❌ - Multiple operations
for _, doc := range documents {
    repo.Register(ctx, doc)
}
```

### 5. Close Resources
```go
repo := NewChromaVectorRepository(client)
defer repo.Close()
```

## Next Steps

After defining interfaces:
1. **Task 1.4**: Implement ChromaDB repository
2. **Task 1.5**: Implement Redis repository
3. **Task 3.x**: Use repositories in service layer

See `BACKEND_REFACTOR_PLAN.md` for complete architecture.

## Task 1.3 Status

**Completed:**
- ✅ Vector repository interface defined
- ✅ Document repository interface defined
- ✅ Domain models created
- ✅ Error types and constructors
- ✅ Validation helpers
- ✅ Documentation

**Files Created:**
- `vector_repository.go` (160 lines)
- `document_repository.go` (193 lines)
- `README.md` (this file)

**Total:** ~353 lines of well-documented interfaces ready for implementation.