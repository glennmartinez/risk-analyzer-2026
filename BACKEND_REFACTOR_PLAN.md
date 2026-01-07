# Backend Architecture Refactor Plan

## Executive Summary

The current architecture has **blurred responsibility boundaries** between Go and Python backends, leading to:
- Dual persistence management (both services talk to Redis/ChromaDB)
- Python doing too much business logic (document registry, upload orchestration)
- Go acting as a thin proxy instead of application orchestrator
- Ghost collection bugs from auto-creation in Python
- No single source of truth for state
- Difficult debugging and maintenance

**Goal**: Refactor to a **clean separation of concerns** where:
- **Go Backend** = Application orchestrator, persistence layer, business logic
- **Python Backend** = Pure ML/AI compute service (stateless transformations)

---

## Current Architecture Analysis

### Python Backend (Port 8000) - Currently Doing Too Much ❌

**What it does:**
```
├── Document parsing (Docling) ✅ ML task
├── Text chunking (LlamaIndex) ✅ ML task  
├── Embedding generation (SentenceTransformer) ✅ ML task
├── Vector storage (ChromaDB HTTP client) ❌ Persistence
├── Document registry (Redis client) ❌ Persistence
├── Upload workflow orchestration ❌ Business logic
├── Collection management ❌ Business logic
├── Search/RAG endpoints ❌ Business logic
└── Document lifecycle ❌ Business logic
```

**Files:** 17 Python files, ~2,500 LOC
**Services:**
- `vector_store.py` - Talks directly to ChromaDB
- `redis_service.py` - Talks directly to Redis
- `parser.py` - Document parsing (GOOD)
- `chunker.py` - Text chunking (GOOD)

**Routes:**
- `/documents/upload` - Full upload workflow
- `/documents/list` - Redis queries
- `/documents/vector` - ChromaDB queries
- `/documents/collection/{name}` - Collection deletion
- `/search/query` - Vector search
- `/rag/search` - RAG retrieval

### Go Backend (Port 8080) - Currently Too Thin ❌

**What it does:**
```
├── HTTP routing ⚠️ Just proxying
├── LLM chat handling ✅ Good
├── RAG orchestration ✅ Good
├── Proxying to Python ❌ Thin wrapper
└── No persistence layer ❌ Missing
```

**Files:** 18 Go files
**Current services:**
- `ms_documents.go` - Just HTTP client to Python
- `llm_service.go` - LLM interactions (GOOD)

**Missing:**
- No Redis client/connection
- No ChromaDB client/connection
- No database abstraction layer
- No business logic for document lifecycle

---

## Problems Identified

### 1. **Dual Persistence = Chaos**
Both Go and Python talk to databases:
```
Frontend → Go → Python → ChromaDB/Redis ❌
          ↓
        MySQL (Go only)
```

**Issues:**
- Python creates collections automatically (ghost collections bug)
- Go has no visibility into ChromaDB/Redis state
- Transaction boundaries unclear
- Can't enforce consistency

### 2. **Python Has Too Much Business Logic**
Example: `/documents/upload` endpoint (174 lines):
1. Validates file
2. Parses document ✅
3. Chunks document ✅
4. **Stores in ChromaDB** ❌
5. **Registers in Redis** ❌
6. **Orchestrates workflow** ❌

This should be:
- Go orchestrates workflow
- Python just: parse → chunk → embed → return

### 3. **No Single Source of Truth**
Where is document state?
- Redis (maybe)
- ChromaDB metadata (yes)
- MySQL (no)
- Go memory (no)

### 4. **Error-Prone State Management**
Example bugs we encountered:
- Collections auto-created on search
- Document in ChromaDB but not Redis
- Can't delete collections cleanly
- No transactional guarantees

### 5. **Poor Separation of Concerns**
Can't answer:
- "Who owns document lifecycle?" → Both (bad)
- "Who enforces business rules?" → Python (bad)
- "Who manages persistence?" → Both (bad)

---

## Target Architecture

### **Go Backend: Application Orchestrator & Persistence Layer**

```go
// Go owns ALL persistence and business logic

Frontend
   ↓
Go API Gateway
   ├── Business Logic Layer
   │   ├── Document lifecycle
   │   ├── Collection management
   │   ├── User workflows
   │   └── Validation/authorization
   │
   ├── Persistence Layer
   │   ├── MySQL (metadata, users, config)
   │   ├── Redis (document registry, cache)
   │   ├── ChromaDB (vector storage)
   │   └── Transaction coordination
   │
   └── External Services
       ├── Python ML Service (stateless)
       └── LM Studio (LLM)
```

**Responsibilities:**
- ✅ All database connections (MySQL, Redis, ChromaDB)
- ✅ Document registry management
- ✅ Collection lifecycle (create, delete, list)
- ✅ Upload workflow orchestration
- ✅ Business logic & validation
- ✅ Transaction coordination
- ✅ API gateway & routing
- ✅ Authentication & authorization
- ✅ Error handling & logging

### **Python Backend: Pure ML/AI Compute Service**

```python
# Python is STATELESS - pure transformations

Go Backend
   ↓
Python ML Service (FastAPI)
   ├── /parse (file_bytes) → ParsedDocument
   ├── /chunk (text, strategy) → Chunks[]
   ├── /embed (texts[]) → Embeddings[]
   ├── /extract-metadata (text) → Metadata
   └── /health → Status
```

**Responsibilities:**
- ✅ Document parsing (Docling)
- ✅ Text chunking (LlamaIndex)
- ✅ Embedding generation (SentenceTransformer)
- ✅ Metadata extraction (LLM)
- ✅ Pure stateless transformations
- ❌ NO database access
- ❌ NO business logic
- ❌ NO state management

---

## Refactor Plan

### **Phase 1: Add Persistence Layer to Go** (Critical)

#### 1.1 Add Go ChromaDB Client
```bash
go get github.com/amikos-tech/chroma-go
```

**New files:**
- `backend/internal/db/chromadb.go` - ChromaDB connection & wrapper
- `backend/internal/repositories/vector_repository.go` - Vector operations

**Interface:**
```go
type VectorRepository interface {
    // Collection management
    CreateCollection(name string) error
    DeleteCollection(name string) error
    ListCollections() ([]string, error)
    GetCollectionStats(name string) (*CollectionStats, error)
    
    // Document operations
    StoreChunks(collectionName string, chunks []Chunk) error
    SearchChunks(collectionName, query string, topK int) ([]SearchResult, error)
    DeleteDocument(collectionName, docID string) (int, error)
    ListDocuments(collectionName string) ([]VectorDocument, error)
}
```

#### 1.2 Add Go Redis Client
```bash
go get github.com/redis/go-redis/v9
```

**New files:**
- `backend/internal/db/redis.go` - Redis connection
- `backend/internal/repositories/document_repository.go` - Document registry

**Interface:**
```go
type DocumentRepository interface {
    Register(doc *Document) error
    Get(docID string) (*Document, error)
    List() ([]*Document, error)
    Delete(docID string) error
    Update(docID string, updates map[string]interface{}) error
}
```

#### 1.3 Database Package Structure
```
backend/internal/
├── db/
│   ├── chromadb.go      # ChromaDB connection
│   ├── redis.go         # Redis connection
│   └── mysql.go         # MySQL connection (if not exists)
│
├── repositories/
│   ├── vector_repository.go     # ChromaDB operations
│   ├── document_repository.go   # Redis operations
│   └── user_repository.go       # MySQL operations (future)
│
└── models/
    ├── document.go      # Document entity
    ├── chunk.go         # Chunk entity
    └── collection.go    # Collection entity
```

---

### **Phase 2: Refactor Python to Pure Compute Service**

#### 2.1 Remove Python Persistence Services

**DELETE:**
- `python-backend/app/services/vector_store.py` (500+ lines)
- `python-backend/app/services/redis_service.py` (140+ lines)

**KEEP:**
- `python-backend/app/services/parser.py` ✅
- `python-backend/app/services/chunker.py` ✅

#### 2.2 Simplify Python Routes

**Before:** `/documents/upload` (174 lines - does everything)
**After:** Multiple simple endpoints

```python
# NEW SIMPLIFIED ROUTES

@router.post("/parse")
async def parse_document(file: UploadFile) -> ParsedDocument:
    """Parse document bytes → structured text"""
    return parser.parse_bytes(await file.read(), file.filename)

@router.post("/chunk")
async def chunk_text(request: ChunkRequest) -> ChunkResponse:
    """Chunk text → array of chunks"""
    return chunker.chunk_document(
        request.text,
        strategy=request.strategy,
        chunk_size=request.chunk_size
    )

@router.post("/embed")
async def generate_embeddings(texts: List[str]) -> EmbeddingResponse:
    """Generate embeddings for texts"""
    embeddings = embedding_model.encode(texts)
    return {"embeddings": embeddings.tolist()}

@router.post("/extract-metadata")
async def extract_metadata(text: str) -> MetadataResponse:
    """Use LLM to extract metadata from text"""
    # Call LM Studio for keywords, summary, etc.
    return metadata_extractor.extract(text)
```

**DELETE entire routes:**
- `/documents/list` → Move to Go
- `/documents/vector` → Move to Go
- `/documents/collection/{name}` → Move to Go
- `/search/query` → Move to Go
- `/rag/search` → Move to Go

#### 2.3 Python Becomes Truly Stateless

**Configuration:**
```python
# NO ChromaDB client
# NO Redis client
# ONLY ML model loading

class Settings(BaseSettings):
    embedding_model: str = "sentence-transformers/all-MiniLM-L6-v2"
    # Remove: chroma_host, chroma_port, redis_host, etc.
```

---

### **Phase 3: Implement Go Orchestration Layer**

#### 3.1 Document Upload Workflow (in Go)

```go
// backend/internal/services/document_service.go

type DocumentService struct {
    vectorRepo    repositories.VectorRepository
    documentRepo  repositories.DocumentRepository
    pythonClient  *PythonClient
}

func (s *DocumentService) UploadDocument(ctx context.Context, 
    file io.Reader, 
    filename string, 
    opts UploadOptions) (*Job, error) {
    
    // NEW ASYNC APPROACH:
    // 1. Create job immediately
    jobID := uuid.New().String()
    job := &Job{
        ID:       jobID,
        Type:     JobTypeDocumentUpload,
        Status:   JobStatusQueued,
        Progress: 0,
        Payload: map[string]interface{}{
            "filename":   filename,
            "collection": opts.Collection,
            "chunk_size": opts.ChunkSize,
        },
    }
    
    // 2. Store file temporarily (or save bytes to payload)
    // 3. Enqueue job in Redis
    err := s.jobRepo.EnqueueJob(ctx, job)
    if err != nil {
        return nil, fmt.Errorf("failed to enqueue job: %w", err)
    }
    
    // 4. Return job ID immediately to frontend
    return job, nil
}

// Background worker processes the job:
func (w *UploadWorker) ProcessJob(ctx context.Context, job *Job) error {
    // Update: Processing
    w.jobRepo.UpdateJobStatus(ctx, job.ID, JobStatusProcessing, 10, "Parsing document...")
    
    // 1. Call Python: Parse
    parsed, err := w.pythonClient.ParseDocument(ctx, file, filename)
    if err != nil {
        return fmt.Errorf("parse failed: %w", err)
    }
    
    // Update: 30% done
    w.jobRepo.SetProgress(ctx, job.ID, 30, "Chunking text...")
    
    // 2. Call Python: Chunk
    chunks, err := w.pythonClient.ChunkText(ctx, ChunkRequest{
        Text: parsed.Text,
        Strategy: opts.ChunkingStrategy,
        ChunkSize: opts.ChunkSize,
    })
    if err != nil {
        return fmt.Errorf("chunk failed: %w", err)
    }
    
    // Update: 60% done
    w.jobRepo.SetProgress(ctx, job.ID, 60, "Generating embeddings...")
    
    // 3. Call Python: Embed
    embeddings, err := w.pythonClient.GenerateEmbeddings(ctx, 
        extractTexts(chunks))
    if err != nil {
        return fmt.Errorf("embed failed: %w", err)
    }
    
    // Update: 80% done
    w.jobRepo.SetProgress(ctx, job.ID, 80, "Storing in vector DB...")
    
    // 4. Store in ChromaDB (Go owns this)
    docID := uuid.New().String()
    err = s.vectorRepo.StoreChunks(opts.Collection, ChunksWithEmbeddings{
        DocumentID: docID,
        Chunks: chunks,
        Embeddings: embeddings,
    })
    if err != nil {
        return nil, fmt.Errorf("vector store failed: %w", err)
    }
    
    // 5. Register in Redis (Go owns this)
    doc := &Document{
        ID: docID,
        Filename: filename,
        Collection: opts.Collection,
        ChunkCount: len(chunks),
        UploadedAt: time.Now(),
    }
    err = s.documentRepo.Register(doc)
    if err != nil {
        // Rollback: delete from ChromaDB
        s.vectorRepo.DeleteDocument(opts.Collection, docID)
        return nil, fmt.Errorf("registry failed: %w", err)
    }
    
    return doc, nil
}
```

**Key improvements:**
- ✅ Go orchestrates entire workflow
- ✅ Python is just compute (parse, chunk, embed)
- ✅ Go owns persistence (ChromaDB, Redis)
- ✅ Transaction-like rollback on errors
- ✅ Single source of truth

#### 3.2 Search/RAG Workflow (in Go)

```go
func (s *SearchService) SearchDocuments(ctx context.Context, 
    query string, 
    collection string, 
    topK int) (*SearchResponse, error) {
    
    // 1. Validate collection exists (Go knows state)
    collections, err := s.vectorRepo.ListCollections()
    if err != nil {
        return nil, err
    }
    if !contains(collections, collection) {
        return nil, fmt.Errorf("collection '%s' not found. Available: %v", 
            collection, collections)
    }
    
    // 2. Generate query embedding via Python
    embedding, err := s.pythonClient.GenerateEmbeddings(ctx, []string{query})
    if err != nil {
        return nil, fmt.Errorf("embedding failed: %w", err)
    }
    
    // 3. Search ChromaDB directly (Go owns this)
    results, err := s.vectorRepo.SearchChunks(collection, embedding[0], topK)
    if err != nil {
        return nil, fmt.Errorf("search failed: %w", err)
    }
    
    return &SearchResponse{
        Query: query,
        Results: results,
        TotalResults: len(results),
    }, nil
}
```

**Benefits:**
- ✅ Go validates collection before search
- ✅ No ghost collections (Go creates them explicitly)
- ✅ Python just does embeddings
- ✅ Go has full visibility into vector store

---

### **Phase 4: New Service Structure**

#### Go Backend Structure
```
backend/
├── cmd/
│   └── server/
│       └── main.go
│
├── internal/
│   ├── db/                      # Database connections
│   │   ├── chromadb.go
│   │   ├── redis.go
│   │   └── mysql.go
│   │
│   ├── repositories/            # Data access layer
│   │   ├── vector_repository.go
│   │   ├── document_repository.go
│   │   └── user_repository.go
│   │
│   ├── services/                # Business logic
│   │   ├── document_service.go  # Upload, delete, lifecycle
│   │   ├── search_service.go    # Search, RAG
│   │   ├── collection_service.go # Collection management
│   │   ├── python_client.go     # HTTP client to Python
│   │   └── llm_service.go       # LM Studio client
│   │
│   ├── handlers/                # HTTP handlers
│   │   ├── document_handler.go
│   │   ├── search_handler.go
│   │   ├── collection_handler.go
│   │   └── chat_handler.go
│   │
│   ├── models/                  # Domain models
│   │   ├── document.go
│   │   ├── chunk.go
│   │   ├── collection.go
│   │   └── search.go
│   │
│   └── routes/
│       └── routes.go
│
└── go.mod
```

#### Python Backend Structure (Simplified)
```
python-backend/
├── app/
│   ├── main.py                  # FastAPI app
│   │
│   ├── services/                # ML services only
│   │   ├── parser.py           # Docling
│   │   ├── chunker.py          # LlamaIndex
│   │   ├── embedder.py         # SentenceTransformer
│   │   └── metadata_extractor.py # LLM metadata
│   │
│   ├── routes/                  # Simple compute endpoints
│   │   ├── parse.py
│   │   ├── chunk.py
│   │   ├── embed.py
│   │   └── metadata.py
│   │
│   ├── models.py                # Pydantic models (DTOs)
│   └── config.py                # ML model configs only
│
└── requirements.txt
```

---

## Migration Strategy

### **Step-by-Step Migration** (Minimize Disruption)

#### Week 1: Foundation
1. ✅ Add ChromaDB Go client
2. ✅ Add Redis Go client  
3. ✅ Create repository interfaces
4. ✅ Write integration tests for persistence layer

#### Week 2: Parallel Implementation
5. ✅ Implement `document_service.go` (new upload workflow)
6. ✅ Implement `search_service.go` (new search)
7. ✅ Keep old Python endpoints running
8. ✅ Add feature flag to switch between old/new

#### Week 3: Python Simplification
9. ✅ Create new Python endpoints (`/parse`, `/chunk`, `/embed`)
10. ✅ Test new Go → Python flow
11. ✅ Migrate one endpoint at a time (e.g., upload first)

#### Week 4: Cutover
12. ✅ Switch all traffic to Go orchestration
13. ✅ Delete old Python routes
14. ✅ Delete Python persistence services
15. ✅ Update frontend to call Go only

#### Week 5: Cleanup
16. ✅ Remove Python ChromaDB/Redis dependencies
17. ✅ Update documentation
18. ✅ Performance testing
19. ✅ Monitor and fix issues

---

## Benefits After Refactor

### **1. Clear Responsibility Boundaries**
```
Go = "What to do, when, and where to store it"
Python = "How to transform data (ML/AI)"
```

### **2. Single Source of Truth**
- Go owns ALL database state
- No confusion about where data lives
- Easier debugging

### **3. Better Error Handling**
```go
// Transactional rollback possible
if err := vectorRepo.Store(chunks); err != nil {
    return err
}
if err := docRepo.Register(doc); err != nil {
    vectorRepo.Delete(docID) // Rollback
    return err
}
```

### **4. No More Ghost Collections**
- Go explicitly creates collections
- Python can't accidentally create them
- Validation before any operation

### **5. Easier Testing**
```go
// Mock Python client
mockPython := &MockPythonClient{
    ParseFunc: func(...) { return mockData },
}
service := NewDocumentService(vectorRepo, docRepo, mockPython)
```

### **6. Better Performance Control**
- Go can batch operations
- Connection pooling in one place
- Caching strategies in Go

### **7. Scalability**
```
Go (stateless orchestrator)
  ├── Python Instance 1 (stateless)
  ├── Python Instance 2 (stateless)
  └── Python Instance 3 (stateless)
  
Shared State:
  ├── ChromaDB (Go manages)
  ├── Redis (Go manages)
  └── MySQL (Go manages)
```

### **8. Simpler Python Service**
- Fewer dependencies
- Faster startup
- Easier to scale
- Pure ML focus

---

## File-by-File Changes

### **Go Files to CREATE:**

1. `backend/internal/db/chromadb.go` (~150 lines)
2. `backend/internal/db/redis.go` (~100 lines)
3. `backend/internal/repositories/vector_repository.go` (~300 lines)
4. `backend/internal/repositories/document_repository.go` (~200 lines)
5. `backend/internal/services/document_service.go` (~400 lines)
6. `backend/internal/services/search_service.go` (~250 lines)
7. `backend/internal/services/collection_service.go` (~150 lines)
8. `backend/internal/services/python_client.go` (~200 lines)
9. `backend/internal/handlers/document_handler.go` (~250 lines)
10. `backend/internal/handlers/search_handler.go` (~150 lines)
11. `backend/internal/handlers/collection_handler.go` (~100 lines)

**Total new Go code: ~2,250 lines**

### **Go Files to MODIFY:**

1. `backend/internal/routes/routes.go` - Update routing
2. `backend/internal/models/document.go` - Enhance models
3. `backend/internal/services/llm_service.go` - Integrate with new search

### **Go Files to DELETE:**

1. `backend/internal/services/ms_documents.go` - Replaced by direct DB access

### **Python Files to CREATE:**

1. `python-backend/app/routes/parse.py` (~50 lines)
2. `python-backend/app/routes/chunk.py` (~50 lines)
3. `python-backend/app/routes/embed.py` (~50 lines)
4. `python-backend/app/routes/metadata.py` (~50 lines)
5. `python-backend/app/services/embedder.py` (~80 lines)

**Total new Python code: ~280 lines**

### **Python Files to DELETE:**

1. `python-backend/app/services/vector_store.py` (500 lines)
2. `python-backend/app/services/redis_service.py` (140 lines)
3. `python-backend/app/routes/documents.py` (500 lines)
4. `python-backend/app/routes/search.py` (150 lines)
5. `python-backend/app/routes/rag.py` (330 lines)

**Total Python code REMOVED: ~1,620 lines**

### **Python Files to MODIFY:**

1. `python-backend/app/main.py` - Remove old routes, add new
2. `python-backend/app/config.py` - Remove DB configs
3. `python-backend/requirements.txt` - Remove chromadb, redis

---

## Code Examples

### Example 1: Go ChromaDB Repository

```go
// backend/internal/repositories/vector_repository.go

package repositories

import (
    chroma "github.com/amikos-tech/chroma-go"
    "context"
)

type ChromaVectorRepository struct {
    client *chroma.Client
}

func NewChromaVectorRepository(host string, port int) (*ChromaVectorRepository, error) {
    client, err := chroma.NewClient(chroma.WithBasePath(fmt.Sprintf("http://%s:%d", host, port)))
    if err != nil {
        return nil, err
    }
    return &ChromaVectorRepository{client: client}, nil
}

func (r *ChromaVectorRepository) CreateCollection(ctx context.Context, name string) error {
    _, err := r.client.CreateCollection(ctx, name, map[string]interface{}{
        "hnsw:space": "cosine",
    }, true, nil, nil)
    return err
}

func (r *ChromaVectorRepository) StoreChunks(ctx context.Context, 
    collectionName string, 
    chunks []*Chunk) error {
    
    collection, err := r.client.GetCollection(ctx, collectionName, nil)
    if err != nil {
        return fmt.Errorf("collection not found: %w", err)
    }
    
    ids := make([]string, len(chunks))
    documents := make([]string, len(chunks))
    embeddings := make([][]float32, len(chunks))
    metadatas := make([]map[string]interface{}, len(chunks))
    
    for i, chunk := range chunks {
        ids[i] = chunk.ID
        documents[i] = chunk.Text
        embeddings[i] = chunk.Embedding
        metadatas[i] = chunk.Metadata
    }
    
    _, err = collection.Add(ctx, embeddings, metadatas, documents, ids)
    return err
}

func (r *ChromaVectorRepository) SearchChunks(ctx context.Context,
    collectionName string,
    queryEmbedding []float32,
    topK int) ([]*SearchResult, error) {
    
    collection, err := r.client.GetCollection(ctx, collectionName, nil)
    if err != nil {
        return nil, fmt.Errorf("collection not found: %w", err)
    }
    
    results, err := collection.Query(ctx, [][]float32{queryEmbedding}, topK, nil, nil, nil)
    if err != nil {
        return nil, err
    }
    
    // Convert to SearchResult structs
    searchResults := make([]*SearchResult, 0)
    // ... conversion logic
    
    return searchResults, nil
}
```

### Example 2: Simplified Python Endpoint

```python
# python-backend/app/routes/parse.py

from fastapi import APIRouter, File, UploadFile, HTTPException
from pydantic import BaseModel
from app.services.parser import DocumentParser

router = APIRouter(prefix="/parse", tags=["compute"])
parser = DocumentParser()

class ParseResponse(BaseModel):
    text: str
    metadata: dict
    page_count: int

@router.post("/", response_model=ParseResponse)
async def parse_document(file: UploadFile = File(...)) -> ParseResponse:
    """
    Parse document bytes into structured text.
    STATELESS: No database access, pure transformation.
    """
    try:
        file_bytes = await file.read()
        parsed = parser.parse_bytes(file_bytes, file.filename)
        
        return ParseResponse(
            text=parsed.text,
            metadata=parsed.metadata,
            page_count=parsed.page_count
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))
```

### Example 3: Go Document Service

```go
// backend/internal/services/document_service.go

package services

type DocumentService struct {
    vectorRepo   repositories.VectorRepository
    docRepo      repositories.DocumentRepository
    pythonClient *PythonClient
    logger       *log.Logger
}

func (s *DocumentService) UploadDocument(ctx context.Context, 
    file io.Reader, 
    opts *UploadOptions) (*Document, error) {
    
    s.logger.Info("Starting document upload", "filename", opts.Filename)
    
    // Step 1: Parse (Python)
    parsed, err := s.pythonClient.Parse(ctx, file, opts.Filename)
    if err != nil {
        return nil, fmt.Errorf("parse failed: %w", err)
    }
    
    // Step 2: Chunk (Python)
    chunks, err := s.pythonClient.Chunk(ctx, &ChunkRequest{
        Text: parsed.Text,
        Strategy: opts.ChunkStrategy,
        ChunkSize: opts.ChunkSize,
    })
    if err != nil {
        return nil, fmt.Errorf("chunk failed: %w", err)
    }
    
    // Step 3: Embed (Python)
    texts := make([]string, len(chunks))
    for i, chunk := range chunks {
        texts[i] = chunk.Text
    }
    embeddings, err := s.pythonClient.Embed(ctx, texts)
    if err != nil {
        return nil, fmt.Errorf("embed failed: %w", err)
    }
    
    // Combine chunks with embeddings
    for i := range chunks {
        chunks[i].Embedding = embeddings[i]
    }
    
    // Step 4: Store in vector DB (Go owns this)
    docID := uuid.NewString()
    err = s.vectorRepo.StoreChunks(ctx, opts.Collection, chunks)
    if err != nil {
        return nil, fmt.Errorf("vector store failed: %w", err)
    }
    
    // Step 5: Register in Redis (Go owns this)
    doc := &Document{
        ID: docID,
        Filename: opts.Filename,
        Collection: opts.Collection,
        ChunkCount: len(chunks),
        CreatedAt: time.Now(),
    }
    
    err = s.docRepo.Register(ctx, doc)
    if err != nil {
        // Rollback: delete from vector store
        s.vectorRepo.DeleteDocument(ctx, opts.Collection, docID)
        return nil, fmt.Errorf("registry failed: %w", err)
    }
    
    s.logger.Info("Document uploaded successfully", "doc_id", docID)
    return doc, nil
}
```

---

## Testing Strategy

### Go Testing
```go
// backend/internal/services/document_service_test.go

func TestDocumentService_Upload(t *testing.T) {
    // Mock Python client
    mockPython := &MockPythonClient{
        ParseFunc: func(ctx context.Context, file io.Reader, filename string) (*ParsedDocument, error) {
            return &ParsedDocument{Text: "test content"}, nil
        },
        ChunkFunc: func(ctx context.Context, req *ChunkRequest) ([]*Chunk, error) {
            return []*Chunk{{Text: "chunk1"}}, nil
        },
        EmbedFunc: func(ctx context.Context, texts []string) ([][]float32, error) {
            return [][]float32{{0.1, 0.2}}, nil
        },
    }
    
    // Mock repositories
    mockVectorRepo := &MockVectorRepository{}
    mockDocRepo := &MockDocumentRepository{}
    
    service := NewDocumentService(mockVectorRepo, mockDocRepo, mockPython)
    
    doc, err := service.UploadDocument(context.Background(), strings.NewReader("test"), &UploadOptions{
        Filename: "test.pdf",
        Collection: "test",
    })
    
    assert.NoError(t, err)
    assert.NotNil(t, doc)
}
```

---

## Performance Considerations

### Before Refactor:
```
Frontend → Go → Python → ChromaDB
                  ↓
                Redis
```
- Extra network hop for every operation
- Python serialization overhead
- Can't optimize database access

### After Refactor:
```
Frontend → Go → ChromaDB (direct)
           ↓
         Redis (direct)
           ↓
         Python (only for compute)
```
- Direct database access from Go
- Connection pooling
- Batch operations possible
- Python only called when needed

**Expected improvements:**
- Search: 30-50% faster (eliminate Python proxy)
- Upload: Similar (still calls Python for ML)
- List operations: 60-80% faster (direct Redis/ChromaDB)

---

## Rollback Plan

If things go wrong:

1. **Feature flag**: Toggle between old/new implementation
2. **Keep old Python endpoints** during migration
3. **Gradual migration**: One endpoint at a time
4. **Monitoring**: Compare performance old vs new
5. **Easy revert**: Just switch feature flag back

```go
// Feature flag example
if config.UseNewArchitecture {
    return newDocumentService.Upload(ctx, file, opts)
} else {
    return oldPythonProxyService.Upload(ctx, file, opts)
}
```

---

## Success Metrics

### Technical Metrics:
- ✅ Search latency reduced by 30%+
- ✅ Zero ghost collections created
- ✅ 100% test coverage on new services
- ✅ Python service LOC reduced by 60%
- ✅ Go service LOC increased by ~2,000 (but well-structured)

### Operational Metrics:
- ✅ Easier debugging (single source of truth)
- ✅ Faster development (clear boundaries)
- ✅ Fewer bugs (no dual persistence)
- ✅ Better error messages
- ✅ Simpler deployment

---

## Timeline

**Total estimated time: 4-5 weeks**

- Week 1: Go persistence layer (ChromaDB, Redis clients)
- Week 2: Go services (document, search, collection)
- Week 3: Python simplification + integration
- Week 4: Migration + testing
- Week 5: Cleanup + documentation

---

## Conclusion

This refactor addresses the root cause of current issues:

**Problem**: Blurred boundaries, dual persistence, unclear ownership
**Solution**: Clean separation - Go = orchestrator/persistence, Python = compute

**Result**: 
- Maintainable codebase
- Clear responsibility boundaries  
- Single source of truth
- Easier to debug and test
- Better performance
- Scalable architecture

The investment (~4-5 weeks) will pay off in:
- Faster development velocity
- Fewer bugs
- Easier onboarding
- Better system reliability

---

## Next Steps

1. **Review this plan** with team
2. **Prioritize**: Can do in phases (persistence first, then migration)
3. **Spike**: Build ChromaDB + Redis clients in Go (1-2 days)
4. **Prototype**: One endpoint end-to-end with new architecture
5. **Decision**: Go/no-go based on prototype
6. **Execute**: Follow week-by-week plan

---

**Questions to Answer:**

1. Do we have bandwidth for 4-5 week refactor?
2. Can we maintain old system during migration?
3. What's the risk tolerance?
4. Any missing requirements?
5. Performance benchmarks to hit?

Let's discuss and refine this plan before starting implementation.

---

## Task Checklist

### **Phase 1: Foundation - Add Persistence Layer to Go** 

#### Task 1.1: Set Up Go Dependencies
- [x] Add ChromaDB Go client: `go get github.com/amikos-tech/chroma-go`
- [x] Add Redis Go client: `go get github.com/redis/go-redis/v9`
- [x] Update `go.mod` and verify dependencies compile
- [x] Test basic connectivity to ChromaDB (port 8001) - Note: Go client has v1/v2 API issues, will use direct HTTP calls
- [x] Test basic connectivity to Redis (port 6379) - ✅ All tests passing

#### Task 1.2: Create Database Connection Layer
- [x] Create `backend/internal/db/chromadb.go` - ChromaDB connection wrapper
- [x] Create `backend/internal/db/redis.go` - Redis connection wrapper
- [x] Add connection pooling configuration
- [x] Add health check methods for each database
- [x] Write unit tests for connection handling

#### Task 1.3: Create Repository Interfaces
- [x] Create `backend/internal/repositories/vector_repository.go` interface
- [x] Create `backend/internal/repositories/document_repository.go` interface
- [x] Define all required methods (CRUD operations)
- [x] Add proper error types and handling
- [x] Document interfaces with examples

#### Task 1.3.1: Create Job Queue Interface (ADDED - Critical for Async Operations)
- [x] Create `backend/internal/repositories/job_repository.go` interface
- [x] Define Job, JobStatus, JobType models
- [x] Define UploadJobPayload and UploadJobResult
- [x] Add job queue operations (enqueue, dequeue)
- [x] Add job progress tracking
- [x] Add validation helpers
- [x] Document async upload workflow

#### Task 1.4: Implement ChromaDB Repository
- [x] Implement `CreateCollection(name string) error`
- [x] Implement `DeleteCollection(name string) error`
- [x] Implement `ListCollections() ([]string, error)`
- [x] Implement `GetCollectionStats(name string) (*CollectionStats, error)`
- [x] Implement `StoreChunks(collection string, chunks []Chunk) error`
- [x] Implement `SearchChunks(collection, query string, topK int) ([]SearchResult, error)`
- [x] Implement `DeleteDocument(collection, docID string) (int, error)`
- [x] Implement `ListDocuments(collection string) ([]VectorDocument, error)`
- [x] Write integration tests with test ChromaDB instance

#### Task 1.5: Implement Redis Document Repository ✅
- [x] Implement `Register(doc *Document) error`
- [x] Implement `Get(docID string) (*Document, error)`
- [x] Implement `List() ([]*Document, error)`
- [x] Implement `Delete(docID string) error`
- [x] Implement `Update(docID string, updates map[string]interface{}) error`
- [x] Add transaction support for atomic operations
- [x] Write integration tests with test Redis instance
- [x] Implemented batch operations (RegisterBatch, GetBatch, DeleteBatch)
- [x] Implemented query operations (ListByCollection, ListByStatus, CountByCollection, etc.)
- [x] Implemented helper methods (GetStats, ListCollections, ClearCollection, etc.)
- [x] Created comprehensive test suite with 762 lines of tests

#### Task 1.6: Implement Redis Job Repository (ADDED - For Async Upload) ✅
- [x] Implement `CreateJob(job *Job) error`
- [x] Implement `GetJob(jobID string) (*Job, error)`
- [x] Implement `UpdateJobStatus(jobID, status, progress, message) error`
- [x] Implement `EnqueueJob(job *Job) error`
- [x] Implement `DequeueJob(jobType JobType) (*Job, error)`
- [x] Implement `SetProgress(jobID, progress, message) error`
- [x] Implement job cleanup methods (CleanupCompletedJobs, CleanupFailedJobs)
- [x] Write integration tests for job queue
- [x] Test concurrent job processing
- [x] Implemented priority-based queue with Redis sorted sets
- [x] Implemented retry logic with RequeueFailedJobs
- [x] Implemented ListJobs with filtering support
- [x] Implemented GetStats helper method
- [x] Created comprehensive test suite with 757 lines of tests

#### Task 1.6.1: Update Domain Models ✅
- [x] Update `backend/internal/models/document.go` with new fields
- [x] Create `backend/internal/models/chunk.go`
- [x] Create `backend/internal/models/collection.go`
- [x] Create `backend/internal/models/job.go`
- [x] Add JSON serialization tags
- [x] Add validation methods
- [x] Added DTO conversion methods (ToDTO/FromDTO)
- [x] Added domain-specific validation logic
- [x] Added helper types (DocumentStatus, JobStatus, JobType, etc.)
- [x] Created 4 model files with comprehensive DTOs and validation

#### Task 1.7: Create Background Worker Pattern (ADDED - For Job Processing) ✅
- [x] Create `backend/internal/workers/worker.go` interface
- [x] Create `backend/internal/workers/upload_worker.go` implementation
- [x] Add worker pool management
- [x] Add graceful shutdown handling
- [x] Add error recovery and retry logic
- [x] Write worker tests
- [x] Implemented BaseWorker with statistics tracking
- [x] Implemented WorkerPool for managing multiple workers
- [x] Implemented UploadWorker with full document processing pipeline
- [x] Added RecoverableJobProcessor for panic recovery
- [x] Added configurable concurrency and retry settings
- [x] Created comprehensive test suite with mocks (1286 lines total)
- [x] Created detailed README with usage examples and best practices

---

### **Phase 2: Simplify Python Backend**

#### Task 2.1: Create New Simplified Python Endpoints ✅
- [x] Create `python-backend/app/routes/parse.py` - Parse endpoint (221 lines)
- [x] Create `python-backend/app/routes/chunk.py` - Chunk endpoint (282 lines)
- [x] Create `python-backend/app/routes/embed.py` - Embed endpoint (312 lines)
- [x] Create `python-backend/app/routes/metadata.py` - Metadata extraction endpoint (326 lines)
- [x] Update `python-backend/app/main.py` to include new routes
- [x] Update `python-backend/app/routes/__init__.py` to export new routers
- [x] All endpoints are **pure compute** - no persistence, no side effects
- [x] Reused existing parser and chunker logic - only changed the interface
- [x] Added health check endpoints for each service
- [x] Added multiple variants (full/simple/specific) for flexibility
- [ ] Write unit tests for each new endpoint (TODO)

#### Task 2.2: Create Python Embedder Service
- [x] Create `python-backend/app/services/embedder.py`
- [x] Extract embedding logic from vector_store.py
- [x] Add batch embedding support
- [x] Add model caching
- [x] Write unit tests

#### Task 2.3: Update Python Models
- [x] Create Pydantic models for parse request/response
- [x] Create Pydantic models for chunk request/response
- [x] Create Pydantic models for embed request/response
- [x] Create Pydantic models for metadata request/response
- [x] Add validation rules
- [x] Create centralized models file (models_compute.py)

#### Task 2.4: Remove Python Persistence (KEEP OLD ROUTES FOR NOW)
- [x] Keep old routes for rollback/compatibility
- [x] New stateless endpoints are primary (parse, chunk, embed, metadata)
- [x] Old routes (/documents, /search, /rag) still available but deprecated
- [x] Config.py unchanged - ChromaDB/Redis settings preserved for rollback
- [x] Python service runs with both old and new endpoints

---

### **Phase 3: Implement Go Orchestration Layer**

#### Task 3.1: Create Python Client in Go
- [x] Create `backend/internal/services/python_client.go`
- [x] Implement `Parse(ctx, file, filename) (*ParsedDoc, error)`
- [x] Implement `Chunk(ctx, *ChunkRequest) ([]*Chunk, error)`
- [x] Implement `Embed(ctx, texts []string) ([][]float32, error)`
- [x] Implement `ExtractMetadata(ctx, text string) (*Metadata, error)`
- [x] Add retry logic and timeouts
- [x] Add connection pooling
- [x] Write unit tests with mock HTTP server (20+ tests, all passing)

#### Task 3.2: Create Document Service ✅
- [x] Create `backend/internal/services/document_service.go`
- [x] Implement `UploadDocument(ctx, file, opts) (*Document, error)`
- [x] Implement `DeleteDocument(ctx, docID, collection) error`
- [x] Implement `ListDocuments(ctx, collection) ([]*Document, error)`
- [x] Implement `GetDocument(ctx, docID) (*Document, error)`
- [x] Add transaction-like rollback on errors
- [x] Add logging and metrics
- [x] Write unit tests with mocked dependencies (23 tests, all passing)

#### Task 3.3: Create Search Service ✅
- [x] Create `backend/internal/services/search_service.go`
- [x] Implement `SearchDocuments(ctx, query, collection, topK) (*SearchResponse, error)`
- [x] Implement collection validation before search
- [x] Add caching layer for frequent queries (in-memory cache with TTL)
- [x] Write unit tests (15 tests, all passing)

#### Task 3.4: Create Collection Service ✅
- [x] Create `backend/internal/services/collection_service.go`
- [x] Implement `CreateCollection(ctx, name string) error`
- [x] Implement `DeleteCollection(ctx, name string) error`
- [x] Implement `ListCollections(ctx) ([]string, error)`
- [x] Implement `GetCollectionStats(ctx, name) (*Stats, error)`
- [x] Add validation (prevent auto-creation, validate names)
- [x] Write unit tests (20 tests, all passing)

#### Task 3.5: Create HTTP Handlers ✅
- [x] Create `backend/internal/handlers/document_handler.go`
- [x] Create `backend/internal/handlers/search_handler.go`
- [x] Create `backend/internal/handlers/collection_handler.go`
- [x] Implement upload handler (multipart form data)
- [x] Implement list documents handler (with collection filter)
- [x] Implement delete document handler
- [x] Implement search handler (POST with JSON, GET with query params)
- [x] Implement collection CRUD handlers
- [x] Add request validation
- [x] Add error handling with proper HTTP codes
- [ ] Write integration tests (deferred to Phase 4)

#### Task 3.6: Update Routes ✅
- [x] Update `backend/internal/routes/routes.go`
- [x] Add new document routes (/api/v1/documents/*)
- [x] Add new search routes (/api/v1/search)
- [x] Add new collection routes (/api/v1/collections/*)
- [x] Update `backend/internal/server/server.go` for service initialization
- [x] Add dependency injection pattern with Handlers struct
- [x] Keep old routes alongside new ones (for rollback)
- [x] All new routes under /api/v1/* namespace

---

### **Phase 4: Integration & Migration**

#### Task 4.1: Integration Testing
- [ ] Test full upload workflow (Go → Python → ChromaDB → Redis)
- [ ] Test search workflow (Go → ChromaDB direct)
- [ ] Test collection management (Go → ChromaDB direct)
- [ ] Test error scenarios and rollbacks
- [ ] Test with real documents (PDF, text, etc.)
- [ ] Load testing with multiple concurrent uploads
- [ ] Verify no ghost collections created

#### Task 4.2: Update Frontend Integration
- [ ] Update frontend API client to call new Go endpoints
- [ ] Add feature flag in frontend for old/new backend
- [ ] Test upload from UI
- [ ] Test search from UI
- [ ] Test document management from UI
- [ ] Verify all existing features work

#### Task 4.3: Parallel Running & Testing
- [ ] Run old and new systems side-by-side
- [ ] Compare response times (old vs new)
- [ ] Compare results accuracy
- [ ] Monitor error rates
- [ ] Fix any discovered issues
- [ ] Document performance improvements

#### Task 4.4: Gradual Migration
- [ ] Migrate upload endpoint (feature flag on)
- [ ] Monitor for 24 hours
- [ ] Migrate search endpoint
- [ ] Monitor for 24 hours
- [ ] Migrate collection management
- [ ] Monitor for 24 hours
- [ ] Verify all data consistency

---

### **Phase 5: Cleanup & Finalization**

#### Task 5.1: Remove Old Python Code
- [ ] Delete `python-backend/app/services/vector_store.py`
- [ ] Delete `python-backend/app/services/redis_service.py`
- [ ] Delete `python-backend/app/routes/documents.py`
- [ ] Delete `python-backend/app/routes/search.py`
- [ ] Delete `python-backend/app/routes/rag.py`
- [ ] Remove ChromaDB from `requirements.txt`
- [ ] Remove Redis from `requirements.txt`
- [ ] Clean up unused imports

#### Task 5.2: Remove Old Go Code
- [ ] Delete `backend/internal/services/ms_documents.go`
- [ ] Remove old route handlers
- [ ] Remove feature flags (keep new implementation only)
- [ ] Clean up unused imports

#### Task 5.3: Update Configuration
- [ ] Update `python-backend/app/config.py` (remove DB configs)
- [ ] Update `docker-compose.yml` (if needed)
- [ ] Update environment variables documentation
- [ ] Add Go database connection configs
- [ ] Test with clean environment

#### Task 5.4: Documentation
- [ ] Update `README.md` with new architecture
- [ ] Document new API endpoints
- [ ] Create architecture diagrams
- [ ] Document Go services and repositories
- [ ] Add code examples for common operations
- [ ] Update deployment documentation

#### Task 5.5: Performance & Monitoring
- [ ] Add metrics/logging to all new Go services
- [ ] Set up performance dashboards
- [ ] Document performance improvements
- [ ] Add alerting for errors
- [ ] Create runbook for common issues

#### Task 5.6: Final Testing
- [ ] Full end-to-end testing
- [ ] Regression testing (all old features work)
- [ ] Load testing (handle expected traffic)
- [ ] Security testing (no new vulnerabilities)
- [ ] User acceptance testing
- [ ] Sign-off from stakeholders

---

### **Progress Tracking**

**Phase 1 (Foundation):** 4.5/9 tasks complete (50%) - Added 3 tasks for async job queue
**Phase 2 (Python Simplification):** 0/4 tasks complete (0%)
**Phase 3 (Go Orchestration):** 0/6 tasks complete (0%)
**Phase 4 (Integration):** 0/4 tasks complete (0%)
**Phase 5 (Cleanup):** 0/6 tasks complete (0%)

**Overall Progress:** 4.5/29 major tasks complete (16%) - Includes async job queue infrastructure

---

### **Risk Mitigation**

- [ ] Create rollback scripts for each phase
- [ ] Set up monitoring before starting migration
- [ ] Keep old system running during entire migration
- [ ] Feature flags in place for instant rollback
- [ ] Database backups before major changes
- [ ] Communicate timeline to users
- [ ] Have rollback plan tested and ready

---

### **Success Criteria**

- [ ] All tests passing (unit, integration, e2e)
- [ ] Performance improved by 30%+ on search operations
- [ ] Zero ghost collections in production
- [ ] Python LOC reduced by 60%
- [ ] Clear code ownership and boundaries
- [ ] Documentation complete and reviewed
- [ ] Team trained on new architecture
- [ ] Monitoring and alerting in place
- [ ] Production running stable for 1 week