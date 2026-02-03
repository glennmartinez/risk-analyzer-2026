package services

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"testing"
	"time"

	"risk-analyzer/internal/repositories"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ============================================================================
// Mock Repositories
// ============================================================================

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

func (m *MockVectorRepository) StoreChunks(ctx context.Context, collectionName string, chunks []*repositories.Chunk) error {
	args := m.Called(ctx, collectionName, chunks)
	return args.Error(0)
}

func (m *MockVectorRepository) SearchChunks(ctx context.Context, collectionName string, queryEmbedding []float32, topK int, filter map[string]interface{}) ([]*repositories.SearchResult, error) {
	args := m.Called(ctx, collectionName, queryEmbedding, topK, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repositories.SearchResult), args.Error(1)
}

func (m *MockVectorRepository) DeleteDocument(ctx context.Context, collectionName string, documentID string) (int, error) {
	args := m.Called(ctx, collectionName, documentID)
	return args.Int(0), args.Error(1)
}

func (m *MockVectorRepository) DeleteChunks(ctx context.Context, collectionName string, chunkIDs []string) error {
	args := m.Called(ctx, collectionName, chunkIDs)
	return args.Error(0)
}

func (m *MockVectorRepository) GetChunk(ctx context.Context, collectionName string, chunkID string) (*repositories.Chunk, error) {
	args := m.Called(ctx, collectionName, chunkID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repositories.Chunk), args.Error(1)
}

func (m *MockVectorRepository) ListDocuments(ctx context.Context, collectionName string) ([]*repositories.VectorDocument, error) {
	args := m.Called(ctx, collectionName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repositories.VectorDocument), args.Error(1)
}

func (m *MockVectorRepository) CountDocuments(ctx context.Context, collectionName string) (int, error) {
	args := m.Called(ctx, collectionName)
	return args.Int(0), args.Error(1)
}

func (m *MockVectorRepository) BatchStoreChunks(ctx context.Context, collectionName string, batches [][]*repositories.Chunk) error {
	args := m.Called(ctx, collectionName, batches)
	return args.Error(0)
}

func (m *MockVectorRepository) BatchDeleteChunks(ctx context.Context, collectionName string, chunkIDs []string) error {
	args := m.Called(ctx, collectionName, chunkIDs)
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

// ============================================================================
// Mock Python Client
// ============================================================================

type MockPythonClient struct {
	mock.Mock
}

func (m *MockPythonClient) ParseDocument(ctx context.Context, fileData []byte, filename string, extractMetadata bool, maxPages int) (*ParseResponse, error) {
	args := m.Called(ctx, fileData, filename, extractMetadata, maxPages)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ParseResponse), args.Error(1)
}

func (m *MockPythonClient) Chunk(ctx context.Context, req *ChunkRequest) (*ChunkResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ChunkResponse), args.Error(1)
}

func (m *MockPythonClient) EmbedBatch(ctx context.Context, texts []string, model *string, batchSize int, useCache bool) (*EmbedBatchResponse, error) {
	args := m.Called(ctx, texts, model, batchSize, useCache)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*EmbedBatchResponse), args.Error(1)
}

func (m *MockPythonClient) ChunkSimple(ctx context.Context, text string, chunkSize, chunkOverlap int) (*ChunkResponse, error) {
	args := m.Called(ctx, text, chunkSize, chunkOverlap)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ChunkResponse), args.Error(1)
}

func (m *MockPythonClient) ParseText(ctx context.Context, text string) (*ParseResponse, error) {
	args := m.Called(ctx, text)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ParseResponse), args.Error(1)
}

func (m *MockPythonClient) Embed(ctx context.Context, text string, model *string, useCache bool) (*EmbeddingResponse, error) {
	args := m.Called(ctx, text, model, useCache)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*EmbeddingResponse), args.Error(1)
}

func (m *MockPythonClient) EmbedQuery(ctx context.Context, text string, model *string, useCache bool) (*EmbeddingResponse, error) {
	args := m.Called(ctx, text, model, useCache)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*EmbeddingResponse), args.Error(1)
}

func (m *MockPythonClient) ExtractMetadata(ctx context.Context, req *MetadataRequest) (*MetadataResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*MetadataResponse), args.Error(1)
}

func (m *MockPythonClient) ExtractTitle(ctx context.Context, text string) (*string, error) {
	args := m.Called(ctx, text)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*string), args.Error(1)
}

func (m *MockPythonClient) ExtractKeywords(ctx context.Context, text string, numKeywords int) ([]string, error) {
	args := m.Called(ctx, text, numKeywords)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockPythonClient) ExtractQuestions(ctx context.Context, text string, numQuestions int) ([]string, error) {
	args := m.Called(ctx, text, numQuestions)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockPythonClient) HealthCheck(ctx context.Context, service string) (bool, error) {
	args := m.Called(ctx, service)
	return args.Bool(0), args.Error(1)
}

func (m *MockPythonClient) GetAvailableModels(ctx context.Context) ([]map[string]interface{}, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]map[string]interface{}), args.Error(1)
}

func (m *MockPythonClient) GetChunkingStrategies(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

// ============================================================================
// Helper Functions
// ============================================================================

func setupTestService(t *testing.T) (*DocumentService, *MockPythonClient, *MockDocumentRepository, *MockVectorRepository, *MockJobRepository) {
	mockPython := new(MockPythonClient)
	mockDocRepo := new(MockDocumentRepository)
	mockVectorRepo := new(MockVectorRepository)
	mockJobRepo := new(MockJobRepository)

	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)

	service := &DocumentService{
		pythonClient: mockPython,
		docRepo:      mockDocRepo,
		vectorRepo:   mockVectorRepo,
		jobRepo:      mockJobRepo,
		logger:       logger,
	}

	return service, mockPython, mockDocRepo, mockVectorRepo, mockJobRepo
}

func createTestUploadRequest() *UploadDocumentRequest {
	content := bytes.NewReader([]byte("test file content"))
	return &UploadDocumentRequest{
		Filename:         "test.pdf",
		FileContent:      content,
		FileSize:         1024,
		Collection:       "test-collection",
		ChunkingStrategy: "semantic",
		ChunkSize:        512,
		ChunkOverlap:     50,
		ExtractMetadata:  true,
		NumQuestions:     3,
		MaxPages:         10,
		Async:            false,
	}
}

func createMockParseResponse() *ParseResponse {
	return &ParseResponse{
		Text:             "This is the extracted text from the document. It contains multiple sentences and paragraphs.",
		Markdown:         "# Document\n\nThis is the extracted text.",
		Metadata:         map[string]interface{}{"author": "Test Author"},
		Pages:            []interface{}{},
		Tables:           []interface{}{},
		Figures:          []interface{}{},
		ExtractionMethod: "pypdf",
		TotalPages:       5,
	}
}

func createMockChunkResponse() *ChunkResponse {
	return &ChunkResponse{
		Chunks: []TextChunk{
			{
				Text:  "This is the first chunk of text.",
				Index: 0,
				Metadata: &ChunkMetadata{
					ChunkIndex: 0,
					Title:      stringPtr("Introduction"),
					Keywords:   []string{"test", "chunk"},
					Questions:  []string{"What is this?"},
				},
			},
			{
				Text:  "This is the second chunk of text.",
				Index: 1,
				Metadata: &ChunkMetadata{
					ChunkIndex: 1,
					Title:      stringPtr("Body"),
					Keywords:   []string{"content", "example"},
					Questions:  []string{"How does it work?"},
				},
			},
		},
		TotalChunks:  2,
		StrategyUsed: "semantic",
		ChunkSize:    512,
		ChunkOverlap: 50,
	}
}

func createMockEmbedResponse() *EmbedBatchResponse {
	return &EmbedBatchResponse{
		Embeddings: [][]float32{
			make([]float32, 384), // MiniLM generates 384-dim vectors
			make([]float32, 384),
		},
		Model:           "all-MiniLM-L6-v2",
		Dimension:       384,
		TotalEmbeddings: 2,
		CachedCount:     0,
	}
}

func createMockMetadataResponse() *MetadataResponse {
	title := "Test Document"
	return &MetadataResponse{
		Title:    &title,
		Keywords: []string{"test", "document", "example"},
		Questions: []string{
			"What is the purpose of this document?",
			"How can it be used?",
			"What are the key features?",
		},
		Metadata: map[string]interface{}{},
	}
}

func stringPtr(s string) *string {
	return &s
}

// ============================================================================
// Tests
// ============================================================================

func TestNewDocumentService(t *testing.T) {
	service, _, _, _, _ := setupTestService(t)
	assert.NotNil(t, service)
	assert.NotNil(t, service.pythonClient)
	assert.NotNil(t, service.docRepo)
	assert.NotNil(t, service.vectorRepo)
	assert.NotNil(t, service.jobRepo)
	assert.NotNil(t, service.logger)
}

func TestValidateUploadRequest(t *testing.T) {
	service, _, _, _, _ := setupTestService(t)

	tests := []struct {
		name        string
		req         *UploadDocumentRequest
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid request",
			req:         createTestUploadRequest(),
			expectError: false,
		},
		{
			name: "Missing filename",
			req: &UploadDocumentRequest{
				FileContent: bytes.NewReader([]byte("test")),
				Collection:  "test",
			},
			expectError: true,
			errorMsg:    "filename is required",
		},
		{
			name: "Missing file content",
			req: &UploadDocumentRequest{
				Filename:   "test.pdf",
				Collection: "test",
			},
			expectError: true,
			errorMsg:    "file content is required",
		},
		{
			name: "Missing collection",
			req: &UploadDocumentRequest{
				Filename:    "test.pdf",
				FileContent: bytes.NewReader([]byte("test")),
			},
			expectError: true,
			errorMsg:    "collection is required",
		},
		{
			name: "Unsupported file type",
			req: &UploadDocumentRequest{
				Filename:    "test.exe",
				FileContent: bytes.NewReader([]byte("test")),
				Collection:  "test",
			},
			expectError: true,
			errorMsg:    "unsupported file type: .exe",
		},
		{
			name: "Valid txt file",
			req: &UploadDocumentRequest{
				Filename:    "test.txt",
				FileContent: bytes.NewReader([]byte("test")),
				Collection:  "test",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.validateUploadRequest(tt.req)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				// Check defaults are set
				assert.NotEmpty(t, tt.req.ChunkingStrategy)
				assert.Greater(t, tt.req.ChunkSize, 0)
				assert.GreaterOrEqual(t, tt.req.ChunkOverlap, 0)
			}
		})
	}
}

func TestUploadDocumentSync_Success(t *testing.T) {
	service, mockPython, mockDocRepo, mockVectorRepo, _ := setupTestService(t)
	ctx := context.Background()
	req := createTestUploadRequest()

	// Setup mocks
	mockDocRepo.On("Register", ctx, mock.AnythingOfType("*repositories.Document")).Return(nil)
	mockDocRepo.On("Update", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("map[string]interface {}")).Return(nil)

	mockPython.On("ParseDocument", ctx, mock.AnythingOfType("[]uint8"), req.Filename, req.ExtractMetadata, req.MaxPages).Return(createMockParseResponse(), nil)
	mockPython.On("Chunk", ctx, mock.AnythingOfType("*services.ChunkRequest")).Return(createMockChunkResponse(), nil)
	mockPython.On("EmbedBatch", ctx, mock.AnythingOfType("[]string"), mock.Anything, mock.AnythingOfType("int"), mock.AnythingOfType("bool")).Return(createMockEmbedResponse(), nil)
	mockPython.On("ExtractMetadata", ctx, mock.AnythingOfType("*services.MetadataRequest")).Return(createMockMetadataResponse(), nil)

	mockVectorRepo.On("CollectionExists", ctx, req.Collection).Return(true, nil)
	mockVectorRepo.On("StoreChunks", ctx, req.Collection, mock.AnythingOfType("[]*repositories.Chunk")).Return(nil)

	// Execute
	resp, err := service.UploadDocument(ctx, req)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.DocumentID)
	assert.Equal(t, "test.pdf", resp.Filename)
	assert.Equal(t, "test-collection", resp.Collection)
	assert.Equal(t, 2, resp.ChunkCount)
	assert.Equal(t, "completed", resp.Status)
	assert.GreaterOrEqual(t, resp.ProcessingTimeMs, 0.0)
	assert.NotNil(t, resp.Metadata)

	mockDocRepo.AssertExpectations(t)
	mockPython.AssertExpectations(t)
	mockVectorRepo.AssertExpectations(t)
}

func TestUploadDocumentSync_ParseFails(t *testing.T) {
	service, mockPython, mockDocRepo, _, _ := setupTestService(t)
	ctx := context.Background()
	req := createTestUploadRequest()

	// Setup mocks
	mockDocRepo.On("Register", ctx, mock.AnythingOfType("*repositories.Document")).Return(nil)
	mockDocRepo.On("Update", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("map[string]interface {}")).Return(nil)

	mockPython.On("ParseDocument", ctx, mock.AnythingOfType("[]uint8"), req.Filename, req.ExtractMetadata, req.MaxPages).Return(nil, errors.New("parse failed"))

	// Execute
	resp, err := service.UploadDocument(ctx, req)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "parse failed")

	mockDocRepo.AssertExpectations(t)
	mockPython.AssertExpectations(t)
}

func TestUploadDocumentSync_ChunkingFails(t *testing.T) {
	service, mockPython, mockDocRepo, _, _ := setupTestService(t)
	ctx := context.Background()
	req := createTestUploadRequest()

	// Setup mocks
	mockDocRepo.On("Register", ctx, mock.AnythingOfType("*repositories.Document")).Return(nil)
	mockDocRepo.On("Update", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("map[string]interface {}")).Return(nil)

	mockPython.On("ParseDocument", ctx, mock.AnythingOfType("[]uint8"), req.Filename, req.ExtractMetadata, req.MaxPages).Return(createMockParseResponse(), nil)
	mockPython.On("Chunk", ctx, mock.AnythingOfType("*services.ChunkRequest")).Return(nil, errors.New("chunking failed"))

	// Execute
	resp, err := service.UploadDocument(ctx, req)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "chunking failed")

	mockDocRepo.AssertExpectations(t)
	mockPython.AssertExpectations(t)
}

func TestUploadDocumentSync_EmbeddingFails(t *testing.T) {
	service, mockPython, mockDocRepo, _, _ := setupTestService(t)
	ctx := context.Background()
	req := createTestUploadRequest()

	// Setup mocks
	mockDocRepo.On("Register", ctx, mock.AnythingOfType("*repositories.Document")).Return(nil)
	mockDocRepo.On("Update", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("map[string]interface {}")).Return(nil)

	mockPython.On("ParseDocument", ctx, mock.AnythingOfType("[]uint8"), req.Filename, req.ExtractMetadata, req.MaxPages).Return(createMockParseResponse(), nil)
	mockPython.On("Chunk", ctx, mock.AnythingOfType("*services.ChunkRequest")).Return(createMockChunkResponse(), nil)
	mockPython.On("EmbedBatch", ctx, mock.AnythingOfType("[]string"), mock.Anything, mock.AnythingOfType("int"), mock.AnythingOfType("bool")).Return(nil, errors.New("embedding failed"))

	// Execute
	resp, err := service.UploadDocument(ctx, req)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "embedding failed")

	mockDocRepo.AssertExpectations(t)
	mockPython.AssertExpectations(t)
}

func TestUploadDocumentSync_VectorStorageFails(t *testing.T) {
	service, mockPython, mockDocRepo, mockVectorRepo, _ := setupTestService(t)
	ctx := context.Background()
	req := createTestUploadRequest()

	// Setup mocks
	mockDocRepo.On("Register", ctx, mock.AnythingOfType("*repositories.Document")).Return(nil)
	mockDocRepo.On("Update", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("map[string]interface {}")).Return(nil)

	mockPython.On("ParseDocument", ctx, mock.AnythingOfType("[]uint8"), req.Filename, req.ExtractMetadata, req.MaxPages).Return(createMockParseResponse(), nil)
	mockPython.On("Chunk", ctx, mock.AnythingOfType("*services.ChunkRequest")).Return(createMockChunkResponse(), nil)
	mockPython.On("EmbedBatch", ctx, mock.AnythingOfType("[]string"), mock.Anything, mock.AnythingOfType("int"), mock.AnythingOfType("bool")).Return(createMockEmbedResponse(), nil)
	mockPython.On("ExtractMetadata", ctx, mock.AnythingOfType("*services.MetadataRequest")).Return(createMockMetadataResponse(), nil)

	mockVectorRepo.On("CollectionExists", ctx, req.Collection).Return(true, nil)
	mockVectorRepo.On("StoreChunks", ctx, req.Collection, mock.AnythingOfType("[]*repositories.Chunk")).Return(errors.New("storage failed"))

	// Execute
	resp, err := service.UploadDocument(ctx, req)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "storage failed")

	mockDocRepo.AssertExpectations(t)
	mockPython.AssertExpectations(t)
	mockVectorRepo.AssertExpectations(t)
}

func TestUploadDocumentSync_CreateCollectionIfNotExists(t *testing.T) {
	service, mockPython, mockDocRepo, mockVectorRepo, _ := setupTestService(t)
	ctx := context.Background()
	req := createTestUploadRequest()

	// Setup mocks
	mockDocRepo.On("Register", ctx, mock.AnythingOfType("*repositories.Document")).Return(nil)
	mockDocRepo.On("Update", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("map[string]interface {}")).Return(nil)

	mockPython.On("ParseDocument", ctx, mock.AnythingOfType("[]uint8"), req.Filename, req.ExtractMetadata, req.MaxPages).Return(createMockParseResponse(), nil)
	mockPython.On("Chunk", ctx, mock.AnythingOfType("*services.ChunkRequest")).Return(createMockChunkResponse(), nil)
	mockPython.On("EmbedBatch", ctx, mock.AnythingOfType("[]string"), mock.Anything, mock.AnythingOfType("int"), mock.AnythingOfType("bool")).Return(createMockEmbedResponse(), nil)
	mockPython.On("ExtractMetadata", ctx, mock.AnythingOfType("*services.MetadataRequest")).Return(createMockMetadataResponse(), nil)

	mockVectorRepo.On("CollectionExists", ctx, req.Collection).Return(false, nil)
	mockVectorRepo.On("CreateCollection", ctx, req.Collection, mock.Anything).Return(nil)
	mockVectorRepo.On("StoreChunks", ctx, req.Collection, mock.AnythingOfType("[]*repositories.Chunk")).Return(nil)

	// Execute
	resp, err := service.UploadDocument(ctx, req)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "completed", resp.Status)

	mockVectorRepo.AssertCalled(t, "CreateCollection", ctx, req.Collection, mock.Anything)
	mockDocRepo.AssertExpectations(t)
	mockPython.AssertExpectations(t)
	mockVectorRepo.AssertExpectations(t)
}

func TestUploadDocumentAsync_Success(t *testing.T) {
	service, _, mockDocRepo, _, mockJobRepo := setupTestService(t)
	ctx := context.Background()
	req := createTestUploadRequest()
	req.Async = true

	// Setup mocks
	mockJobRepo.On("CreateJob", ctx, mock.AnythingOfType("*repositories.Job")).Return(nil)
	mockDocRepo.On("Register", ctx, mock.AnythingOfType("*repositories.Document")).Return(nil)

	// Execute
	resp, err := service.UploadDocument(ctx, req)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.DocumentID)
	assert.NotEmpty(t, resp.JobID)
	assert.Equal(t, "queued", resp.Status)
	assert.Contains(t, resp.Message, "queued")

	mockJobRepo.AssertExpectations(t)
	mockDocRepo.AssertExpectations(t)
}

func TestUploadDocumentAsync_JobCreationFails(t *testing.T) {
	service, _, _, _, mockJobRepo := setupTestService(t)
	ctx := context.Background()
	req := createTestUploadRequest()
	req.Async = true

	// Setup mocks
	mockJobRepo.On("CreateJob", ctx, mock.AnythingOfType("*repositories.Job")).Return(errors.New("job creation failed"))

	// Execute
	resp, err := service.UploadDocument(ctx, req)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "job creation failed")

	mockJobRepo.AssertExpectations(t)
}

func TestDeleteDocument_Success(t *testing.T) {
	service, _, mockDocRepo, mockVectorRepo, _ := setupTestService(t)
	ctx := context.Background()
	documentID := "test-doc-123"

	doc := &repositories.Document{
		ID:         documentID,
		Collection: "test-collection",
		Status:     repositories.DocumentStatusCompleted,
	}

	// Setup mocks
	mockDocRepo.On("Get", ctx, documentID).Return(doc, nil)
	mockVectorRepo.On("DeleteDocument", ctx, doc.Collection, documentID).Return(5, nil)
	mockDocRepo.On("Update", ctx, documentID, mock.AnythingOfType("map[string]interface {}")).Return(nil)

	// Execute
	err := service.DeleteDocument(ctx, documentID)

	// Assert
	assert.NoError(t, err)

	mockDocRepo.AssertExpectations(t)
	mockVectorRepo.AssertExpectations(t)
}

func TestDeleteDocument_NotFound(t *testing.T) {
	service, _, mockDocRepo, _, _ := setupTestService(t)
	ctx := context.Background()
	documentID := "non-existent"

	// Setup mocks
	mockDocRepo.On("Get", ctx, documentID).Return(nil, repositories.DocumentNotFoundError(documentID))

	// Execute
	err := service.DeleteDocument(ctx, documentID)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get document")

	mockDocRepo.AssertExpectations(t)
}

func TestGetDocument_Success(t *testing.T) {
	service, _, mockDocRepo, _, _ := setupTestService(t)
	ctx := context.Background()
	documentID := "test-doc-123"

	expectedDoc := &repositories.Document{
		ID:         documentID,
		Filename:   "test.pdf",
		Collection: "test-collection",
		Status:     repositories.DocumentStatusCompleted,
		ChunkCount: 10,
	}

	// Setup mocks
	mockDocRepo.On("Get", ctx, documentID).Return(expectedDoc, nil)

	// Execute
	doc, err := service.GetDocument(ctx, documentID)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, doc)
	assert.Equal(t, expectedDoc.ID, doc.ID)
	assert.Equal(t, expectedDoc.Filename, doc.Filename)

	mockDocRepo.AssertExpectations(t)
}

func TestListDocuments_Success(t *testing.T) {
	service, _, mockDocRepo, _, _ := setupTestService(t)
	ctx := context.Background()

	expectedDocs := []*repositories.Document{
		{ID: "doc1", Filename: "file1.pdf"},
		{ID: "doc2", Filename: "file2.pdf"},
	}

	// Setup mocks
	mockDocRepo.On("List", ctx).Return(expectedDocs, nil)

	// Execute
	docs, err := service.ListDocuments(ctx)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, docs)
	assert.Len(t, docs, 2)

	mockDocRepo.AssertExpectations(t)
}

func TestListDocumentsByCollection_Success(t *testing.T) {
	service, _, mockDocRepo, _, _ := setupTestService(t)
	ctx := context.Background()
	collection := "test-collection"

	expectedDocs := []*repositories.Document{
		{ID: "doc1", Collection: collection},
		{ID: "doc2", Collection: collection},
	}

	// Setup mocks
	mockDocRepo.On("ListByCollection", ctx, collection).Return(expectedDocs, nil)

	// Execute
	docs, err := service.ListDocumentsByCollection(ctx, collection)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, docs)
	assert.Len(t, docs, 2)
	for _, doc := range docs {
		assert.Equal(t, collection, doc.Collection)
	}

	mockDocRepo.AssertExpectations(t)
}

func TestGetDocumentStatus_Success(t *testing.T) {
	service, _, mockDocRepo, _, _ := setupTestService(t)
	ctx := context.Background()
	documentID := "test-doc-123"

	doc := &repositories.Document{
		ID:     documentID,
		Status: repositories.DocumentStatusCompleted,
	}

	// Setup mocks
	mockDocRepo.On("Get", ctx, documentID).Return(doc, nil)

	// Execute
	status, err := service.GetDocumentStatus(ctx, documentID)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, repositories.DocumentStatusCompleted, status)

	mockDocRepo.AssertExpectations(t)
}

func TestUploadDocumentSync_WithoutMetadataExtraction(t *testing.T) {
	service, mockPython, mockDocRepo, mockVectorRepo, _ := setupTestService(t)
	ctx := context.Background()
	req := createTestUploadRequest()
	req.ExtractMetadata = false

	// Setup mocks
	mockDocRepo.On("Register", ctx, mock.AnythingOfType("*repositories.Document")).Return(nil)
	mockDocRepo.On("Update", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("map[string]interface {}")).Return(nil)

	mockPython.On("ParseDocument", ctx, mock.AnythingOfType("[]uint8"), req.Filename, req.ExtractMetadata, req.MaxPages).Return(createMockParseResponse(), nil)
	mockPython.On("Chunk", ctx, mock.AnythingOfType("*services.ChunkRequest")).Return(createMockChunkResponse(), nil)
	mockPython.On("EmbedBatch", ctx, mock.AnythingOfType("[]string"), mock.Anything, mock.AnythingOfType("int"), mock.AnythingOfType("bool")).Return(createMockEmbedResponse(), nil)
	// Note: ExtractMetadata should NOT be called

	mockVectorRepo.On("CollectionExists", ctx, req.Collection).Return(true, nil)
	mockVectorRepo.On("StoreChunks", ctx, req.Collection, mock.AnythingOfType("[]*repositories.Chunk")).Return(nil)

	// Execute
	resp, err := service.UploadDocument(ctx, req)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "completed", resp.Status)

	// ExtractMetadata should NOT have been called
	mockPython.AssertNotCalled(t, "ExtractMetadata", mock.Anything, mock.Anything, mock.Anything)

	mockDocRepo.AssertExpectations(t)
	mockPython.AssertExpectations(t)
	mockVectorRepo.AssertExpectations(t)
}
