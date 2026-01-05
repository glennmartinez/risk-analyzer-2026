package workers

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"risk-analyzer/internal/repositories"
)

// Mock implementations

type MockJobRepository struct {
	mock.Mock
}

func (m *MockJobRepository) CreateJob(ctx context.Context, job *repositories.Job) error {
	args := m.Called(ctx, job)
	return args.Error(0)
}

func (m *MockJobRepository) GetJob(ctx context.Context, jobID string) (*repositories.Job, error) {
	args := m.Called(ctx, jobID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repositories.Job), args.Error(1)
}

func (m *MockJobRepository) UpdateJobStatus(ctx context.Context, jobID string, status repositories.JobStatus, progress int, message string) error {
	args := m.Called(ctx, jobID, status, progress, message)
	return args.Error(0)
}

func (m *MockJobRepository) UpdateJobResult(ctx context.Context, jobID string, result map[string]interface{}) error {
	args := m.Called(ctx, jobID, result)
	return args.Error(0)
}

func (m *MockJobRepository) DeleteJob(ctx context.Context, jobID string) error {
	args := m.Called(ctx, jobID)
	return args.Error(0)
}

func (m *MockJobRepository) ListJobs(ctx context.Context, filter *repositories.JobFilter) ([]*repositories.Job, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repositories.Job), args.Error(1)
}

func (m *MockJobRepository) ListJobsByStatus(ctx context.Context, status repositories.JobStatus) ([]*repositories.Job, error) {
	args := m.Called(ctx, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repositories.Job), args.Error(1)
}

func (m *MockJobRepository) ListJobsByType(ctx context.Context, jobType repositories.JobType) ([]*repositories.Job, error) {
	args := m.Called(ctx, jobType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repositories.Job), args.Error(1)
}

func (m *MockJobRepository) GetActiveJobs(ctx context.Context) ([]*repositories.Job, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repositories.Job), args.Error(1)
}

func (m *MockJobRepository) EnqueueJob(ctx context.Context, job *repositories.Job) error {
	args := m.Called(ctx, job)
	return args.Error(0)
}

func (m *MockJobRepository) DequeueJob(ctx context.Context, jobType repositories.JobType) (*repositories.Job, error) {
	args := m.Called(ctx, jobType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repositories.Job), args.Error(1)
}

func (m *MockJobRepository) RequeueFailedJobs(ctx context.Context, maxRetries int) (int, error) {
	args := m.Called(ctx, maxRetries)
	return args.Int(0), args.Error(1)
}

func (m *MockJobRepository) SetProgress(ctx context.Context, jobID string, progress int, message string) error {
	args := m.Called(ctx, jobID, progress, message)
	return args.Error(0)
}

func (m *MockJobRepository) GetProgress(ctx context.Context, jobID string) (*repositories.JobProgress, error) {
	args := m.Called(ctx, jobID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repositories.JobProgress), args.Error(1)
}

func (m *MockJobRepository) CleanupCompletedJobs(ctx context.Context, olderThan time.Duration) (int, error) {
	args := m.Called(ctx, olderThan)
	return args.Int(0), args.Error(1)
}

func (m *MockJobRepository) CleanupFailedJobs(ctx context.Context, olderThan time.Duration, maxRetries int) (int, error) {
	args := m.Called(ctx, olderThan, maxRetries)
	return args.Int(0), args.Error(1)
}

func (m *MockJobRepository) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockJobRepository) Close() error {
	args := m.Called()
	return args.Error(0)
}

type MockDocumentRepository struct {
	mock.Mock
}

func (m *MockDocumentRepository) Register(ctx context.Context, doc *repositories.Document) error {
	args := m.Called(ctx, doc)
	return args.Error(0)
}

func (m *MockDocumentRepository) Get(ctx context.Context, documentID string) (*repositories.Document, error) {
	args := m.Called(ctx, documentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repositories.Document), args.Error(1)
}

func (m *MockDocumentRepository) List(ctx context.Context) ([]*repositories.Document, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repositories.Document), args.Error(1)
}

func (m *MockDocumentRepository) Delete(ctx context.Context, documentID string) error {
	args := m.Called(ctx, documentID)
	return args.Error(0)
}

func (m *MockDocumentRepository) Update(ctx context.Context, documentID string, updates map[string]interface{}) error {
	args := m.Called(ctx, documentID, updates)
	return args.Error(0)
}

func (m *MockDocumentRepository) Exists(ctx context.Context, documentID string) (bool, error) {
	args := m.Called(ctx, documentID)
	return args.Bool(0), args.Error(1)
}

func (m *MockDocumentRepository) RegisterBatch(ctx context.Context, docs []*repositories.Document) error {
	args := m.Called(ctx, docs)
	return args.Error(0)
}

func (m *MockDocumentRepository) GetBatch(ctx context.Context, documentIDs []string) ([]*repositories.Document, error) {
	args := m.Called(ctx, documentIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repositories.Document), args.Error(1)
}

func (m *MockDocumentRepository) DeleteBatch(ctx context.Context, documentIDs []string) error {
	args := m.Called(ctx, documentIDs)
	return args.Error(0)
}

func (m *MockDocumentRepository) ListByCollection(ctx context.Context, collection string) ([]*repositories.Document, error) {
	args := m.Called(ctx, collection)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repositories.Document), args.Error(1)
}

func (m *MockDocumentRepository) ListByStatus(ctx context.Context, status repositories.DocumentStatus) ([]*repositories.Document, error) {
	args := m.Called(ctx, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repositories.Document), args.Error(1)
}

func (m *MockDocumentRepository) CountByCollection(ctx context.Context, collection string) (int, error) {
	args := m.Called(ctx, collection)
	return args.Int(0), args.Error(1)
}

func (m *MockDocumentRepository) CountTotal(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func (m *MockDocumentRepository) FindByFilename(ctx context.Context, filename string) (*repositories.Document, error) {
	args := m.Called(ctx, filename)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repositories.Document), args.Error(1)
}

func (m *MockDocumentRepository) FilterByMetadata(ctx context.Context, filter map[string]interface{}) ([]*repositories.Document, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repositories.Document), args.Error(1)
}

func (m *MockDocumentRepository) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockDocumentRepository) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockDocumentRepository) Cleanup(ctx context.Context, olderThan time.Duration) (int, error) {
	args := m.Called(ctx, olderThan)
	return args.Int(0), args.Error(1)
}

type MockVectorRepository struct {
	mock.Mock
}

func (m *MockVectorRepository) CreateCollection(ctx context.Context, name string, metadata map[string]interface{}) error {
	args := m.Called(ctx, name, metadata)
	return args.Error(0)
}

func (m *MockVectorRepository) DeleteCollection(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func (m *MockVectorRepository) GetCollection(ctx context.Context, name string) (*repositories.CollectionInfo, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repositories.CollectionInfo), args.Error(1)
}

func (m *MockVectorRepository) ListCollections(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockVectorRepository) GetCollectionStats(ctx context.Context, name string) (*repositories.CollectionStats, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repositories.CollectionStats), args.Error(1)
}

func (m *MockVectorRepository) CollectionExists(ctx context.Context, name string) (bool, error) {
	args := m.Called(ctx, name)
	return args.Bool(0), args.Error(1)
}

func (m *MockVectorRepository) StoreChunks(ctx context.Context, collection string, chunks []*repositories.Chunk) error {
	args := m.Called(ctx, collection, chunks)
	return args.Error(0)
}

func (m *MockVectorRepository) SearchChunks(ctx context.Context, collection string, query []float32, limit int, filter map[string]interface{}) ([]*repositories.SearchResult, error) {
	args := m.Called(ctx, collection, query, limit, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repositories.SearchResult), args.Error(1)
}

func (m *MockVectorRepository) DeleteDocument(ctx context.Context, collection string, documentID string) (int, error) {
	args := m.Called(ctx, collection, documentID)
	return args.Int(0), args.Error(1)
}

func (m *MockVectorRepository) DeleteChunks(ctx context.Context, collection string, chunkIDs []string) error {
	args := m.Called(ctx, collection, chunkIDs)
	return args.Error(0)
}

func (m *MockVectorRepository) GetChunk(ctx context.Context, collection string, chunkID string) (*repositories.Chunk, error) {
	args := m.Called(ctx, collection, chunkID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repositories.Chunk), args.Error(1)
}

func (m *MockVectorRepository) ListDocuments(ctx context.Context, collection string) ([]*repositories.VectorDocument, error) {
	args := m.Called(ctx, collection)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repositories.VectorDocument), args.Error(1)
}

func (m *MockVectorRepository) CountDocuments(ctx context.Context, collection string) (int, error) {
	args := m.Called(ctx, collection)
	return args.Int(0), args.Error(1)
}

func (m *MockVectorRepository) BatchStoreChunks(ctx context.Context, collection string, batches [][]*repositories.Chunk) error {
	args := m.Called(ctx, collection, batches)
	return args.Error(0)
}

func (m *MockVectorRepository) BatchDeleteChunks(ctx context.Context, collection string, chunkIDs []string) error {
	args := m.Called(ctx, collection, chunkIDs)
	return args.Error(0)
}

func (m *MockVectorRepository) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockVectorRepository) Close() error {
	args := m.Called()
	return args.Error(0)
}

type MockPythonClient struct {
	mock.Mock
}

func (m *MockPythonClient) ParseDocument(ctx context.Context, filename string, extractMetadata bool, numQuestions int, maxPages int) (ParseResult, error) {
	args := m.Called(ctx, filename, extractMetadata, numQuestions, maxPages)
	return args.Get(0).(ParseResult), args.Error(1)
}

func (m *MockPythonClient) ChunkText(ctx context.Context, text string, strategy string, chunkSize int, chunkOverlap int) (ChunkResult, error) {
	args := m.Called(ctx, text, strategy, chunkSize, chunkOverlap)
	return args.Get(0).(ChunkResult), args.Error(1)
}

func (m *MockPythonClient) GenerateEmbeddings(ctx context.Context, texts []string) (EmbeddingResult, error) {
	args := m.Called(ctx, texts)
	return args.Get(0).(EmbeddingResult), args.Error(1)
}

type MockLogger struct {
	Logs []string
}

func (l *MockLogger) Info(msg string, args ...interface{}) {
	l.Logs = append(l.Logs, fmt.Sprintf("[INFO] "+msg, args...))
}

func (l *MockLogger) Error(msg string, args ...interface{}) {
	l.Logs = append(l.Logs, fmt.Sprintf("[ERROR] "+msg, args...))
}

func (l *MockLogger) Warn(msg string, args ...interface{}) {
	l.Logs = append(l.Logs, fmt.Sprintf("[WARN] "+msg, args...))
}

func (l *MockLogger) Debug(msg string, args ...interface{}) {
	l.Logs = append(l.Logs, fmt.Sprintf("[DEBUG] "+msg, args...))
}

// Test functions

func TestNewUploadWorker(t *testing.T) {
	config := UploadWorkerConfig{
		WorkerConfig: DefaultWorkerConfig("test-upload-worker"),
		JobRepo:      &MockJobRepository{},
		DocumentRepo: &MockDocumentRepository{},
		VectorRepo:   &MockVectorRepository{},
		PythonClient: &MockPythonClient{},
		Logger:       &MockLogger{},
	}

	worker := NewUploadWorker(config)
	assert.NotNil(t, worker)
	assert.Equal(t, "test-upload-worker", worker.Name())
	assert.False(t, worker.IsRunning())
}

func TestUploadWorker_StartStop(t *testing.T) {
	jobRepo := &MockJobRepository{}
	logger := &MockLogger{}

	config := UploadWorkerConfig{
		WorkerConfig: WorkerConfig{
			WorkerName:      "test-worker",
			Concurrency:     1,
			PollInterval:    100 * time.Millisecond,
			ShutdownTimeout: 1 * time.Second,
		},
		JobRepo:      jobRepo,
		DocumentRepo: &MockDocumentRepository{},
		VectorRepo:   &MockVectorRepository{},
		PythonClient: &MockPythonClient{},
		Logger:       logger,
	}

	worker := NewUploadWorker(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Mock DequeueJob to return nil (no jobs)
	jobRepo.On("DequeueJob", mock.Anything, repositories.JobTypeDocumentUpload).Return(nil, nil)

	// Start worker
	err := worker.Start(ctx)
	require.NoError(t, err)
	assert.True(t, worker.IsRunning())

	// Give it time to start
	time.Sleep(200 * time.Millisecond)

	// Stop worker
	cancel()
	err = worker.Stop(ctx)
	require.NoError(t, err)
	assert.False(t, worker.IsRunning())
}

func TestUploadWorker_ProcessJob_Success(t *testing.T) {
	jobRepo := &MockJobRepository{}
	docRepo := &MockDocumentRepository{}
	vectorRepo := &MockVectorRepository{}
	pythonClient := &MockPythonClient{}
	logger := &MockLogger{}

	config := UploadWorkerConfig{
		WorkerConfig: DefaultWorkerConfig("test-worker"),
		JobRepo:      jobRepo,
		DocumentRepo: docRepo,
		VectorRepo:   vectorRepo,
		PythonClient: pythonClient,
		Logger:       logger,
	}

	worker := NewUploadWorker(config)

	// Create test job
	job := &repositories.Job{
		ID:         "job-123",
		Type:       repositories.JobTypeDocumentUpload,
		Status:     repositories.JobStatusProcessing,
		MaxRetries: 3,
		Payload: map[string]interface{}{
			"filename":          "test.pdf",
			"file_size":         float64(1024),
			"collection":        "test-collection",
			"chunking_strategy": "fixed",
			"chunk_size":        float64(512),
			"chunk_overlap":     float64(50),
			"extract_metadata":  true,
			"num_questions":     float64(5),
			"max_pages":         float64(10),
		},
	}

	ctx := context.Background()

	// Mock document registration
	docRepo.On("Register", mock.Anything, mock.MatchedBy(func(doc *repositories.Document) bool {
		return doc.Filename == "test.pdf" && doc.Collection == "test-collection"
	})).Return(nil)

	// Mock progress updates
	jobRepo.On("SetProgress", mock.Anything, "job-123", mock.Anything, mock.Anything).Return(nil)

	// Mock Python client calls
	pythonClient.On("ParseDocument", mock.Anything, "test.pdf", true, 5, 10).Return(
		ParseResult{
			Text:     "This is test document text",
			Metadata: map[string]interface{}{"pages": 5},
		},
		nil,
	)

	pythonClient.On("ChunkText", mock.Anything, "This is test document text", "fixed", 512, 50).Return(
		ChunkResult{
			Chunks: []string{"chunk1", "chunk2", "chunk3"},
		},
		nil,
	)

	pythonClient.On("GenerateEmbeddings", mock.Anything, []string{"chunk1", "chunk2", "chunk3"}).Return(
		EmbeddingResult{
			Embeddings: [][]float32{
				{0.1, 0.2, 0.3},
				{0.4, 0.5, 0.6},
				{0.7, 0.8, 0.9},
			},
		},
		nil,
	)

	// Mock vector storage
	vectorRepo.On("StoreChunks", mock.Anything, "test-collection", mock.MatchedBy(func(chunks []*repositories.Chunk) bool {
		return len(chunks) == 3
	})).Return(nil)

	// Mock document update
	docRepo.On("Update", mock.Anything, mock.Anything, mock.MatchedBy(func(updates map[string]interface{}) bool {
		return updates["chunk_count"] == 3 && updates["status"] == repositories.DocumentStatusCompleted
	})).Return(nil)

	// Mock job result update
	jobRepo.On("UpdateJobResult", mock.Anything, "job-123", mock.Anything).Return(nil)

	// Mock job status update
	jobRepo.On("UpdateJobStatus", mock.Anything, "job-123", repositories.JobStatusCompleted, 100, "Upload completed successfully").Return(nil)

	// Process job
	worker.processJob(ctx, job)

	// Verify all mocks were called
	docRepo.AssertExpectations(t)
	vectorRepo.AssertExpectations(t)
	pythonClient.AssertExpectations(t)
	jobRepo.AssertExpectations(t)

	// Verify stats
	stats := worker.Stats()
	assert.Equal(t, int64(1), stats.JobsProcessed)
	assert.Equal(t, int64(1), stats.JobsSucceeded)
	assert.Equal(t, int64(0), stats.JobsFailed)
}

func TestUploadWorker_ProcessJob_Failure(t *testing.T) {
	jobRepo := &MockJobRepository{}
	docRepo := &MockDocumentRepository{}
	vectorRepo := &MockVectorRepository{}
	pythonClient := &MockPythonClient{}
	logger := &MockLogger{}

	config := UploadWorkerConfig{
		WorkerConfig: WorkerConfig{
			WorkerName:  "test-worker",
			MaxRetries:  3,
			RetryDelay:  10 * time.Millisecond,
			Concurrency: 1,
		},
		JobRepo:      jobRepo,
		DocumentRepo: docRepo,
		VectorRepo:   vectorRepo,
		PythonClient: pythonClient,
		Logger:       logger,
	}

	worker := NewUploadWorker(config)

	// Create test job
	job := &repositories.Job{
		ID:         "job-failed",
		Type:       repositories.JobTypeDocumentUpload,
		Status:     repositories.JobStatusProcessing,
		RetryCount: 0,
		MaxRetries: 3,
		Payload: map[string]interface{}{
			"filename":          "test.pdf",
			"file_size":         float64(1024),
			"collection":        "test-collection",
			"chunking_strategy": "fixed",
			"chunk_size":        float64(512),
			"chunk_overlap":     float64(50),
		},
	}

	ctx := context.Background()

	// Mock document registration to succeed
	docRepo.On("Register", mock.Anything, mock.Anything).Return(nil)

	// Mock progress update
	jobRepo.On("SetProgress", mock.Anything, "job-failed", mock.Anything, mock.Anything).Return(nil)

	// Mock Python client to fail
	pythonClient.On("ParseDocument", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		ParseResult{},
		errors.New("parse failed"),
	)

	// Mock retry status update
	jobRepo.On("UpdateJobStatus", mock.Anything, "job-failed", repositories.JobStatusRetrying, 0, mock.Anything).Return(nil)

	// Mock re-enqueue
	jobRepo.On("EnqueueJob", mock.Anything, mock.Anything).Return(nil)

	// Process job
	worker.processJob(ctx, job)

	// Verify stats
	stats := worker.Stats()
	assert.Equal(t, int64(1), stats.JobsProcessed)
	assert.Equal(t, int64(0), stats.JobsSucceeded)
	assert.Equal(t, int64(1), stats.JobsFailed)

	// Verify retry was attempted
	jobRepo.AssertCalled(t, "UpdateJobStatus", mock.Anything, "job-failed", repositories.JobStatusRetrying, 0, mock.Anything)
	jobRepo.AssertCalled(t, "EnqueueJob", mock.Anything, mock.Anything)
}

func TestUploadWorker_ProcessJob_MaxRetriesExceeded(t *testing.T) {
	jobRepo := &MockJobRepository{}
	docRepo := &MockDocumentRepository{}
	vectorRepo := &MockVectorRepository{}
	pythonClient := &MockPythonClient{}
	logger := &MockLogger{}

	config := UploadWorkerConfig{
		WorkerConfig: WorkerConfig{
			WorkerName:  "test-worker",
			MaxRetries:  3,
			Concurrency: 1,
		},
		JobRepo:      jobRepo,
		DocumentRepo: docRepo,
		VectorRepo:   vectorRepo,
		PythonClient: pythonClient,
		Logger:       logger,
	}

	worker := NewUploadWorker(config)

	// Create test job that has already exceeded retries
	job := &repositories.Job{
		ID:         "job-max-retries",
		Type:       repositories.JobTypeDocumentUpload,
		Status:     repositories.JobStatusProcessing,
		RetryCount: 3,
		MaxRetries: 3,
		Payload: map[string]interface{}{
			"filename":          "test.pdf",
			"file_size":         float64(1024),
			"collection":        "test-collection",
			"chunking_strategy": "fixed",
			"chunk_size":        float64(512),
			"chunk_overlap":     float64(50),
		},
	}

	ctx := context.Background()

	// Mock document registration
	docRepo.On("Register", mock.Anything, mock.Anything).Return(nil)

	// Mock progress update
	jobRepo.On("SetProgress", mock.Anything, "job-max-retries", mock.Anything, mock.Anything).Return(nil)

	// Mock Python client to fail
	pythonClient.On("ParseDocument", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		ParseResult{},
		errors.New("parse failed"),
	)

	// Mock failed status update
	jobRepo.On("UpdateJobStatus", mock.Anything, "job-max-retries", repositories.JobStatusFailed, 0, mock.Anything).Return(nil)

	// Mock document update to failed
	docRepo.On("Update", mock.Anything, mock.Anything, mock.MatchedBy(func(updates map[string]interface{}) bool {
		return updates["status"] == repositories.DocumentStatusFailed
	})).Return(nil)

	// Process job
	worker.processJob(ctx, job)

	// Verify permanent failure
	jobRepo.AssertCalled(t, "UpdateJobStatus", mock.Anything, "job-max-retries", repositories.JobStatusFailed, 0, mock.Anything)
	docRepo.AssertCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
}

func TestUploadJobPayload_DocumentID(t *testing.T) {
	payload := &UploadJobPayload{
		Filename:   "test-document.pdf",
		Collection: "my-collection",
	}

	docID := payload.DocumentID()
	assert.NotEmpty(t, docID)
	assert.Contains(t, docID, "doc-")
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple.pdf", "simple-pdf"},
		{"with spaces.pdf", "with-spaces-pdf"},
		{"special!@#$%chars.pdf", "special-----chars-pdf"},
		{"123numbers456.pdf", "123numbers456-pdf"},
		{"CamelCase.PDF", "CamelCase-PDF"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeFilename(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultLogger(t *testing.T) {
	logger := &DefaultLogger{}

	// Should not panic
	logger.Info("test info")
	logger.Error("test error")
	logger.Warn("test warn")
	logger.Debug("test debug")
}

func TestUploadWorker_ParsePayload(t *testing.T) {
	config := UploadWorkerConfig{
		WorkerConfig: DefaultWorkerConfig("test-worker"),
		JobRepo:      &MockJobRepository{},
		DocumentRepo: &MockDocumentRepository{},
		VectorRepo:   &MockVectorRepository{},
		PythonClient: &MockPythonClient{},
		Logger:       &MockLogger{},
	}

	worker := NewUploadWorker(config)

	t.Run("valid payload", func(t *testing.T) {
		payload := map[string]interface{}{
			"filename":          "test.pdf",
			"file_size":         float64(1024),
			"collection":        "test-coll",
			"chunking_strategy": "fixed",
			"chunk_size":        float64(512),
			"chunk_overlap":     float64(50),
			"extract_metadata":  true,
			"num_questions":     float64(5),
			"max_pages":         float64(10),
		}

		parsed, err := worker.parsePayload(payload)
		require.NoError(t, err)
		assert.Equal(t, "test.pdf", parsed.Filename)
		assert.Equal(t, int64(1024), parsed.FileSize)
		assert.Equal(t, "test-coll", parsed.Collection)
		assert.Equal(t, "fixed", parsed.ChunkingStrategy)
		assert.Equal(t, 512, parsed.ChunkSize)
		assert.Equal(t, 50, parsed.ChunkOverlap)
		assert.True(t, parsed.ExtractMetadata)
		assert.Equal(t, 5, parsed.NumQuestions)
		assert.Equal(t, 10, parsed.MaxPages)
	})

	t.Run("invalid payload", func(t *testing.T) {
		payload := map[string]interface{}{
			"invalid": "data",
		}

		parsed, err := worker.parsePayload(payload)
		require.NoError(t, err) // JSON unmarshaling succeeds, but fields are zero values
		assert.Empty(t, parsed.Filename)
	})
}
