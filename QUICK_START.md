# Quick Start Guide - Risk Analyzer Go Backend

## Prerequisites
- Docker (for Redis & ChromaDB)
- Go 1.21+ 
- Python 3.11+ (for Python backend)

## Start All Services (Docker Compose - Recommended)

```bash
docker-compose up -d
```

That's it! All services will start:
- Go Backend: http://localhost:8080
- Python Backend: http://localhost:8001  
- ChromaDB: http://localhost:8000
- Redis: localhost:6379

## Manual Startup (Local Development)

### 1. Start Dependencies

```bash
# Redis
docker run -d -p 6379:6379 --name redis redis:7-alpine

# ChromaDB  
docker run -d -p 8000:8000 --name chromadb chromadb/chroma
```

### 2. Start Python Backend

```bash
cd python-backend
python -m uvicorn main:app --host 0.0.0.0 --port 8001
```

### 3. Start Go Backend

```bash
cd backend
go run cmd/grok-server/main.go
```

## Verify Everything Works

```bash
# Health check
curl http://localhost:8080/health

# Create a collection
curl -X POST http://localhost:8080/api/v1/collections \
  -H "Content-Type: application/json" \
  -d '{"name":"test"}'

# Upload a document
echo "Test document content" > test.txt
curl -X POST http://localhost:8080/api/v1/documents/upload \
  -F "file=@test.txt" \
  -F "collection=test"

# Search
curl -X POST http://localhost:8080/api/v1/search \
  -H "Content-Type: application/json" \
  -d '{"query":"test","collection":"test","top_k":5}'
```

## Common Issues

### "Redis connection failed"
```bash
# Check if Redis is running
docker ps | grep redis

# Start Redis
docker run -d -p 6379:6379 --name redis redis:7-alpine
```

### "ChromaDB connection failed"  
```bash
# Check if ChromaDB is running
docker ps | grep chroma

# Start ChromaDB
docker run -d -p 8000:8000 --name chromadb chromadb/chroma
```

### "Python backend not responding"
```bash
# Check Python backend
curl http://localhost:8001/health

# Make sure it's running on port 8001 (not 8000 - ChromaDB uses 8000)
export PYTHON_BACKEND_URL=http://localhost:8001
```

## Environment Variables

Create a `.env` file or export these:

```bash
# Python Backend
PYTHON_BACKEND_URL=http://localhost:8001

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379

# ChromaDB
CHROMA_HOST=localhost  
CHROMA_PORT=8000
```

## Documentation

- Full config guide: `backend/CONFIG.md`
- Task 4.1 details: `backend/TASK_4_1_COMPLETED.md`
- API docs: http://localhost:8080/swagger/ (when server is running)

## Next Steps

1. Try the document upload workflow
2. Test semantic search  
3. Explore collection management
4. Check out the Swagger docs

Enjoy! 🚀
