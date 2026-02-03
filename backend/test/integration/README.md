# Integration Tests

This directory contains integration tests that verify connectivity and operations with external services.

## Tests

### `db_connectivity_test.go`

Tests basic connectivity and operations for database services used by the application.

#### ChromaDB Tests

- **TestChromaDBConnectivity**: Verifies connection to ChromaDB vector database
  - **Note**: The ChromaDB Go client library (v0.3.0-alpha.1) has known v1/v2 API compatibility issues
  - This test is currently skipped but documents the issue
  - Production code will use a custom HTTP wrapper to handle v2 API directly
  - ChromaDB runs on port **8001** (mapped to container port 8000)

#### Redis Tests

- **TestRedisConnectivity**: Verifies basic connection to Redis
  - Tests PING/PONG
  - Tests basic SET/GET operations
  - Redis runs on port **6379**

- **TestRedisOperations**: Tests Redis operations used for document registry
  - **Hash operations** (HSET/HGETALL) - Used to store document metadata
  - **Set operations** (SADD/SMEMBERS) - Used to track document IDs
  - Validates the data structures we use in production

## Running Tests

### Run all integration tests:
```bash
cd backend
go test -v ./test/integration/
```

### Run specific test:
```bash
go test -v ./test/integration/ -run TestRedisConnectivity
```

### Skip integration tests (for CI):
```bash
go test -short ./test/integration/
```

## Prerequisites

Integration tests require the following services to be running:

1. **ChromaDB** - Vector database
   ```bash
   docker-compose up chromadb
   ```
   - Accessible at: `http://localhost:8001`
   - Health check: `curl http://localhost:8001/api/v2`

2. **Redis** - Document registry and caching
   ```bash
   docker-compose up redis
   ```
   - Accessible at: `localhost:6379`
   - Health check: `redis-cli ping`

### Quick Start (All Services)
```bash
# From project root
docker-compose up -d chromadb redis

# Run tests
cd backend
go test -v ./test/integration/
```

## Test Results (Expected)

✅ **TestRedisConnectivity** - PASS  
✅ **TestRedisOperations** - PASS  
⚠️  **TestChromaDBConnectivity** - SKIP (known client library issues)

## Known Issues

### ChromaDB Go Client

The official ChromaDB Go client has compatibility issues with ChromaDB's v2 API:
- Client uses v1 API endpoints which are deprecated
- ChromaDB server returns: `Error (410) Unimplemented: The v1 API is deprecated. Please use /v2 apis`

**Solution**: We will implement a custom HTTP client wrapper in `internal/db/chromadb.go` that directly calls v2 API endpoints.

## Future Tests

Additional integration tests to add:
- [ ] MySQL connectivity and operations
- [ ] End-to-end document upload workflow
- [ ] Vector search operations
- [ ] Collection management operations
- [ ] Transaction rollback scenarios