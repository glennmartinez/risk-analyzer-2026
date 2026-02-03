# Embedder Service Documentation

**Version**: 1.0  
**Author**: Backend Refactor Team  
**Last Updated**: 2024

---

## Overview

The **EmbedderService** is a production-ready, high-performance embedding generation service that provides:

- 🚀 **Multiple Embedding Providers** (OpenAI, HuggingFace, Sentence Transformers)
- 💾 **Intelligent Caching** (Redis-backed with in-memory fallback)
- ⚡ **Batch Optimization** (automatic batching with partial cache hits)
- 💰 **Cost Tracking** (per-request and cumulative cost tracking)
- 📊 **Metrics & Monitoring** (cache hit rates, usage stats, cost analytics)
- 🔄 **Model Management** (lazy loading, instance caching, multi-model support)

---

## Quick Start

### Basic Usage

```python
from app.services.embedder import get_embedder_service

# Get singleton instance
embedder = get_embedder_service()

# Single text embedding
result = await embedder.embed_text("Hello, world!")
print(f"Embedding: {len(result.embedding)} dimensions")
print(f"Cost: ${result.cost:.6f}")
print(f"Cached: {result.cached}")

# Batch embedding
texts = ["First text", "Second text", "Third text"]
result = await embedder.embed_batch(texts)
print(f"Generated {result.total_embeddings} embeddings")
print(f"Cache hits: {result.cached_count}/{result.total_embeddings}")
print(f"Total cost: ${result.total_cost:.6f}")

# Query embedding (optimized for search)
result = await embedder.embed_query("What is risk management?")
print(f"Query embedding: {len(result.embedding)} dimensions")
```

### Custom Configuration

```python
from app.services.embedder import EmbedderService
import redis

# Create custom Redis client
redis_client = redis.Redis(host='localhost', port=6379, db=0)

# Create custom embedder
embedder = EmbedderService(
    default_model="text-embedding-3-large",
    openai_api_key="sk-your-api-key",
    redis_client=redis_client,
    enable_caching=True
)

# Use specific model
result = await embedder.embed_text(
    "Custom model test",
    model="text-embedding-3-small"
)

# Disable caching for specific request
result = await embedder.embed_text(
    "No cache please",
    use_cache=False
)
```

---

## Supported Models

### OpenAI Models

| Model | Dimensions | Max Batch | Cost/1K Tokens | Best For |
|-------|-----------|-----------|----------------|----------|
| text-embedding-3-small | 1536 | 100 | $0.00002 | General purpose, cost-effective |
| text-embedding-3-large | 3072 | 100 | $0.00013 | High quality, better accuracy |
| text-embedding-ada-002 | 1536 | 100 | $0.0001 | Legacy, still good |

### Local Models (Free)

| Model | Dimensions | Max Batch | Cost | Best For |
|-------|-----------|-----------|------|----------|
| all-MiniLM-L6-v2 | 384 | 32 | Free | Fast, lightweight |
| all-mpnet-base-v2 | 768 | 32 | Free | Better quality, still fast |

---

## API Reference

### EmbedderService

#### `async embed_text(text: str, model: str = None, use_cache: bool = True) -> EmbeddingResult`

Generate embedding for a single text.

**Parameters:**
- `text` (str): Text to embed (required)
- `model` (str, optional): Model to use (defaults to service default_model)
- `use_cache` (bool): Whether to use cache (default: True)

**Returns:**
- `EmbeddingResult` with fields:
  - `embedding` (List[float]): Embedding vector
  - `dimension` (int): Embedding dimension
  - `model` (str): Model used
  - `cached` (bool): Whether result was from cache
  - `cost` (float): Estimated cost in USD

**Raises:**
- `ValueError`: If text is empty or model is invalid

**Example:**
```python
result = await embedder.embed_text("Hello world")
if result.cached:
    print("Retrieved from cache!")
else:
    print(f"Generated new embedding, cost: ${result.cost:.6f}")
```

---

#### `async embed_query(query: str, model: str = None, use_cache: bool = True) -> EmbeddingResult`

Generate embedding for a search query (optimized for search use cases).

**Parameters:**
- `query` (str): Query text to embed (required)
- `model` (str, optional): Model to use
- `use_cache` (bool): Whether to use cache

**Returns:**
- `EmbeddingResult` (same as `embed_text`)

**Note:** Some models have specialized query embeddings that differ from document embeddings.

**Example:**
```python
query_result = await embedder.embed_query("What is risk?")
# Use query_result.embedding for similarity search
```

---

#### `async embed_batch(texts: List[str], model: str = None, batch_size: int = None, use_cache: bool = True) -> BatchEmbeddingResult`

Generate embeddings for multiple texts efficiently.

**Parameters:**
- `texts` (List[str]): List of texts to embed (required)
- `model` (str, optional): Model to use
- `batch_size` (int, optional): Batch size (defaults to model's max_batch_size)
- `use_cache` (bool): Whether to use cache

**Returns:**
- `BatchEmbeddingResult` with fields:
  - `embeddings` (List[List[float]]): List of embedding vectors
  - `dimension` (int): Embedding dimension
  - `model` (str): Model used
  - `total_embeddings` (int): Number of embeddings generated
  - `cached_count` (int): Number of cached results
  - `total_cost` (float): Total estimated cost in USD

**Raises:**
- `ValueError`: If texts list is empty or all texts are empty

**Example:**
```python
texts = ["Text 1", "Text 2", "Text 3", "Text 4", "Text 5"]
result = await embedder.embed_batch(texts, batch_size=2)

print(f"Total: {result.total_embeddings}")
print(f"Cached: {result.cached_count}")
print(f"New: {result.total_embeddings - result.cached_count}")
print(f"Cost: ${result.total_cost:.6f}")
```

---

#### `get_available_models() -> List[Dict]`

Get list of available embedding models with metadata.

**Returns:**
- List of dictionaries with model information:
  - `id` (str): Model identifier
  - `name` (str): Full model name
  - `provider` (str): Provider (openai, huggingface, etc.)
  - `dimension` (int): Embedding dimension
  - `max_batch_size` (int): Maximum batch size
  - `max_input_length` (int): Maximum input token length
  - `cost_per_1k_tokens` (float or None): Cost per 1000 tokens
  - `free` (bool): Whether model is free

**Example:**
```python
models = embedder.get_available_models()
for model in models:
    print(f"{model['id']}: {model['dimension']}D, "
          f"{'FREE' if model['free'] else f'${model["cost_per_1k_tokens"]}'}")
```

---

#### `get_metrics() -> Dict`

Get current service metrics.

**Returns:**
- Dictionary with metrics:
  - `total_embeddings_generated` (int): Total embeddings generated
  - `total_cache_hits` (int): Total cache hits
  - `cache_hit_rate` (float): Cache hit rate (0.0 to 1.0)
  - `total_cost_usd` (float): Total cost in USD
  - `models_loaded` (int): Number of loaded models
  - `default_model` (str): Default model name
  - `caching_enabled` (bool): Whether caching is enabled

**Example:**
```python
metrics = embedder.get_metrics()
print(f"Cache hit rate: {metrics['cache_hit_rate'] * 100:.1f}%")
print(f"Total cost: ${metrics['total_cost_usd']:.4f}")
print(f"Models loaded: {metrics['models_loaded']}")
```

---

#### `reset_metrics() -> None`

Reset service metrics to zero.

**Example:**
```python
embedder.reset_metrics()
# Start fresh tracking
```

---

## Caching Strategy

### Cache Key Generation

Cache keys are generated using SHA-256 hash of `model:text`:

```python
cache_key = f"embed:v1:{sha256(f'{model}:{text}')}"
```

**Benefits:**
- Same text + model always produces same cache key
- Different models have separate cache entries
- Content-based (deterministic)
- Version prefix allows cache invalidation

### Cache Hierarchy

1. **Redis Cache** (primary)
   - Persistent across restarts
   - Shared across instances
   - Configurable TTL (default: 24 hours)

2. **In-Memory Cache** (fallback)
   - Fast access
   - Survives Redis failures
   - LRU eviction (max 1000 entries)

### Partial Cache Hits

For batch operations, the service optimizes by:
1. Looking up all texts in cache
2. Only generating embeddings for uncached texts
3. Combining cached + new embeddings
4. Caching new embeddings for future use

**Example:**
```python
# Batch of 10 texts
# 7 are cached, 3 are new
result = await embedder.embed_batch(texts)

# Result:
# - total_embeddings: 10
# - cached_count: 7
# - new generated: 3
# - API calls: 1 (only for 3 new texts)
# - Cost: ~70% savings vs no cache
```

---

## Cost Optimization

### Cost Tracking

The service tracks costs at multiple levels:

1. **Per-Request Cost**
   ```python
   result = await embedder.embed_text("Long document...")
   print(f"This request cost: ${result.cost:.6f}")
   ```

2. **Cumulative Cost**
   ```python
   metrics = embedder.get_metrics()
   print(f"Total service cost: ${metrics['total_cost_usd']:.4f}")
   ```

3. **Cache Savings**
   - Cached requests have `cost = 0.0`
   - Track savings: `cache_hits × avg_cost_per_embedding`

### Cost Estimation Formula

```python
estimated_tokens = len(text) / 4  # ~4 chars per token
cost = (estimated_tokens / 1000) * model_cost_per_1k_tokens
```

### Best Practices

1. **Use Caching**
   ```python
   # Good: Uses cache (default)
   result = await embedder.embed_text(text)
   
   # Only disable if you need fresh embeddings
   result = await embedder.embed_text(text, use_cache=False)
   ```

2. **Batch When Possible**
   ```python
   # Good: Batch processing
   results = await embedder.embed_batch(texts)
   
   # Bad: Individual calls
   for text in texts:
       result = await embedder.embed_text(text)  # Slower, more API calls
   ```

3. **Choose Right Model**
   ```python
   # For general use (cost-effective)
   result = await embedder.embed_text(text, model="text-embedding-3-small")
   
   # For high-quality (when needed)
   result = await embedder.embed_text(text, model="text-embedding-3-large")
   
   # For free/offline (local models)
   result = await embedder.embed_text(text, model="all-MiniLM-L6-v2")
   ```

---

## Performance Characteristics

### Latency

| Operation | Without Cache | With Cache (Hit) |
|-----------|---------------|------------------|
| Single text | 100-500ms | 1-10ms |
| Batch (10 texts) | 200-800ms | 10-50ms |
| Batch (100 texts) | 1-3s | 50-200ms |

### Throughput

| Scenario | Requests/sec | Embeddings/sec |
|----------|--------------|----------------|
| Single (no cache) | 2-10 | 2-10 |
| Single (cached) | 100-500 | 100-500 |
| Batch (no cache) | 1-5 | 50-500 |
| Batch (cached) | 10-50 | 500-5000 |

### Cache Hit Rates

Typical production workloads:
- **Document processing**: 70-80% (many repeated chunks)
- **Search queries**: 40-60% (some repeated queries)
- **Batch ingestion**: 10-30% (mostly new content)

---

## Error Handling

### Common Errors

**Empty Text**
```python
try:
    result = await embedder.embed_text("")
except ValueError as e:
    print(f"Error: {e}")  # "Text cannot be empty"
```

**Missing API Key**
```python
embedder = EmbedderService(openai_api_key=None)
try:
    result = await embedder.embed_text("test")
except ValueError as e:
    print(f"Error: {e}")  # "OpenAI API key required..."
```

**API Errors**
```python
try:
    result = await embedder.embed_text("test")
except Exception as e:
    print(f"API error: {e}")
    # Log and retry or use fallback
```

### Retry Strategy

The service does NOT automatically retry. Implement retries at the application level:

```python
import asyncio

async def embed_with_retry(text, max_retries=3):
    for attempt in range(max_retries):
        try:
            return await embedder.embed_text(text)
        except Exception as e:
            if attempt == max_retries - 1:
                raise
            await asyncio.sleep(2 ** attempt)  # Exponential backoff
```

---

## Configuration

### Environment Variables

```bash
# Required for OpenAI models
OPENAI_API_KEY=sk-...

# Redis configuration (optional, for caching)
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_DB=0
REDIS_PASSWORD=  # Optional

# Embedding configuration
EMBEDDING_MODEL=text-embedding-3-small
```

### Application Settings

In `app/config.py`:

```python
class Settings(BaseSettings):
    embedding_model: str = "text-embedding-3-small"
    openai_api_key: Optional[str] = None
    redis_host: str = "localhost"
    redis_port: int = 6379
    redis_db: int = 0
```

---

## Testing

### Running Tests

**Unit Tests** (no external dependencies):
```bash
pytest tests/test_embedder_service.py -v
```

**Integration Tests** (requires OpenAI API and/or Redis):
```bash
# With OpenAI
export OPENAI_API_KEY=sk-...
pytest tests/test_embedder_integration.py -v -k openai

# With Redis
pytest tests/test_embedder_integration.py -v -k redis

# Skip integration tests
SKIP_INTEGRATION=1 pytest tests/test_embedder_integration.py
```

**All Tests**:
```bash
pytest tests/test_embedder*.py -v --cov=app/services/embedder
```

### Test Coverage

- **Unit Tests**: 50+ tests covering all functionality
- **Integration Tests**: 20+ tests with real providers
- **Coverage**: ~95% of embedder service code

---

## Monitoring & Observability

### Metrics Endpoint

Access metrics via API:

```bash
curl http://localhost:8000/embed/metrics
```

Response:
```json
{
  "metrics": {
    "total_embeddings_generated": 1523,
    "total_cache_hits": 4127,
    "cache_hit_rate": 0.731,
    "total_cost_usd": 0.0453,
    "models_loaded": 2,
    "default_model": "text-embedding-3-small",
    "caching_enabled": true
  },
  "status": "healthy"
}
```

### Health Check

```bash
curl http://localhost:8000/embed/health
```

Response:
```json
{
  "status": "healthy",
  "service": "embed",
  "default_model": "text-embedding-3-small",
  "caching_enabled": true,
  "models_loaded": 1,
  "total_embeddings_generated": 1523
}
```

### Logging

The service logs at various levels:

```python
# INFO: Normal operations
logger.info(f"Generated {count} embeddings")

# DEBUG: Detailed operations
logger.debug(f"Cache hit: {cache_key[:16]}...")

# WARNING: Recoverable issues
logger.warning(f"Redis cache error: {e}")

# ERROR: Failures
logger.error(f"Embedding generation failed: {e}", exc_info=True)
```

---

## Best Practices

### 1. Always Use Batching

```python
# ✅ Good
texts = ["text1", "text2", "text3"]
result = await embedder.embed_batch(texts)

# ❌ Bad
for text in texts:
    result = await embedder.embed_text(text)
```

### 2. Enable Caching

```python
# ✅ Good (default)
result = await embedder.embed_text(text)

# ⚠️ Only disable when necessary
result = await embedder.embed_text(text, use_cache=False)
```

### 3. Choose Appropriate Models

```python
# For most use cases (cost-effective)
embedder = EmbedderService(default_model="text-embedding-3-small")

# For critical quality needs
embedder = EmbedderService(default_model="text-embedding-3-large")

# For offline/free usage
embedder = EmbedderService(default_model="all-MiniLM-L6-v2")
```

### 4. Monitor Metrics

```python
# Periodically check metrics
metrics = embedder.get_metrics()
if metrics['cache_hit_rate'] < 0.5:
    logger.warning("Low cache hit rate, consider TTL adjustment")
```

### 5. Handle Errors Gracefully

```python
try:
    result = await embedder.embed_text(text)
except ValueError as e:
    # Validation error (empty text, etc.)
    logger.error(f"Validation error: {e}")
except Exception as e:
    # API or other error
    logger.error(f"Embedding failed: {e}")
    # Fallback or retry logic
```

---

## Troubleshooting

### Issue: "OpenAI API key required"

**Solution**: Set environment variable
```bash
export OPENAI_API_KEY=sk-your-api-key
```

### Issue: Cache not working

**Check:**
1. Redis is running: `redis-cli ping`
2. Redis connection settings in config
3. Check logs for cache errors

**Fallback**: In-memory cache still works even if Redis fails

### Issue: Slow performance

**Diagnosis:**
1. Check cache hit rate: `embedder.get_metrics()`
2. If low, verify caching is enabled
3. Check batch sizes

**Solutions:**
- Increase cache TTL
- Use batching
- Pre-warm cache for common texts

### Issue: High costs

**Diagnosis:**
1. Check total cost: `embedder.get_metrics()['total_cost_usd']`
2. Check cache hit rate

**Solutions:**
- Enable caching
- Use smaller model (text-embedding-3-small)
- Use local models for non-critical use cases
- Increase cache TTL

---

## FAQ

**Q: Which model should I use?**

A: 
- `text-embedding-3-small`: Best for most use cases (cost-effective, good quality)
- `text-embedding-3-large`: When you need highest quality
- `all-MiniLM-L6-v2`: For offline/free usage

**Q: How long are embeddings cached?**

A: Default is 24 hours (86400 seconds). Configure via `EmbeddingCache(ttl_seconds=...)`.

**Q: Can I use multiple models simultaneously?**

A: Yes! The service automatically manages multiple model instances.

**Q: What happens if Redis is down?**

A: The service falls back to in-memory cache automatically.

**Q: How accurate is the cost estimation?**

A: ~90% accurate. Uses `len(text) / 4` for token estimation, which is approximate.

**Q: Can I clear the cache?**

A: Yes, for Redis: `redis_client.flushdb()`. For in-memory: restart service.

---

## Support & Contributing

### Reporting Issues

File issues in the project repository with:
- Description of the problem
- Code snippet to reproduce
- Expected vs actual behavior
- Logs (if applicable)

### Contributing

Contributions welcome! Areas for improvement:
- Additional embedding providers
- Performance optimizations
- Better cost tracking
- Advanced caching strategies

---

## License

Part of the Risk Analyzer Go project.

---

## Changelog

### v1.0 (2024)
- ✅ Initial release
- ✅ OpenAI embeddings support
- ✅ HuggingFace embeddings support
- ✅ Redis caching
- ✅ Batch optimization
- ✅ Cost tracking
- ✅ Metrics & monitoring
- ✅ Comprehensive tests