# Async Document Upload Flow

## Overview

Document upload processing (parsing, chunking, embedding) can take 30+ seconds for large documents. To provide a responsive user experience, we use an **asynchronous job queue** pattern.

## Architecture

```
Frontend                Go Backend              Redis Job Queue        Background Worker       Python Service
   |                         |                         |                       |                      |
   | POST /upload            |                         |                       |                      |
   |------------------------>|                         |                       |                      |
   |                         |                         |                       |                      |
   |                         | 1. Create Job           |                       |                      |
   |                         |------------------------>|                       |                      |
   |                         |                         |                       |                      |
   |                         | 2. Enqueue Job          |                       |                      |
   |                         |------------------------>|                       |                      |
   |                         |                         |                       |                      |
   | 3. Return Job ID        |                         |                       |                      |
   |<------------------------|                         |                       |                      |
   |                         |                         |                       |                      |
   | GET /jobs/{id}/status   |                         |                       |                      |
   |------------------------>|                         |                       |                      |
   |                         | Get Job Status          |                       |                      |
   |                         |------------------------>|                       |                      |
   |                         |                         |                       |                      |
   | Return Progress         |                         |                       |                      |
   |<------------------------|                         |                       |                      |
   |                         |                         |                       |                      |
   |                         |                         | Dequeue Job           |                      |
   |                         |                         |<----------------------|                      |
   |                         |                         |                       |                      |
   |                         |                         |                       | Parse Document       |
   |                         |                         |                       |--------------------->|
   |                         |                         |                       |                      |
   |                         |                         | Update: 30%           |                      |
   |                         |                         |<----------------------|                      |
   |                         |                         |                       |                      |
   |                         |                         |                       | Chunk Text           |
   |                         |                         |                       |--------------------->|
   |                         |                         |                       |                      |
   |                         |                         | Update: 60%           |                      |
   |                         |                         |<----------------------|                      |
   |                         |                         |                       |                      |
   |                         |                         |                       | Generate Embeddings  |
   |                         |                         |                       |--------------------->|
   |                         |                         |                       |                      |
   |                         |                         | Update: 80%           |                      |
   |                         |                         |<----------------------|                      |
   |                         |                         |                       |                      |
   |                         |                         |                       | Store in ChromaDB    |
   |                         |                         |                       |                      |
   |                         |                         | Update: Completed     |                      |
   |                         |                         |<----------------------|                      |
   |                         |                         |                       |                      |
   | GET /jobs/{id}/status   |                         |                       |                      |
   |------------------------>|                         |                       |                      |
   |                         | Get Job Result          |                       |                      |
   |                         |------------------------>|                       |                      |
   |                         |                         |                       |                      |
   | Return Document ID      |                         |                       |                      |
   |<------------------------|                         |                       |                      |
```

## Flow Details

### 1. Upload Request (Immediate Response)

**Frontend → Go Backend:**
```http
POST /api/documents/upload
Content-Type: multipart/form-data

file: [binary data]
collection: "my-collection"
chunk_size: 512
```

**Go Backend Response (< 100ms):**
```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "queued",
  "message": "Upload queued for processing",
  "created_at": "2024-01-05T10:30:00Z"
}
```

### 2. Job Storage (Redis)

**Job Object in Redis:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "document_upload",
  "status": "queued",
  "progress": 0,
  "message": "Waiting for worker...",
  "payload": {
    "filename": "document.pdf",
    "file_size": 1048576,
    "collection": "my-collection",
    "chunking_strategy": "sentence",
    "chunk_size": 512,
    "chunk_overlap": 50
  },
  "created_at": "2024-01-05T10:30:00Z",
  "updated_at": "2024-01-05T10:30:00Z"
}
```

**Redis Keys:**
```
job:550e8400-e29b-41d4-a716-446655440000        # Job hash
queue:document_upload                            # Job queue (list)
jobs:active                                      # Set of active job IDs
```

### 3. Status Polling (Frontend)

**Frontend polls every 2 seconds:**
```http
GET /api/jobs/550e8400-e29b-41d4-a716-446655440000
```

**Response - Queued:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "document_upload",
  "status": "queued",
  "progress": 0,
  "message": "Waiting for worker..."
}
```

**Response - Processing (30%):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "document_upload",
  "status": "processing",
  "progress": 30,
  "message": "Chunking text...",
  "started_at": "2024-01-05T10:30:05Z"
}
```

**Response - Completed:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "document_upload",
  "status": "completed",
  "progress": 100,
  "message": "Upload completed successfully",
  "result": {
    "document_id": "abc123",
    "chunk_count": 42,
    "collection": "my-collection",
    "processing_time_ms": 28340
  },
  "started_at": "2024-01-05T10:30:05Z",
  "completed_at": "2024-01-05T10:30:33Z"
}
```

### 4. Background Worker Processing

**Worker Loop:**
```go
func (w *UploadWorker) Run(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        default:
            // Dequeue job
            job, err := w.jobRepo.DequeueJob(ctx, JobTypeDocumentUpload)
            if err != nil {
                time.Sleep(1 * time.Second)
                continue
            }
            
            // Process job
            if err := w.ProcessJob(ctx, job); err != nil {
                w.handleError(ctx, job, err)
            }
        }
    }
}
```

**Processing Steps with Progress Updates:**

1. **Parse (0% → 30%)**
   ```go
   w.jobRepo.SetProgress(ctx, job.ID, 10, "Parsing document...")
   parsed, err := w.pythonClient.Parse(ctx, fileBytes, filename)
   w.jobRepo.SetProgress(ctx, job.ID, 30, "Document parsed")
   ```

2. **Chunk (30% → 60%)**
   ```go
   w.jobRepo.SetProgress(ctx, job.ID, 30, "Chunking text...")
   chunks, err := w.pythonClient.Chunk(ctx, parsed.Text, opts)
   w.jobRepo.SetProgress(ctx, job.ID, 60, fmt.Sprintf("Created %d chunks", len(chunks)))
   ```

3. **Embed (60% → 80%)**
   ```go
   w.jobRepo.SetProgress(ctx, job.ID, 60, "Generating embeddings...")
   embeddings, err := w.pythonClient.Embed(ctx, texts)
   w.jobRepo.SetProgress(ctx, job.ID, 80, "Embeddings generated")
   ```

4. **Store (80% → 100%)**
   ```go
   w.jobRepo.SetProgress(ctx, job.ID, 80, "Storing in vector DB...")
   err := w.vectorRepo.StoreChunks(ctx, collection, chunks)
   err = w.docRepo.Register(ctx, document)
   w.jobRepo.UpdateJobStatus(ctx, job.ID, JobStatusCompleted, 100, "Upload completed")
   ```

### 5. Error Handling

**On Failure:**
```go
if err != nil {
    if job.CanRetry() {
        // Retry logic
        job.RetryCount++
        w.jobRepo.UpdateJobStatus(ctx, job.ID, JobStatusRetrying, 0, 
            fmt.Sprintf("Retrying (%d/%d): %v", job.RetryCount, job.MaxRetries, err))
        w.jobRepo.EnqueueJob(ctx, job)
    } else {
        // Max retries exceeded
        w.jobRepo.UpdateJobStatus(ctx, job.ID, JobStatusFailed, 0, err.Error())
    }
}
```

**Failed Job Response:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "document_upload",
  "status": "failed",
  "progress": 30,
  "message": "Chunking failed",
  "error": "Python service timeout after 30s",
  "retry_count": 3,
  "max_retries": 3
}
```

## Implementation Files

### Redis Data Structures

**Job Hash:**
```redis
HSET job:550e8400... id "550e8400..." type "document_upload" status "queued" ...
```

**Job Queue (List):**
```redis
LPUSH queue:document_upload "550e8400-e29b-41d4-a716-446655440000"
RPOP queue:document_upload  # Worker dequeues
```

**Active Jobs Set:**
```redis
SADD jobs:active "550e8400-e29b-41d4-a716-446655440000"
SREM jobs:active "550e8400-e29b-41d4-a716-446655440000"  # On completion
```

## Benefits

✅ **Responsive UI**: Instant response (< 100ms) instead of 30+ seconds  
✅ **Progress Tracking**: Real-time progress updates (0% → 100%)  
✅ **Scalability**: Multiple workers can process jobs in parallel  
✅ **Reliability**: Jobs persist in Redis, survive server restarts  
✅ **Retry Logic**: Automatic retry on transient failures  
✅ **Monitoring**: Track job queue depth, success rate, processing time  

## Frontend Implementation

```typescript
// Upload file
async function uploadDocument(file: File, options: UploadOptions) {
  const formData = new FormData();
  formData.append('file', file);
  formData.append('collection', options.collection);
  
  const response = await fetch('/api/documents/upload', {
    method: 'POST',
    body: formData,
  });
  
  const { job_id } = await response.json();
  
  // Poll for status
  return pollJobStatus(job_id);
}

// Poll job status
async function pollJobStatus(jobId: string): Promise<UploadResult> {
  return new Promise((resolve, reject) => {
    const interval = setInterval(async () => {
      const status = await fetch(`/api/jobs/${jobId}`).then(r => r.json());
      
      // Update progress bar
      updateProgress(status.progress, status.message);
      
      if (status.status === 'completed') {
        clearInterval(interval);
        resolve(status.result);
      } else if (status.status === 'failed') {
        clearInterval(interval);
        reject(new Error(status.error));
      }
    }, 2000); // Poll every 2 seconds
  });
}
```

## Next Steps

1. **Task 1.5**: Implement Redis Document Repository
2. **Task 1.6**: Implement Redis Job Repository  
3. **Task 1.7**: Create Background Worker Pattern
4. **Task 3.2**: Implement Document Service with async upload

See `BACKEND_REFACTOR_PLAN.md` for complete implementation plan.