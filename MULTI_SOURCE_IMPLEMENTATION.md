# Multi-Source Document Processing Implementation

## Overview

This document describes the implementation of multi-source document processing support in the Risk Analyzer API. The system now supports ingesting content from multiple sources beyond traditional file uploads, including Jira issues, Notion pages, Markdown, and JSON content.

## Architecture

### Key Design Decision: Use Existing Python Endpoints

Instead of creating new Python endpoints, the implementation leverages **existing endpoints**:

1. **`/chunk/text`** - Accepts JSON with `text` field, chunks it, returns chunks with metadata
2. **`/embed/batch`** - Accepts `texts` array, returns embeddings

This means **no Python changes are required** - the Go side normalizes content and uses existing endpoints.

### Processing Flow

```
API Request (Jira/Notion/Markdown/JSON)
    ↓
Content Handler validates and creates job
    ↓
StateMachineWorker picks up job
    ↓
ContentIngestProcessor.StartProcessing()
    ├─ Normalize content using Adapters (JiraAdapter, NotionAdapter, etc.)
    ├─ Send text to /chunk/text endpoint → Get chunks
    ├─ Send chunks to /embed/batch endpoint → Get embeddings
    └─ Store chunks + embeddings in ChromaDB
    ↓
Job completed, document updated
```

### New Components

#### 1. Content Models (`internal/models/content.go`)

Added data models for content ingestion:

```go
type ContentSourceType string
const (
    SourceTypeJira     ContentSourceType = "jira"
    SourceTypeNotion   ContentSourceType = "notion"
    SourceTypeMarkdown ContentSourceType = "markdown"
    SourceTypeJSON     ContentSourceType = "json"
)

type JiraIssuePayload struct
type NotionPagePayload struct
type ContentIngestRequest struct
type ContentIngestResponse struct
```

#### 2. Content Handler (`internal/handlers/content_handler.go`)

New HTTP handler exposing:

- `POST /api/v1/content/ingest` - Accepts JSON payloads for all content types
- `GET /api/v1/content/{id}/status` - Check processing status

#### 3. Content Adapters (`internal/services/adapters/content_adapter.go`)

Adapter pattern for normalizing different content types to plain text:

- **JiraAdapter** - Converts Jira issue JSON to readable text format
- **NotionAdapter** - Converts Notion page blocks to plain text
- **MarkdownAdapter** - Passes through Markdown content
- **JSONAdapter** - Formats JSON as readable text

#### 4. Content Ingest Processor (`internal/services/processors/content_ingest_processor.go`)

Unified processor that handles all content types:

1. Gets content payload from job
2. Uses appropriate adapter to normalize to text
3. Calls Python `/chunk/text` endpoint
4. Calls Python `/embed/batch` endpoint
5. Stores results in ChromaDB

#### 5. Updated BaseProcessor

Added `VectorRepo` field to enable direct vector storage.

### Modified Components

| File | Changes |
|------|---------|
| `models/Documents.go` | Added `SourceType`, `SourceID` fields |
| `models/job.go` | Added `JobTypeJiraIngest`, `JobTypeNotionIngest`, `JobTypeMarkdownIngest`, `JobTypeJSONIngest` |
| `repositories/job_repository.go` | Added new JobType constants |
| `services/processors/process_abstract.go` | Added `VectorRepo` field |
| `services/processors/processor_register.go` | Registered content processors with VectorRepo |
| `routes/routes.go` | Added `/api/v1/content/ingest` and `/api/v1/content/{id}/status` |
| `server/server.go` | Initialized ContentHandler, passed VectorRepo to processors |

## API Usage

### Ingest Jira Issue

```bash
curl -X POST http://localhost:8080/api/v1/content/ingest \
  -H "Content-Type: application/json" \
  -d '{
    "source_type": "jira",
    "collection": "my-collection",
    "jira": {
      "key": "PROJ-123",
      "summary": "Fix login bug",
      "description": "Users cannot login with SSO",
      "status": "Open",
      "priority": "High",
      "project": "Backend",
      "issue_type": "Bug",
      "reporter": "john.doe",
      "labels": ["backend", "security"]
    },
    "chunking_strategy": "semantic",
    "chunk_size": 512,
    "extract_metadata": true
  }'
```

### Ingest Notion Page

```bash
curl -X POST http://localhost:8080/api/v1/content/ingest \
  -H "Content-Type: application/json" \
  -d '{
    "source_type": "notion",
    "collection": "documentation",
    "notion": {
      "id": "abc123",
      "title": "API Documentation",
      "url": "https://notion.so/...",
      "created_by": "user@example.com",
      "content": [
        {"type": "heading_1", "rich_text": [{"text": "Introduction"}]},
        {"type": "paragraph", "rich_text": [{"text": "This document..."}]}
      ]
    },
    "extract_metadata": true
  }'
```

### Ingest Markdown

```bash
curl -X POST http://localhost:8080/api/v1/content/ingest \
  -H "Content-Type: application/json" \
  -d '{
    "source_type": "markdown",
    "collection": "docs",
    "markdown": "# My Document\n\nThis is the content...",
    "chunking_strategy": "recursive",
    "chunk_size": 1024
  }'
```

### Ingest JSON

```bash
curl -X POST http://localhost:8080/api/v1/content/ingest \
  -H "Content-Type: application/json" \
  -d '{
    "source_type": "json",
    "collection": "data",
    "json": {"title": "Data Record", "fields": ["field1", "field2"]}
  }'
```

## Python Backend (No Changes Required)

The implementation uses **existing Python endpoints**:

### `/chunk/text` (Existing)

```python
# Request
POST /chunk/text
{
    "text": "content to chunk",
    "strategy": "semantic",
    "chunk_size": 512,
    "chunk_overlap": 50,
    "extract_metadata": true,
    "num_questions": 3
}

# Response
{
    "chunks": [...],
    "total_chunks": 10,
    "strategy_used": "semantic",
    ...
}
```

### `/embed/batch` (Existing)

```python
# Request
POST /embed/batch
{
    "texts": ["chunk1", "chunk2", ...],
    "model": null,
    "batch_size": 32,
    "use_cache": false
}

# Response
{
    "embeddings": [[0.1, 0.2, ...], ...],
    "dimension": 768,
    "model": "text-embedding-3-small",
    "total_embeddings": 10
}
```

## Data Flow Details

### 1. Normalization (Go - Adapters)

Each adapter converts source-specific format to plain text:

**Jira Example:**
```
Input: {"key": "PROJ-123", "summary": "...", "description": "..."}
Output: "Issue: PROJ-123\nSummary: ...\nDescription: ...\nStatus: Open\n..."
```

**Notion Example:**
```
Input: {"title": "...", "content": [...blocks...]}
Output: "Title: ...\n\n# Introduction\nContent...\n## Section\nContent..."
```

### 2. Chunking (Python `/chunk/text`)

- Text is split according to strategy (sentence, semantic, recursive, etc.)
- Optional LLM metadata extraction (title, keywords, questions)
- Returns list of chunks with text and metadata

### 3. Embedding (Python `/embed/batch`)

- All chunk texts sent in batch
- Returns embeddings for each chunk

### 4. Storage (Go → ChromaDB)

- Chunks stored with embeddings in ChromaDB
- Metadata includes: document_id, chunk_index, source_type, source metadata, LLM-extracted fields

## Files Created

| File | Description |
|------|-------------|
| `internal/models/content.go` | Content source models and payload structures |
| `internal/handlers/content_handler.go` | Content ingestion HTTP handler |
| `internal/services/adapters/content_adapter.go` | Adapter pattern for content normalization |
| `internal/services/processors/content_ingest_processor.go` | Content processor using existing Python endpoints |

## Files Modified

| File | Changes |
|------|---------|
| `internal/models/Documents.go` | Added SourceType and SourceID fields |
| `internal/models/job.go` | Added new JobType constants |
| `internal/repositories/job_repository.go` | Added new JobType constants and validation |
| `internal/services/processors/process_abstract.go` | Added VectorRepo field |
| `internal/services/processors/processor_register.go` | Registered content processors |
| `internal/routes/routes.go` | Added content ingestion endpoints |
| `internal/server/server.go` | Initialized ContentHandler, passed VectorRepo |

## Advantages of This Approach

1. **No Python Changes** - Leverages existing, tested endpoints
2. **Consistent Processing** - Same chunking/embedding pipeline for all content types
3. **Extensible** - Adding new sources only requires a new adapter
4. **Async Support** - Uses existing job queue infrastructure
5. **Type Safety** - Go handles source-specific validation

## Future Enhancements

1. **Bulk Operations** - Support batch ingestion for multiple Jira issues/Notion pages
2. **Webhook Callbacks** - Add callback support for long-running processing
3. **Incremental Updates** - Support updating existing documents
4. **Content Deduplication** - Detect and handle duplicate content
5. **Rate Limiting** - Add rate limiting for API calls
