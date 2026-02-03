# Workers Package

This package provides a background worker system for processing asynchronous jobs in the risk analyzer application.

## Overview

The workers package implements a robust background job processing system with the following features:

- **Worker Pool Management**: Manage multiple workers with different responsibilities
- **Graceful Shutdown**: Proper cleanup and shutdown handling
- **Job Queuing**: Priority-based job queuing with Redis backend
- **Retry Logic**: Automatic retry with exponential backoff for failed jobs
- **Panic Recovery**: Automatic recovery from panics during job processing
- **Statistics Tracking**: Real-time statistics for monitoring worker performance
- **Concurrent Processing**: Multiple goroutines processing jobs in parallel

## Architecture

### Core Components

1. **Worker Interface**: Defines the contract for all workers
2. **BaseWorker**: Common functionality shared by all workers
3. **WorkerPool**: Manages multiple worker instances
4. **UploadWorker**: Specific implementation for document upload processing

## Worker Interface

```go
type Worker interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Name() string
    IsRunning() bool
    Stats() WorkerStats
}
```

## Usage

### Creating a Worker

```go
// Configure the worker
config := UploadWorkerConfig{
    WorkerConfig: WorkerConfig{
        WorkerName:      "upload-worker-1",
        Concurrency:     3,              // Process 3 jobs concurrently
        PollInterval:    2 * time.Second, // Check for new jobs every 2s
        ShutdownTimeout: 30 * time.Second,
        MaxRetries:      3,
        RetryDelay:      5 * time.Second,
        EnableRecovery:  true,
    },
    JobRepo:      jobRepository,
    DocumentRepo: documentRepository,
    VectorRepo:   vectorRepository,
    PythonClient: pythonClient,
    Logger:       logger,
}

// Create the worker
worker := NewUploadWorker(config)

// Start processing
ctx := context.Background()
if err := worker.Start(ctx); err != nil {
    log.Fatal(err)
}

// Gracefully stop when done
defer worker.Stop(ctx)
```

### Using a Worker Pool

```go
// Create a pool
pool := NewWorkerPool()

// Add workers
pool.AddWorker(uploadWorker1)
pool.AddWorker(uploadWorker2)
pool.AddWorker(uploadWorker3)

// Start all workers
ctx := context.Background()
if err := pool.StartAll(ctx); err != nil {
    log.Fatal(err)
}

// Get statistics for all workers
stats := pool.GetAllStats()
for _, stat := range stats {
    fmt.Printf("Worker: %s, Jobs: %d, Success Rate: %.2f%%\n", 
        stat.WorkerName, 
        stat.JobsProcessed,
        float64(stat.JobsSucceeded) / float64(stat.JobsProcessed) * 100)
}

// Gracefully stop all workers
defer pool.StopAll(ctx)
```

## Upload Worker

The `UploadWorker` processes document upload jobs through the following pipeline:

### Processing Pipeline

1. **Document Registration**: Register document metadata in Redis
2. **Document Parsing**: Call Python backend to parse the document
3. **Text Chunking**: Split document text into manageable chunks
4. **Embedding Generation**: Generate vector embeddings for each chunk
5. **Vector Storage**: Store chunks and embeddings in ChromaDB
6. **Finalization**: Update document status to completed

### Job Payload

Upload jobs expect the following payload:

```go
type UploadJobPayload struct {
    Filename         string `json:"filename"`
    FileSize         int64  `json:"file_size"`
    Collection       string `json:"collection"`
    ChunkingStrategy string `json:"chunking_strategy"`
    ChunkSize        int    `json:"chunk_size"`
    ChunkOverlap     int    `json:"chunk_overlap"`
    ExtractMetadata  bool   `json:"extract_metadata"`
    NumQuestions     int    `json:"num_questions"`
    MaxPages         int    `json:"max_pages"`
}
```

### Progress Tracking

The upload worker reports progress at each stage:

- 10% - Parsing document
- 30% - Chunking text
- 50% - Generating embeddings
- 70% - Storing in vector database
- 90% - Finalizing document
- 100% - Upload completed

### Error Handling

The worker implements robust error handling:

- **Retry Logic**: Failed jobs are automatically retried up to `MaxRetries` times
- **Exponential Backoff**: Delays between retries prevent overwhelming services
- **Panic Recovery**: Panics are caught and converted to errors
- **Document Status Updates**: Failed jobs update document status to `failed`

## Worker Statistics

Each worker tracks the following statistics:

```go
type WorkerStats struct {
    WorkerName         string        `json:"worker_name"`
    JobsProcessed      int64         `json:"jobs_processed"`
    JobsSucceeded      int64         `json:"jobs_succeeded"`
    JobsFailed         int64         `json:"jobs_failed"`
    AverageProcessTime time.Duration `json:"average_process_time"`
    LastJobTime        time.Time     `json:"last_job_time,omitempty"`
    Uptime             time.Duration `json:"uptime"`
    IsRunning          bool          `json:"is_running"`
}
```

Access statistics via:

```go
stats := worker.Stats()
fmt.Printf("Success Rate: %.2f%%\n", 
    float64(stats.JobsSucceeded) / float64(stats.JobsProcessed) * 100)
```

## Configuration

### Worker Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `WorkerName` | string | (required) | Unique identifier for the worker |
| `Concurrency` | int | 3 | Number of concurrent job processors |
| `PollInterval` | time.Duration | 2s | How often to check for new jobs |
| `ShutdownTimeout` | time.Duration | 30s | Max time to wait for graceful shutdown |
| `MaxRetries` | int | 3 | Maximum retry attempts for failed jobs |
| `RetryDelay` | time.Duration | 5s | Delay between retry attempts |
| `EnableRecovery` | bool | true | Enable panic recovery |

### Tuning Concurrency

Choose concurrency based on:

- **CPU-bound tasks**: Set to number of CPU cores
- **I/O-bound tasks**: Can be higher (2-4x CPU cores)
- **Memory constraints**: Lower concurrency for large documents
- **External service limits**: Respect rate limits

Example:

```go
config := DefaultWorkerConfig("upload-worker")

// For CPU-intensive processing
config.Concurrency = runtime.NumCPU()

// For I/O-bound operations
config.Concurrency = runtime.NumCPU() * 2

// For rate-limited APIs
config.Concurrency = 1
config.PollInterval = 1 * time.Second
```

## Testing

The package includes comprehensive tests:

```bash
# Run all worker tests
go test -v ./backend/internal/workers/

# Run specific test
go test -v ./backend/internal/workers/ -run TestUploadWorker_ProcessJob_Success

# Run with coverage
go test -v -cover ./backend/internal/workers/

# Run with race detection
go test -v -race ./backend/internal/workers/
```

### Test Coverage

- Unit tests for all worker components
- Integration tests with mock dependencies
- Concurrent access tests
- Error handling and retry logic tests
- Panic recovery tests

## Monitoring

### Health Checks

```go
// Check if worker is running
if !worker.IsRunning() {
    log.Warn("Worker stopped unexpectedly")
}

// Get current stats
stats := worker.Stats()
if stats.JobsFailed > stats.JobsSucceeded {
    log.Error("Worker has high failure rate")
}
```

### Logging

Workers log important events:

- Worker start/stop
- Job processing start/completion
- Errors and retries
- Panic recovery

Configure logging level based on environment:

```go
// Production: Info and above
logger := &ProductionLogger{Level: INFO}

// Development: Debug and above  
logger := &DevelopmentLogger{Level: DEBUG}
```

## Best Practices

### 1. Graceful Shutdown

Always use context for proper shutdown:

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

// Handle shutdown signals
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

go func() {
    <-sigChan
    log.Info("Shutdown signal received")
    cancel()
}()

worker.Start(ctx)
```

### 2. Resource Cleanup

Ensure resources are cleaned up:

```go
// Close repositories
defer jobRepo.Close()
defer documentRepo.Close()
defer vectorRepo.Close()

// Stop workers
defer pool.StopAll(ctx)
```

### 3. Error Handling

Handle errors appropriately:

```go
if err := worker.Start(ctx); err != nil {
    log.Error("Failed to start worker: %v", err)
    // Implement retry or alerting logic
}
```

### 4. Monitoring

Monitor worker health:

```go
ticker := time.NewTicker(1 * time.Minute)
go func() {
    for range ticker.C {
        stats := pool.GetAllStats()
        // Send metrics to monitoring system
        sendMetrics(stats)
    }
}()
```

## Troubleshooting

### Worker Not Processing Jobs

1. Check if worker is running: `worker.IsRunning()`
2. Verify jobs are being enqueued: Check job repository
3. Check poll interval: May be too long
4. Review logs for errors

### High Failure Rate

1. Review error logs
2. Check external service availability (Python backend, ChromaDB, Redis)
3. Verify job payloads are valid
4. Consider increasing retry delay
5. Check resource constraints (memory, CPU)

### Memory Issues

1. Reduce concurrency
2. Implement batch size limits
3. Add memory profiling
4. Check for resource leaks in job processing

### Slow Processing

1. Increase concurrency (if not CPU-bound)
2. Profile job processing pipeline
3. Optimize external service calls
4. Consider caching frequently accessed data
5. Check database connection pool settings

## Future Enhancements

Potential improvements:

- [ ] Priority queue support (already implemented in job repository)
- [ ] Dynamic concurrency adjustment based on load
- [ ] Circuit breaker pattern for external services
- [ ] Job scheduling with cron-like syntax
- [ ] Distributed worker coordination
- [ ] Dead letter queue for permanently failed jobs
- [ ] Metrics export to Prometheus
- [ ] Worker auto-scaling based on queue depth

## Related Packages

- `repositories`: Data access layer for jobs, documents, and vectors
- `db`: Database connection management
- `models`: Domain models

## License

See project LICENSE file.