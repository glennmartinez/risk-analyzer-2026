# Document Service

## Overview

The Document Service is a **Go-based orchestration layer** that coordinates document processing by calling Python compute endpoints and managing persistence through repository interfaces. It implements the **orchestration pattern** where Go handles business logic, state management, and coordination, while Python provides specialized compute capabilities (parsing, chunking, embedding, metadata extraction).

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Document Service                          │
│  (Go Orchestration - Business Logic & State Management)     │
└────────────┬──────────────────────────────────┬─────────────┘
             │                                  │
             ▼                                  ▼
┌────────────────────────┐         ┌──────────────────────────┐
│   Python Client        │         │   Repository Layer       │
│  (Compute Endpoints)   │         │  (Persistence)           │
└────────────────────────┘         └──────────────────────────┘
             │                                  │
             ▼                                  ▼
┌────────────────────────┐         ┌──────────────────────────┐
│  Python Backend        │         │  Redis + ChromaDB        │
│  - Parse               │         │  - Documents             │
│  - Chunk               │         │  - Jobs                  │
│  - Embed               │         │  - Vectors               │
│  - Metadata            │         │                          │
└────────────────────────┘         └──────────────────────────┘
```

## Key Features

- **Stateless Compute**: Delegates compute-heavy tasks to Python backend
- **State Management**: Manages document lifecycle, job tracking, and metadata
- **Transaction-like Semantics**: Rolls back on errors, updates status atomically
- **Async & Sync Modes**: Supports both immediate and queued processing
- **Comprehensive Logging**: Detailed progress tracking and error reporting
- **Batch Processing**: Efficient vector storage with batching
- **Auto-Recovery**: Handles partial failures gracefully

## Components

### DocumentService

Main orchestration service that coordinates document processing.

```go
type DocumentService struct {
    pythonClient PythonClientInterface
    docRepo      repositories.DocumentRepository
    vectorRepo   repositories.VectorRepository
    jobRepo      repositories.JobRepository
    logger       *log.Logger
}
```

### Processing Pipeline

The service implements a **5-step pipeline** for document processing:

1. **Parse** - Extract text from document (PDF, DOCX, TXT, etc.)
2. **Chunk** - Split text into semantic chunks
3. **Embed** - Generate vector embeddings for each chunk
4. **Metadata** - Extract document-level metadata (optional)
5. **Store** - Persist chunks and embeddings in vector database

## API

### UploadDocument

Orchestrates the full document upload and processing pipeline.

```go
func (s *DocumentService) UploadDocument(
    ctx context.Context, 
    req *UploadDocumentRequest,
) (*UploadDocumentResponse, error)
```

**Request Parameters:**
```go
type UploadDocumentRequest struct {
    Filename         string      // Required
    FileContent      io.Reader   // Required
    FileSize         int64       // File size in bytes
    Collection       string      // Required - vector collection name
    ChunkingStrategy string      // Default: "semantic"
    ChunkSize        int         // Default: 512
    ChunkOverlap     int         // Default: 50
    ExtractMetadata  bool        // Extract title, keywords, questions
    NumQuestions     int         // Number of questions to generate
    MaxPages         int         // 0 = unlimited
    Async            bool        // True = queue job, False = process immediately
}
```

**Response:**
```go
type UploadDocumentResponse struct {
    DocumentID       string                 // Unique document ID
    JobID            string                 // Job ID (if async)
    Filename         string                 // Original filename
    Collection       string                 // Collection name
    ChunkCount       int                    // Number of chunks created
    Status           string                 // "completed", "queued", "failed"
    ProcessingTimeMs float64                // Time taken (sync only)
    Metadata         map[string]interface{} // Extracted metadata
    Message          string                 // Status message
}
```

**Supported File Types:**
- `.pdf` - PDF documents
- `.txt` - Plain text
- `.md` - Markdown
- `.docx`, `.doc` - Microsoft Word
- `.html`, `.htm` - HTML documents

### DeleteDocument

Removes a document and all its chunks from the vector database.

```go
func (s *DocumentService) DeleteDocument(
    ctx context.Context, 
    documentID string,
) error
```

### GetDocument

Retrieves document metadata.

```go
func (s *DocumentService) GetDocument(
    ctx context.Context, 
    documentID string,
) (*repositories.Document, error)
```

### ListDocuments

Lists all documents.

```go
func (s *DocumentService) ListDocuments(
    ctx context.Context,
) ([]*repositories.Document, error)
```

### ListDocumentsByCollection

Lists documents in a specific collection.

```go
func (s *DocumentService) ListDocumentsByCollection(
    ctx context.Context, 
    collection string,
) ([]*repositories.Document, error)
```

### GetDocumentStatus

Retrieves the processing status of a document.

```go
func (s *DocumentService) GetDocumentStatus(
    ctx context.Context, 
    documentID string,
) (repositories.DocumentStatus, error)
```

**Possible Statuses:**
- `pending` - Document registered, waiting for processing
- `processing` - Currently being processed
- `completed` - Successfully processed and stored
- `failed` - Processing failed
- `deleted` - Document has been deleted

## Processing Modes

### Synchronous Mode

Process immediately and return results:

```go
req := &UploadDocumentRequest{
    Filename:    "report.pdf",
    FileContent: fileReader,
    Collection:  "reports",
    Async:       false,  // Synchronous
}

resp, err := service.UploadDocument(ctx, req)
// resp.ChunkCount, resp.ProcessingTimeMs available immediately
```

**Use Cases:**
- Small documents (< 10 pages)
- Interactive upload flows
- When immediate feedback is required

### Asynchronous Mode

Queue for background processing:

```go
req := &UploadDocumentRequest{
    Filename:    "large_document.pdf",
    FileContent: fileReader,
    Collection:  "reports",
    Async:       true,  // Asynchronous
}

resp, err := service.UploadDocument(ctx, req)
// resp.JobID available, poll for status later

// Later, check job status:
job, _ := jobRepo.GetJob(ctx, resp.JobID)
// job.Status, job.Progress, job.Result
```

**Use Cases:**
- Large documents (> 10 pages)
- Batch uploads
- When processing time is unpredictable

## Error Handling

The service implements **graceful error handling** with rollback semantics:

1. **Parse Failure** → Document marked as `failed`, no persistence
2. **Chunk Failure** → Document marked as `failed`, no persistence
3. **Embed Failure** → Document marked as `failed`, no persistence
4. **Metadata Failure** → Non-critical, continues with partial metadata
5. **Vector Storage Failure** → Document marked as `failed`, rollback

### Example Error Handling

```go
resp, err := service.UploadDocument(ctx, req)
if err != nil {
    // Check specific error types
    if errors.Is(err, repositories.DocumentAlreadyExistsError("")) {
        // Handle duplicate
    }
    // Generic error handling
    log.Printf("Upload failed: %v", err)
    return err
}

// Check status
if resp.Status == "failed" {
    // Handle failed processing
}
```

## Performance

### Batching

Chunks are stored in batches of 100 to optimize vector database operations:

```go
batchSize := 100
for i := 0; i < len(chunks); i += batchSize {
    batch := chunks[i:min(i+batchSize, len(chunks))]
    s.vectorRepo.StoreChunks(ctx, collection, batch)
}
```

### Metrics

- **Parse**: ~100-500ms for typical PDF (10 pages)
- **Chunk**: ~50-200ms (depends on strategy)
- **Embed**: ~100ms per batch of 32 chunks (local model)
- **Metadata**: ~200-500ms (if enabled)
- **Vector Storage**: ~50ms per 100 chunks

**Total**: Typically 1-2 seconds for a 10-page PDF with metadata extraction.

## Testing

Comprehensive unit tests with mocks for all dependencies:

```bash
# Run all document service tests
cd backend
go test -v ./internal/services -run "^Test.*Document"

# Run specific test
go test -v ./internal/services -run TestUploadDocumentSync_Success
```

### Test Coverage

- ✅ Request validation (file types, required fields)
- ✅ Synchronous upload (success path)
- ✅ Asynchronous upload (job creation)
- ✅ Error handling (parse, chunk, embed, storage failures)
- ✅ Collection auto-creation
- ✅ Metadata extraction (optional)
- ✅ Document deletion
- ✅ Document retrieval and listing
- ✅ Status tracking

## Usage Examples

### Basic Upload

```go
package main

import (
    "context"
    "os"
    "risk-analyzer/internal/services"
    "risk-analyzer/internal/repositories"
)

func main() {
    // Initialize dependencies
    pythonClient := services.NewPythonClient("http://localhost:8000")
    docRepo := repositories.NewRedisDocumentRepository(redisClient)
    vectorRepo := repositories.NewChromaVectorRepository(chromaClient)
    jobRepo := repositories.NewRedisJobRepository(redisClient)
    
    // Create service
    service := services.NewDocumentService(
        pythonClient,
        docRepo,
        vectorRepo,
        jobRepo,
        logger,
    )
    
    // Upload document
    file, _ := os.Open("document.pdf")
    defer file.Close()
    
    resp, err := service.UploadDocument(context.Background(), &services.UploadDocumentRequest{
        Filename:         "document.pdf",
        FileContent:      file,
        Collection:       "my-collection",
        ChunkingStrategy: "semantic",
        ExtractMetadata:  true,
        NumQuestions:     5,
        Async:            false,
    })
    
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Document uploaded: %s, chunks: %d", resp.DocumentID, resp.ChunkCount)
}
```

### Async Upload with Polling

```go
// Queue document
resp, _ := service.UploadDocument(ctx, &services.UploadDocumentRequest{
    Filename:    "large.pdf",
    FileContent: file,
    Collection:  "reports",
    Async:       true,
})

log.Printf("Job queued: %s", resp.JobID)

// Poll for completion
ticker := time.NewTicker(2 * time.Second)
defer ticker.Stop()

for {
    select {
    case <-ticker.C:
        job, _ := jobRepo.GetJob(ctx, resp.JobID)
        log.Printf("Progress: %d%% - %s", job.Progress, job.Message)
        
        if job.Status.IsTerminal() {
            if job.Status == repositories.JobStatusCompleted {
                log.Printf("Success! Document: %s", job.Result["document_id"])
            } else {
                log.Printf("Failed: %s", job.Error)
            }
            return
        }
    case <-ctx.Done():
        return
    }
}
```

## Dependencies

### Required

- **PythonClient** - Communication with Python compute backend
- **DocumentRepository** - Document metadata persistence (Redis)
- **VectorRepository** - Vector storage (ChromaDB)
- **JobRepository** - Job queue management (Redis)

### Optional

- **Logger** - Structured logging (stdlib log or custom)

## Configuration

### Defaults

```go
// Applied automatically if not specified
ChunkingStrategy: "semantic"
ChunkSize:        512
ChunkOverlap:     50
MaxPages:         0  // unlimited
NumQuestions:     3
Async:            false
```

### Tuning

**For Large Documents:**
- Set `Async: true`
- Increase `MaxPages` to limit processing
- Use `ChunkingStrategy: "fixed"` for speed

**For High Quality:**
- Use `ChunkingStrategy: "semantic"`
- Enable `ExtractMetadata: true`
- Increase `NumQuestions` for better context

**For Performance:**
- Use `ChunkingStrategy: "fixed"`
- Disable `ExtractMetadata`
- Increase batch size in vector storage

## Future Enhancements

- [ ] Support for more file types (PPT, Excel, images)
- [ ] Parallel chunk embedding
- [ ] Incremental document updates
- [ ] Document versioning
- [ ] Custom chunking strategies
- [ ] Embedding model selection per request
- [ ] Cost tracking and quotas
- [ ] Rate limiting

## Related

- [Python Client](./python_client.go) - HTTP client for Python backend
- [Repository Interfaces](../repositories/) - Data persistence layer
- [Legacy Document Service](./legacy_document_service.go) - Old implementation (deprecated)