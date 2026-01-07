package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"risk-analyzer/internal/db"
	"risk-analyzer/internal/handlers"
	"risk-analyzer/internal/repositories"
	"risk-analyzer/internal/routes"
	"risk-analyzer/internal/services"
	"risk-analyzer/internal/workers"

	"github.com/gorilla/mux"
	httpSwagger "github.com/swaggo/http-swagger"
)

// corsMiddleware adds CORS headers to all responses
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func NewServer() *http.Server {
	logger := log.New(os.Stdout, "[SERVER] ", log.LstdFlags)

	// Initialize services
	pythonClient := initializePythonClient(logger)
	docRepo, vectorRepo, jobRepo := initializeRepositories(logger)

	// Create service layer (only if repositories are available)
	var documentService *services.DocumentService
	var searchService *services.SearchService
	var collectionService *services.CollectionService
	var docHandler *handlers.DocumentHandler
	var searchHandler *handlers.SearchHandler
	var collectionHandler *handlers.CollectionHandler

	if docRepo != nil && vectorRepo != nil && jobRepo != nil {
		documentService = services.NewDocumentService(pythonClient, docRepo, vectorRepo, jobRepo, logger)
		searchService = services.NewSearchService(pythonClient, vectorRepo, docRepo, logger)
		collectionService = services.NewCollectionService(vectorRepo, docRepo, logger)

		// Create handlers
		docHandler = handlers.NewDocumentHandler(documentService, logger)
		searchHandler = handlers.NewSearchHandler(searchService, logger)
		collectionHandler = handlers.NewCollectionHandler(collectionService, logger)

		// Start background workers for async job processing
		go startWorkers(pythonClient, docRepo, vectorRepo, jobRepo, logger)

		logger.Println("✅ Orchestration services initialized successfully")
		logger.Println("✅ Background workers started for async job processing")
	} else {
		logger.Println("⚠️  Orchestration services disabled - repositories not available")
		logger.Println("   New API endpoints (/api/v1/*) will not be registered")
		logger.Println("   Legacy endpoints will continue to work")
	}

	// Create handlers struct
	h := &routes.Handlers{
		// Legacy handlers (existing)
		Health:                         handlers.HealthCheckHandler,
		Home:                           handlers.HomeHandler,
		Issues:                         handlers.IssuesHandler,
		IssuesWithKeywords:             handlers.IssuesWithKeywordsHandler,
		KeywordsOnly:                   handlers.KeywordsOnlyHandler,
		Chat:                           handlers.ChatHandler,
		LLMHealth:                      handlers.LLMHealthHandler,
		DocumentsList:                  handlers.DocumentsListHandler,
		DocumentsChunks:                handlers.DocumentsChunksHandler,
		DocumentsCollectionStats:       handlers.DocumentsCollectionStatsHandler,
		DocumentsProcessExample:        handlers.DocumentsProcessExampleHandler,
		DocumentsUpload:                handlers.DocumentsUploadHandler,
		DocumentsSearchResetCollection: handlers.DocumentsSearchResetCollectionHandler,
		DocumentsSearchCollectionStats: handlers.DocumentsSearchCollectionStatsHandler,
		DocumentsSearchCollections:     handlers.DocumentsSearchCollectionsHandler,
		DocumentsSearchQuery:           handlers.DocumentsSearchQueryHandler,
		DocumentsSearch:                handlers.DocumentsSearchHandler,
		RAGChat:                        handlers.RAGChatHandler,
		// Legacy microservice handlers removed - not implemented yet
		ListDocuments:         nil,
		ListVectorDocuments:   nil,
		DocumentServiceHealth: nil,
		UploadDocument:        nil,
		GetDocumentChunks:     nil,
		DeleteCollection:      nil,
		DeleteDocument:        nil,

		// New orchestration handlers
		DocHandler:        docHandler,
		SearchHandler:     searchHandler,
		CollectionHandler: collectionHandler,
	}

	router := mux.NewRouter()
	routes.RegisterRoutes(router, h)

	// Add Swagger endpoints
	router.PathPrefix("/swagger/").Handler(httpSwagger.Handler(
		httpSwagger.URL("http://localhost:8080/swagger/doc.json"), // The url pointing to API definition
		httpSwagger.DeepLinking(true),
		httpSwagger.DocExpansion("none"),
		httpSwagger.DomID("swagger-ui"),
	))

	return &http.Server{
		Addr:    ":8080",
		Handler: corsMiddleware(router),
	}
}

// initializePythonClient creates and configures the Python backend client
func initializePythonClient(logger *log.Logger) services.PythonClientInterface {
	pythonURL := os.Getenv("PYTHON_BACKEND_URL")
	if pythonURL == "" {
		pythonURL = "http://localhost:8000"
	}

	timeout := 60 * time.Second
	retries := 3

	logger.Printf("Initializing Python client: %s (timeout: %v, retries: %d)", pythonURL, timeout, retries)
	return services.NewPythonClientWithOptions(pythonURL, timeout, retries)
}

// initializeRepositories creates repository instances with Redis and ChromaDB
func initializeRepositories(logger *log.Logger) (repositories.DocumentRepository, repositories.VectorRepository, repositories.JobRepository) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Initialize Redis client
	redisConfig := getRedisConfig()
	logger.Printf("Connecting to Redis: %s:%d (DB: %d)", redisConfig.Host, redisConfig.Port, redisConfig.DB)

	redisClient, err := db.NewRedisClient(redisConfig)
	if err != nil {
		logger.Printf("❌ Failed to create Redis client: %v", err)
		logger.Println("   Orchestration services will be disabled")
		return nil, nil, nil
	}

	// Test Redis connection
	if err := redisClient.Ping(ctx); err != nil {
		logger.Printf("❌ Redis connection failed: %v", err)
		logger.Println("   Orchestration services will be disabled")
		logger.Println("   Hint: Ensure Redis is running (docker run -d -p 6379:6379 redis:7-alpine)")
		return nil, nil, nil
	}
	logger.Println("✅ Redis connected successfully")

	// Initialize ChromaDB client
	chromaConfig := getChromaConfig()
	logger.Printf("Connecting to ChromaDB: %s:%d", chromaConfig.Host, chromaConfig.Port)

	chromaClient := db.NewChromaDBClient(chromaConfig)

	// Test ChromaDB connection
	if err := chromaClient.Heartbeat(ctx); err != nil {
		logger.Printf("❌ ChromaDB connection failed: %v", err)
		logger.Println("   Orchestration services will be disabled")
		logger.Println("   Hint: Ensure ChromaDB is running (docker run -d -p 8000:8000 chromadb/chroma)")
		return nil, nil, nil
	}
	logger.Println("✅ ChromaDB connected successfully")

	// Create repository instances
	docRepo := repositories.NewRedisDocumentRepository(redisClient.GetClient())
	jobRepo := repositories.NewRedisJobRepository(redisClient.GetClient())
	vectorRepo := repositories.NewChromaVectorRepository(chromaClient)

	logger.Println("✅ All repositories initialized successfully")

	return docRepo, vectorRepo, jobRepo
}

// getRedisConfig reads Redis configuration from environment variables
func getRedisConfig() db.RedisConfig {
	config := db.DefaultRedisConfig()

	if host := os.Getenv("REDIS_HOST"); host != "" {
		config.Host = host
	}

	if portStr := os.Getenv("REDIS_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			config.Port = port
		}
	}

	if password := os.Getenv("REDIS_PASSWORD"); password != "" {
		config.Password = password
	}

	if dbStr := os.Getenv("REDIS_DB"); dbStr != "" {
		if dbNum, err := strconv.Atoi(dbStr); err == nil {
			config.DB = dbNum
		}
	}

	if poolSizeStr := os.Getenv("REDIS_POOL_SIZE"); poolSizeStr != "" {
		if poolSize, err := strconv.Atoi(poolSizeStr); err == nil {
			config.PoolSize = poolSize
		}
	}

	return config
}

// getChromaConfig reads ChromaDB configuration from environment variables
func getChromaConfig() db.ChromaDBConfig {
	config := db.ChromaDBConfig{
		Host:     "localhost",
		Port:     8000,
		Tenant:   "default_tenant",
		Database: "default_database",
		Timeout:  30 * time.Second,
	}

	if host := os.Getenv("CHROMA_HOST"); host != "" {
		config.Host = host
	}

	if portStr := os.Getenv("CHROMA_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			config.Port = port
		}
	}

	if tenant := os.Getenv("CHROMA_TENANT"); tenant != "" {
		config.Tenant = tenant
	}

	if database := os.Getenv("CHROMA_DATABASE"); database != "" {
		config.Database = database
	}

	return config
}

// startWorkers initializes and starts background workers for async job processing
func startWorkers(pythonClient services.PythonClientInterface, docRepo repositories.DocumentRepository, vectorRepo repositories.VectorRepository, jobRepo repositories.JobRepository, logger *log.Logger) {
	ctx := context.Background()

	// Create a simple logger wrapper for workers
	workerLogger := &simpleLogger{logger: logger}

	// Create upload worker
	uploadWorker := workers.NewUploadWorker(workers.UploadWorkerConfig{
		WorkerConfig: workers.WorkerConfig{
			WorkerName:      "upload-worker",
			PollInterval:    2 * time.Second,
			Concurrency:     3,
			ShutdownTimeout: 30 * time.Second,
			MaxRetries:      3,
			RetryDelay:      5 * time.Second,
			EnableRecovery:  true,
		},
		JobRepo:      jobRepo,
		DocumentRepo: docRepo,
		VectorRepo:   vectorRepo,
		PythonClient: &pythonClientAdapter{client: pythonClient},
		Logger:       workerLogger,
	})

	// Start the worker
	if err := uploadWorker.Start(ctx); err != nil {
		logger.Printf("⚠️  Failed to start upload worker: %v", err)
	} else {
		logger.Println("✅ Upload worker started successfully")
	}
}

// simpleLogger wraps log.Logger to implement workers.Logger interface
type simpleLogger struct {
	logger *log.Logger
}

func (l *simpleLogger) Info(msg string, args ...interface{}) {
	l.logger.Printf("[INFO] "+msg, args...)
}

func (l *simpleLogger) Error(msg string, args ...interface{}) {
	l.logger.Printf("[ERROR] "+msg, args...)
}

func (l *simpleLogger) Warn(msg string, args ...interface{}) {
	l.logger.Printf("[WARN] "+msg, args...)
}

func (l *simpleLogger) Debug(msg string, args ...interface{}) {
	l.logger.Printf("[DEBUG] "+msg, args...)
}

// pythonClientAdapter adapts services.PythonClientInterface to workers.PythonClient
type pythonClientAdapter struct {
	client services.PythonClientInterface
}

func (a *pythonClientAdapter) ParseDocument(ctx context.Context, filePath string, extractMetadata bool, numQuestions int, maxPages int) (workers.ParseResult, error) {
	// Debug logging
	log.Printf("DEBUG pythonClientAdapter.ParseDocument - filePath=%s, extractMetadata=%v, numQuestions=%d, maxPages=%d",
		filePath, extractMetadata, numQuestions, maxPages)

	// Read file from disk
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return workers.ParseResult{}, fmt.Errorf("failed to read file: %w", err)
	}

	// Extract filename from path
	filename := filepath.Base(filePath)

	// Debug: Log what we're sending to Python
	log.Printf("DEBUG Calling Python ParseDocument - filename=%s, extractMetadata=%v, maxPages=%d", filename, extractMetadata, maxPages)

	// Call Python client with file data
	resp, err := a.client.ParseDocument(ctx, fileData, filename, extractMetadata, maxPages)
	if err != nil {
		return workers.ParseResult{}, err
	}

	return workers.ParseResult{
		Text:     resp.Text,
		Metadata: resp.Metadata,
	}, nil
}

func (a *pythonClientAdapter) ChunkText(ctx context.Context, text string, strategy string, chunkSize int, chunkOverlap int, extractMetadata bool, numQuestions int) (workers.ChunkResult, error) {
	req := &services.ChunkRequest{
		Text:            text,
		Strategy:        strategy,
		ChunkSize:       chunkSize,
		ChunkOverlap:    chunkOverlap,
		ExtractMetadata: extractMetadata,
		NumQuestions:    numQuestions,
	}
	resp, err := a.client.Chunk(ctx, req)
	if err != nil {
		return workers.ChunkResult{}, err
	}

	// Debug: Log response from Python
	log.Printf("DEBUG ChunkText response - got %d chunks", len(resp.Chunks))

	// Convert TextChunk array to string array and extract metadata
	chunks := make([]string, len(resp.Chunks))
	metadata := make([]map[string]interface{}, len(resp.Chunks))
	for i, chunk := range resp.Chunks {
		chunks[i] = chunk.Text

		// Debug: Log first 3 chunks' metadata
		if i < 3 {
			if chunk.Metadata != nil {
				log.Printf("DEBUG Chunk %d has metadata: Title=%v, Keywords=%v, Questions=%v",
					i, chunk.Metadata.Title, chunk.Metadata.Keywords, chunk.Metadata.Questions)
			} else {
				log.Printf("DEBUG Chunk %d has NO metadata (nil)", i)
			}
		}

		// Extract metadata from chunk if available
		if chunk.Metadata != nil {
			metadata[i] = map[string]interface{}{}
			// Title is a pointer
			if chunk.Metadata.Title != nil && *chunk.Metadata.Title != "" {
				metadata[i]["title"] = *chunk.Metadata.Title
			}
			if len(chunk.Metadata.Keywords) > 0 {
				metadata[i]["keywords"] = chunk.Metadata.Keywords
			}
			if len(chunk.Metadata.Questions) > 0 {
				metadata[i]["questions"] = chunk.Metadata.Questions
			}
			// TokenCount is a pointer
			if chunk.Metadata.TokenCount != nil && *chunk.Metadata.TokenCount > 0 {
				metadata[i]["token_count"] = *chunk.Metadata.TokenCount
			}
		}
	}

	return workers.ChunkResult{
		Chunks:   chunks,
		Metadata: metadata,
	}, nil
}

func (a *pythonClientAdapter) GenerateEmbeddings(ctx context.Context, texts []string) (workers.EmbeddingResult, error) {
	batchSize := 100 // Default batch size
	resp, err := a.client.EmbedBatch(ctx, texts, nil, batchSize, false)
	if err != nil {
		return workers.EmbeddingResult{}, err
	}
	return workers.EmbeddingResult{
		Embeddings: resp.Embeddings,
	}, nil
}
