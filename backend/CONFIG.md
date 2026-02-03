# Configuration Guide

This document describes all environment variables used by the Risk Analyzer Go backend.

## Table of Contents

- [Python Backend Configuration](#python-backend-configuration)
- [Redis Configuration](#redis-configuration)
- [ChromaDB Configuration](#chromadb-configuration)
- [Server Configuration](#server-configuration)
- [Quick Start](#quick-start)

---

## Python Backend Configuration

The Go backend communicates with the Python compute backend for parsing, chunking, and embedding operations.

| Variable | Default | Description |
|----------|---------|-------------|
| `PYTHON_BACKEND_URL` | `http://localhost:8000` | Base URL of the Python backend service |

**Example:**
```bash
export PYTHON_BACKEND_URL="http://localhost:8001"
```

---

## Redis Configuration

Redis is used for document registry and job queue management.

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_HOST` | `localhost` | Redis server hostname |
| `REDIS_PORT` | `6379` | Redis server port |
| `REDIS_PASSWORD` | _(empty)_ | Redis password (optional) |
| `REDIS_DB` | `0` | Redis database number (0-15) |
| `REDIS_POOL_SIZE` | `10` | Maximum number of connections in pool |

**Example:**
```bash
export REDIS_HOST="localhost"
export REDIS_PORT="6379"
export REDIS_PASSWORD=""
export REDIS_DB="0"
export REDIS_POOL_SIZE="10"
```

### Redis Data Structure

- **Document Registry**: `document:{id}` - JSON document metadata
- **Document Index**: `documents:index` - Set of all document IDs
- **Collection Index**: `collection:{name}` - Set of document IDs in collection
- **Filename Index**: `filename:{name}` - Document ID by filename
- **Status Index**: `status:{status}` - Set of document IDs by status
- **Job Queue**: `job:queue:{type}` - List of pending jobs by type
- **Job Data**: `job:{id}` - JSON job metadata

---

## ChromaDB Configuration

ChromaDB is used for vector storage and semantic search.

| Variable | Default | Description |
|----------|---------|-------------|
| `CHROMA_HOST` | `localhost` | ChromaDB server hostname |
| `CHROMA_PORT` | `8000` | ChromaDB server port |
| `CHROMA_TENANT` | `default_tenant` | ChromaDB tenant name |
| `CHROMA_DATABASE` | `default_database` | ChromaDB database name |

**Example:**
```bash
export CHROMA_HOST="localhost"
export CHROMA_PORT="8000"
export CHROMA_TENANT="default_tenant"
export CHROMA_DATABASE="default_database"
```

### ChromaDB Collections

Each collection stores document chunks with their embeddings:
- **Collection Name**: User-defined (e.g., `documents`, `contracts`, etc.)
- **Chunk IDs**: `{document_id}:chunk:{index}`
- **Metadata**: Document ID, chunk index, original text
- **Embeddings**: 384-dimensional vectors (sentence-transformers/all-MiniLM-L6-v2)

---

## Server Configuration

General server settings.

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server port |
| `GO_ENV` | `development` | Environment (`development`, `staging`, `production`) |

**Example:**
```bash
export PORT="8080"
export GO_ENV="production"
```

---

## Quick Start

### Local Development (without Docker)

1. **Start Redis:**
   ```bash
   docker run -d -p 6379:6379 --name redis redis:7-alpine
   ```

2. **Start ChromaDB:**
   ```bash
   docker run -d -p 8000:8000 --name chromadb chromadb/chroma
   ```

3. **Start Python Backend:**
   ```bash
   cd python-backend
   python -m uvicorn main:app --host 0.0.0.0 --port 8001
   ```

4. **Start Go Backend:**
   ```bash
   cd backend
   go run cmd/grok-server/main.go
   ```

### Docker Compose (recommended)

All services are pre-configured in `docker-compose.yml`:

```bash
docker-compose up -d
```

**Service Ports:**
- Go Backend: `http://localhost:8080`
- Python Backend: `http://localhost:8001`
- ChromaDB: `http://localhost:8000`
- Redis: `localhost:6379`

### Verify Services

```bash
# Check Redis
redis-cli ping
# Should return: PONG

# Check ChromaDB
curl http://localhost:8000/api/v1/heartbeat
# Should return: {"nanosecond heartbeat": ...}

# Check Go Backend
curl http://localhost:8080/health
# Should return: {"status": "healthy"}

# Check Python Backend
curl http://localhost:8001/health
# Should return: {"status": "healthy"}
```

---

## Configuration by Environment

### Development

Create a `.env` file (or export variables):

```bash
# Python Backend
PYTHON_BACKEND_URL=http://localhost:8001

# Redis (local)
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_DB=0

# ChromaDB (local)
CHROMA_HOST=localhost
CHROMA_PORT=8000

# Server
PORT=8080
GO_ENV=development
```

### Production

Use environment variables from your deployment platform (Kubernetes secrets, AWS Parameter Store, etc.):

```bash
# Python Backend (internal service)
PYTHON_BACKEND_URL=http://python-backend-service:8001

# Redis (managed service)
REDIS_HOST=redis-cluster.abc123.us-east-1.cache.amazonaws.com
REDIS_PORT=6379
REDIS_PASSWORD=<secure-password>
REDIS_POOL_SIZE=50

# ChromaDB (managed service or self-hosted)
CHROMA_HOST=chromadb-service.internal
CHROMA_PORT=8000

# Server
PORT=8080
GO_ENV=production
```

---

## Graceful Degradation

The Go backend is designed to degrade gracefully when dependencies are unavailable:

- **Redis unavailable**: New orchestration endpoints (`/api/v1/*`) are disabled; legacy proxy endpoints continue to work
- **ChromaDB unavailable**: Same behavior as Redis unavailable
- **Python backend unavailable**: All endpoints return 503 errors

### Startup Logs

**✅ All services healthy:**
```
[SERVER] Connecting to Redis: localhost:6379 (DB: 0)
[SERVER] ✅ Redis connected successfully
[SERVER] Connecting to ChromaDB: localhost:8000
[SERVER] ✅ ChromaDB connected successfully
[SERVER] ✅ All repositories initialized successfully
[SERVER] ✅ Orchestration services initialized successfully
[SERVER] 📍 New API endpoints:
[SERVER]    POST   /api/v1/documents/upload
[SERVER]    GET    /api/v1/documents
[SERVER]    ...
```

**⚠️ Dependencies unavailable:**
```
[SERVER] Connecting to Redis: localhost:6379 (DB: 0)
[SERVER] ❌ Redis connection failed: dial tcp [::1]:6379: connect: connection refused
[SERVER]    Orchestration services will be disabled
[SERVER]    Hint: Ensure Redis is running (docker run -d -p 6379:6379 redis:7-alpine)
[SERVER] ⚠️  Orchestration services disabled - repositories not available
[SERVER]    New API endpoints (/api/v1/*) will not be registered
[SERVER]    Legacy endpoints will continue to work
```

---

## Troubleshooting

### Redis Connection Issues

**Error:** `dial tcp [::1]:6379: connect: connection refused`

**Solution:**
```bash
# Check if Redis is running
docker ps | grep redis

# Start Redis if not running
docker run -d -p 6379:6379 --name redis redis:7-alpine

# Test connection
redis-cli ping
```

### ChromaDB Connection Issues

**Error:** `Post "http://localhost:8000/api/v1/heartbeat": dial tcp [::1]:8000: connect: connection refused`

**Solution:**
```bash
# Check if ChromaDB is running
docker ps | grep chroma

# Start ChromaDB if not running
docker run -d -p 8000:8000 --name chromadb chromadb/chroma

# Test connection
curl http://localhost:8000/api/v1/heartbeat
```

### Python Backend Connection Issues

**Error:** `Failed to connect to Python backend`

**Solution:**
```bash
# Check if Python backend is running
curl http://localhost:8001/health

# Check environment variable
echo $PYTHON_BACKEND_URL

# Update if needed
export PYTHON_BACKEND_URL="http://localhost:8001"
```

---

## Security Considerations

### Production Deployment

1. **Use TLS/SSL** for all inter-service communication
2. **Enable Redis AUTH** with strong passwords
3. **Use network policies** to restrict access between services
4. **Store secrets** in a secure vault (AWS Secrets Manager, HashiCorp Vault, etc.)
5. **Enable audit logging** for all database operations
6. **Use read replicas** for Redis if needed for scalability
7. **Implement rate limiting** on public endpoints
8. **Enable CORS** only for trusted origins

### Example Production Configuration

```bash
# Python Backend (internal network only)
PYTHON_BACKEND_URL=https://python-backend.internal:8001

# Redis (managed, encrypted, password-protected)
REDIS_HOST=redis-cluster.internal
REDIS_PORT=6380
REDIS_PASSWORD=${REDIS_PASSWORD_FROM_VAULT}
REDIS_POOL_SIZE=100

# ChromaDB (internal network, mTLS)
CHROMA_HOST=chromadb.internal
CHROMA_PORT=8000

# Server
PORT=8080
GO_ENV=production
```

---

## Monitoring

### Health Check Endpoints

- **Go Backend**: `GET /health`
- **Python Backend**: `GET /health`
- **Redis**: `redis-cli PING`
- **ChromaDB**: `GET /api/v1/heartbeat`

### Metrics to Monitor

1. **Redis**:
   - Connection pool stats
   - Memory usage
   - Command latency
   - Key count by prefix

2. **ChromaDB**:
   - Collection count
   - Document count per collection
   - Query latency
   - Memory usage

3. **Go Backend**:
   - Request rate
   - Error rate
   - Response time (p50, p95, p99)
   - Active connections

---

## References

- [Redis Documentation](https://redis.io/docs/)
- [ChromaDB Documentation](https://docs.trychroma.com/)
- [Go Redis Client](https://github.com/redis/go-redis)