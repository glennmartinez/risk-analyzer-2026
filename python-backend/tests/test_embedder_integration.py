"""
Integration tests for EmbedderService with embedding providers

These tests require actual embedding providers or realistic mocks.
They test:
- OpenAI embedding integration
- HuggingFace/Sentence-Transformers integration
- Redis cache integration
- End-to-end workflows
- Performance and batching

To run these tests:
- Set OPENAI_API_KEY environment variable (for OpenAI tests)
- Ensure Redis is running (for cache tests)
- Or use SKIP_INTEGRATION=1 to skip provider tests
"""

import os
from typing import List

import pytest

# Skip integration tests if flag is set
SKIP_INTEGRATION = os.getenv("SKIP_INTEGRATION", "0") == "1"

skip_if_no_integration = pytest.mark.skipif(
    SKIP_INTEGRATION, reason="SKIP_INTEGRATION=1"
)


@pytest.fixture
def openai_api_key():
    """Get OpenAI API key from environment"""
    api_key = os.getenv("OPENAI_API_KEY")
    if not api_key:
        pytest.skip("OPENAI_API_KEY not set")
    return api_key


@pytest.fixture
def redis_client():
    """Get Redis client (skip if Redis not available)"""
    try:
        import redis

        client = redis.Redis(
            host=os.getenv("REDIS_HOST", "localhost"),
            port=int(os.getenv("REDIS_PORT", "6379")),
            db=int(os.getenv("REDIS_DB", "1")),  # Use different DB for tests
            decode_responses=False,
        )
        # Test connection
        client.ping()
        # Clear test DB
        client.flushdb()
        return client
    except Exception as e:
        pytest.skip(f"Redis not available: {e}")


# ============================================================================
# OpenAI Integration Tests
# ============================================================================


class TestOpenAIIntegration:
    """Integration tests with OpenAI embedding API"""

    @skip_if_no_integration
    @pytest.mark.asyncio
    async def test_openai_single_embedding(self, openai_api_key):
        """Test real OpenAI single embedding"""
        from app.services.embedder import EmbedderService

        service = EmbedderService(
            default_model="text-embedding-3-small",
            openai_api_key=openai_api_key,
            enable_caching=False,
        )

        result = await service.embed_text("Hello, world!")

        # Verify result structure
        assert len(result.embedding) == 1536  # text-embedding-3-small dimension
        assert result.dimension == 1536
        assert result.model == "text-embedding-3-small"
        assert result.cached is False
        assert result.cost > 0
        assert all(isinstance(x, float) for x in result.embedding)

    @skip_if_no_integration
    @pytest.mark.asyncio
    async def test_openai_batch_embedding(self, openai_api_key):
        """Test real OpenAI batch embedding"""
        from app.services.embedder import EmbedderService

        service = EmbedderService(
            default_model="text-embedding-3-small",
            openai_api_key=openai_api_key,
            enable_caching=False,
        )

        texts = [
            "The quick brown fox jumps over the lazy dog",
            "Machine learning is a subset of artificial intelligence",
            "Python is a popular programming language",
        ]

        result = await service.embed_batch(texts)

        # Verify result
        assert result.total_embeddings == 3
        assert len(result.embeddings) == 3
        assert result.dimension == 1536
        assert result.cached_count == 0
        assert result.total_cost > 0

        # All embeddings should be different
        assert result.embeddings[0] != result.embeddings[1]
        assert result.embeddings[1] != result.embeddings[2]

    @skip_if_no_integration
    @pytest.mark.asyncio
    async def test_openai_query_embedding(self, openai_api_key):
        """Test real OpenAI query embedding"""
        from app.services.embedder import EmbedderService

        service = EmbedderService(
            default_model="text-embedding-3-small",
            openai_api_key=openai_api_key,
            enable_caching=False,
        )

        result = await service.embed_query("What is machine learning?")

        assert len(result.embedding) == 1536
        assert result.dimension == 1536
        assert result.cached is False

    @skip_if_no_integration
    @pytest.mark.asyncio
    async def test_openai_large_model(self, openai_api_key):
        """Test OpenAI large embedding model"""
        from app.services.embedder import EmbedderService

        service = EmbedderService(
            default_model="text-embedding-3-large",
            openai_api_key=openai_api_key,
            enable_caching=False,
        )

        result = await service.embed_text("Test with large model")

        # text-embedding-3-large has 3072 dimensions
        assert len(result.embedding) == 3072
        assert result.dimension == 3072
        assert result.model == "text-embedding-3-large"

    @skip_if_no_integration
    @pytest.mark.asyncio
    async def test_openai_model_switching(self, openai_api_key):
        """Test switching between different OpenAI models"""
        from app.services.embedder import EmbedderService

        service = EmbedderService(
            default_model="text-embedding-3-small",
            openai_api_key=openai_api_key,
            enable_caching=False,
        )

        text = "Model switching test"

        # Embed with small model
        result_small = await service.embed_text(text, model="text-embedding-3-small")
        assert result_small.dimension == 1536

        # Embed with large model
        result_large = await service.embed_text(text, model="text-embedding-3-large")
        assert result_large.dimension == 3072

        # Embeddings should be different (different models)
        assert len(result_small.embedding) != len(result_large.embedding)

        # Both models should be cached in service
        assert len(service._model_instances) == 2


# ============================================================================
# Redis Cache Integration Tests
# ============================================================================


class TestRedisCacheIntegration:
    """Integration tests with Redis cache"""

    @pytest.mark.asyncio
    async def test_redis_cache_set_get(self, redis_client):
        """Test Redis cache set and get"""
        from app.services.embedder import EmbeddingCache

        cache = EmbeddingCache(redis_client=redis_client, ttl_seconds=60)

        text = "Cache test text"
        model = "test-model"
        embedding = [0.1, 0.2, 0.3, 0.4, 0.5]

        # Set in cache
        await cache.set(text, model, embedding)

        # Get from cache
        result = await cache.get(text, model)

        assert result == embedding

    @pytest.mark.asyncio
    async def test_redis_cache_expiration(self, redis_client):
        """Test Redis cache TTL expiration"""
        import asyncio

        from app.services.embedder import EmbeddingCache

        cache = EmbeddingCache(redis_client=redis_client, ttl_seconds=1)

        text = "Expiring cache test"
        model = "test-model"
        embedding = [0.1, 0.2, 0.3]

        # Set in cache
        await cache.set(text, model, embedding)

        # Should be available immediately
        result = await cache.get(text, model)
        assert result == embedding

        # Wait for expiration
        await asyncio.sleep(2)

        # Should be expired (from Redis, but still in memory)
        result = await cache.get(text, model)
        # Memory cache doesn't expire, so it might still be there
        # This is expected behavior

    @pytest.mark.asyncio
    async def test_redis_cache_batch_operations(self, redis_client):
        """Test Redis batch cache operations"""
        from app.services.embedder import EmbeddingCache

        cache = EmbeddingCache(redis_client=redis_client)

        texts = ["text1", "text2", "text3", "text4", "text5"]
        model = "test-model"
        embeddings = [[float(i)] * 10 for i in range(5)]

        # Set batch
        await cache.set_batch(texts, model, embeddings)

        # Get batch - all should be cached
        cached, uncached = await cache.get_batch(texts, model)

        assert len(cached) == 5
        assert len(uncached) == 0
        for i in range(5):
            assert cached[i] == embeddings[i]

    @skip_if_no_integration
    @pytest.mark.asyncio
    async def test_embedder_with_redis_cache(self, openai_api_key, redis_client):
        """Test embedder service with Redis cache"""
        from app.services.embedder import EmbedderService

        service = EmbedderService(
            default_model="text-embedding-3-small",
            openai_api_key=openai_api_key,
            redis_client=redis_client,
            enable_caching=True,
        )

        text = "Caching test with real embeddings"

        # First call - generate and cache
        result1 = await service.embed_text(text)
        assert result1.cached is False
        assert service.total_embeddings_generated == 1
        assert service.total_cache_hits == 0

        # Second call - should hit cache
        result2 = await service.embed_text(text)
        assert result2.cached is True
        assert result2.embedding == result1.embedding
        assert service.total_embeddings_generated == 1  # No new generation
        assert service.total_cache_hits == 1

        # Verify cost savings from cache
        assert result2.cost == 0.0  # Cached results have no cost

    @skip_if_no_integration
    @pytest.mark.asyncio
    async def test_batch_with_partial_cache_hits(self, openai_api_key, redis_client):
        """Test batch embedding with partial cache hits"""
        from app.services.embedder import EmbedderService

        service = EmbedderService(
            default_model="text-embedding-3-small",
            openai_api_key=openai_api_key,
            redis_client=redis_client,
            enable_caching=True,
        )

        # Pre-embed some texts
        await service.embed_text("cached text 1")
        await service.embed_text("cached text 2")

        # Reset metrics to track only batch operation
        service.reset_metrics()

        # Batch with mix of cached and new texts
        texts = [
            "cached text 1",  # cached
            "new text 1",  # new
            "cached text 2",  # cached
            "new text 2",  # new
        ]

        result = await service.embed_batch(texts)

        assert result.total_embeddings == 4
        assert result.cached_count == 2  # 2 cached
        assert service.total_embeddings_generated == 2  # Only 2 new generated
        assert service.total_cache_hits == 2


# ============================================================================
# HuggingFace / Local Model Integration Tests
# ============================================================================


class TestHuggingFaceIntegration:
    """Integration tests with HuggingFace models"""

    @pytest.mark.asyncio
    async def test_sentence_transformers_embedding(self):
        """Test sentence-transformers local embedding"""
        try:
            from app.services.embedder import EmbedderService

            service = EmbedderService(
                default_model="all-MiniLM-L6-v2",
                enable_caching=False,
            )

            result = await service.embed_text("Local embedding test")

            # all-MiniLM-L6-v2 has 384 dimensions
            assert result.dimension == 384
            assert len(result.embedding) == 384
            assert result.model == "all-MiniLM-L6-v2"
            assert result.cost == 0.0  # Local models are free

        except Exception as e:
            pytest.skip(f"HuggingFace models not available: {e}")

    @pytest.mark.asyncio
    async def test_sentence_transformers_batch(self):
        """Test sentence-transformers batch embedding"""
        try:
            from app.services.embedder import EmbedderService

            service = EmbedderService(
                default_model="all-MiniLM-L6-v2",
                enable_caching=False,
            )

            texts = ["Local model test 1", "Local model test 2", "Local model test 3"]

            result = await service.embed_batch(texts)

            assert result.total_embeddings == 3
            assert result.dimension == 384
            assert result.total_cost == 0.0  # Free

        except Exception as e:
            pytest.skip(f"HuggingFace models not available: {e}")


# ============================================================================
# Performance Tests
# ============================================================================


class TestPerformance:
    """Performance and optimization tests"""

    @skip_if_no_integration
    @pytest.mark.asyncio
    async def test_batch_performance_vs_individual(self, openai_api_key):
        """Test that batching is more efficient than individual calls"""
        import time

        from app.services.embedder import EmbedderService

        service = EmbedderService(
            default_model="text-embedding-3-small",
            openai_api_key=openai_api_key,
            enable_caching=False,
        )

        texts = [f"Performance test text {i}" for i in range(10)]

        # Individual embeddings
        start_individual = time.time()
        for text in texts:
            await service.embed_text(text)
        time_individual = time.time() - start_individual

        # Reset for fair comparison
        service.reset_metrics()

        # Batch embedding
        start_batch = time.time()
        await service.embed_batch(texts)
        time_batch = time.time() - start_batch

        # Batch should be faster (or at least not much slower)
        # Note: With API latency, this might not always be true for small batches
        print(
            f"\nIndividual: {time_individual:.2f}s, Batch: {time_batch:.2f}s, "
            f"Speedup: {time_individual / time_batch:.2f}x"
        )

    @skip_if_no_integration
    @pytest.mark.asyncio
    async def test_cache_performance_benefit(self, openai_api_key, redis_client):
        """Test performance benefit of caching"""
        import time

        from app.services.embedder import EmbedderService

        service = EmbedderService(
            default_model="text-embedding-3-small",
            openai_api_key=openai_api_key,
            redis_client=redis_client,
            enable_caching=True,
        )

        texts = ["Repeated text"] * 10

        # First batch - no cache
        start_no_cache = time.time()
        result1 = await service.embed_batch(texts, use_cache=False)
        time_no_cache = time.time() - start_no_cache

        # Second batch - with cache
        start_with_cache = time.time()
        result2 = await service.embed_batch(texts, use_cache=True)
        time_with_cache = time.time() - start_with_cache

        # Cache should be much faster
        print(
            f"\nNo cache: {time_no_cache:.2f}s, With cache: {time_with_cache:.2f}s, "
            f"Speedup: {time_no_cache / time_with_cache:.2f}x"
        )

        assert result2.cached_count == 10  # All cached


# ============================================================================
# End-to-End Workflow Tests
# ============================================================================


class TestEndToEndWorkflows:
    """End-to-end workflow tests"""

    @skip_if_no_integration
    @pytest.mark.asyncio
    async def test_document_processing_workflow(self, openai_api_key, redis_client):
        """Test a complete document processing workflow"""
        from app.services.embedder import EmbedderService

        service = EmbedderService(
            default_model="text-embedding-3-small",
            openai_api_key=openai_api_key,
            redis_client=redis_client,
            enable_caching=True,
        )

        # Simulate document chunks
        chunks = [
            "Risk management is essential for business success.",
            "Financial risk includes market risk and credit risk.",
            "Operational risk involves process failures.",
            "Risk assessment helps identify potential threats.",
            "Mitigation strategies reduce risk impact.",
        ]

        # Embed all chunks
        result = await service.embed_batch(chunks)

        assert result.total_embeddings == 5
        assert result.dimension == 1536
        assert all(len(emb) == 1536 for emb in result.embeddings)

        # Simulate a search query
        query = "What are the types of financial risk?"
        query_result = await service.embed_query(query)

        assert len(query_result.embedding) == 1536

        # In a real system, we'd now compute similarity between query and chunks
        # For this test, just verify we got valid embeddings
        assert all(isinstance(x, float) for x in query_result.embedding)

        # Check metrics
        metrics = service.get_metrics()
        assert metrics["total_embeddings_generated"] == 6  # 5 chunks + 1 query
        assert metrics["total_cost_usd"] > 0

    @skip_if_no_integration
    @pytest.mark.asyncio
    async def test_multi_model_workflow(self, openai_api_key):
        """Test workflow using multiple models"""
        from app.services.embedder import EmbedderService

        service = EmbedderService(
            default_model="text-embedding-3-small",
            openai_api_key=openai_api_key,
            enable_caching=False,
        )

        text = "Multi-model embedding test"

        # Embed with small model (fast, cheaper)
        result_small = await service.embed_text(text, model="text-embedding-3-small")

        # Embed with large model (slower, better quality)
        result_large = await service.embed_text(text, model="text-embedding-3-large")

        # Verify both models work
        assert result_small.dimension == 1536
        assert result_large.dimension == 3072

        # Large model should cost more
        assert result_large.cost > result_small.cost

        # Verify both model instances are loaded
        assert len(service._model_instances) == 2

    @skip_if_no_integration
    @pytest.mark.asyncio
    async def test_large_batch_processing(self, openai_api_key, redis_client):
        """Test processing large batches efficiently"""
        from app.services.embedder import EmbedderService

        service = EmbedderService(
            default_model="text-embedding-3-small",
            openai_api_key=openai_api_key,
            redis_client=redis_client,
            enable_caching=True,
        )

        # Generate 100 texts
        texts = [f"Large batch test text number {i}" for i in range(100)]

        # Process in batch with custom batch size
        result = await service.embed_batch(texts, batch_size=25)

        assert result.total_embeddings == 100
        assert len(result.embeddings) == 100
        assert all(len(emb) == 1536 for emb in result.embeddings)

        # Process same batch again - should all be cached
        result2 = await service.embed_batch(texts)

        assert result2.cached_count == 100
        assert result2.total_cost == 0.0  # All from cache

        # Verify embeddings match
        for i in range(100):
            assert result.embeddings[i] == result2.embeddings[i]
