"""
Unit tests for EmbedderService

Tests cover:
- Single text embedding
- Query embedding
- Batch embedding with caching
- Model management and switching
- Cache hit/miss scenarios
- Error handling
- Metrics tracking
- Multiple providers (OpenAI, HuggingFace)
"""

import asyncio
from typing import List
from unittest.mock import AsyncMock, MagicMock, Mock, patch

import pytest

from app.services.embedder import (
    EMBEDDING_MODELS,
    BatchEmbeddingResult,
    EmbedderService,
    EmbeddingCache,
    EmbeddingModelConfig,
    EmbeddingProvider,
    EmbeddingResult,
)

# ============================================================================
# Fixtures
# ============================================================================


@pytest.fixture
def mock_redis():
    """Mock Redis client"""
    redis = MagicMock()
    redis.get = Mock(return_value=None)
    redis.setex = Mock()
    return redis


@pytest.fixture
def mock_openai_embedding():
    """Mock OpenAI embedding model"""
    mock = MagicMock()
    mock.get_text_embedding = Mock(return_value=[0.1] * 1536)
    mock.get_text_embedding_batch = Mock(return_value=[[0.1] * 1536, [0.2] * 1536])
    mock.get_query_embedding = Mock(return_value=[0.3] * 1536)
    return mock


@pytest.fixture
def mock_huggingface_embedding():
    """Mock HuggingFace embedding model"""
    mock = MagicMock()
    mock.get_text_embedding = Mock(return_value=[0.1] * 384)
    mock.get_text_embedding_batch = Mock(return_value=[[0.1] * 384, [0.2] * 384])
    return mock


@pytest.fixture
def embedder_service_no_cache():
    """EmbedderService without caching"""
    return EmbedderService(
        default_model="text-embedding-3-small",
        openai_api_key="test-key",
        redis_client=None,
        enable_caching=False,
    )


@pytest.fixture
def embedder_service_with_cache(mock_redis):
    """EmbedderService with Redis cache"""
    return EmbedderService(
        default_model="text-embedding-3-small",
        openai_api_key="test-key",
        redis_client=mock_redis,
        enable_caching=True,
    )


# ============================================================================
# EmbeddingCache Tests
# ============================================================================


class TestEmbeddingCache:
    """Test EmbeddingCache functionality"""

    @pytest.mark.asyncio
    async def test_cache_set_get_memory(self):
        """Test memory cache set and get"""
        cache = EmbeddingCache(redis_client=None)

        text = "test text"
        model = "text-embedding-3-small"
        embedding = [0.1, 0.2, 0.3]

        # Set in cache
        await cache.set(text, model, embedding)

        # Get from cache
        result = await cache.get(text, model)

        assert result == embedding

    @pytest.mark.asyncio
    async def test_cache_miss(self):
        """Test cache miss returns None"""
        cache = EmbeddingCache(redis_client=None)

        result = await cache.get("nonexistent", "model")

        assert result is None

    @pytest.mark.asyncio
    async def test_cache_different_models(self):
        """Test cache differentiates between models"""
        cache = EmbeddingCache(redis_client=None)

        text = "test"
        embedding1 = [0.1] * 1536
        embedding2 = [0.2] * 384

        await cache.set(text, "model1", embedding1)
        await cache.set(text, "model2", embedding2)

        result1 = await cache.get(text, "model1")
        result2 = await cache.get(text, "model2")

        assert result1 == embedding1
        assert result2 == embedding2

    @pytest.mark.asyncio
    async def test_cache_batch_operations(self):
        """Test batch cache get/set"""
        cache = EmbeddingCache(redis_client=None)

        texts = ["text1", "text2", "text3"]
        model = "test-model"
        embeddings = [[0.1] * 10, [0.2] * 10, [0.3] * 10]

        # Set batch
        await cache.set_batch(texts, model, embeddings)

        # Get batch - all should be cached
        cached, uncached = await cache.get_batch(texts, model)

        assert len(cached) == 3
        assert len(uncached) == 0
        assert cached[0] == embeddings[0]
        assert cached[1] == embeddings[1]
        assert cached[2] == embeddings[2]

    @pytest.mark.asyncio
    async def test_cache_batch_partial_hit(self):
        """Test batch cache with partial hits"""
        cache = EmbeddingCache(redis_client=None)

        texts = ["cached1", "uncached", "cached2"]
        model = "test-model"

        # Pre-cache some texts
        await cache.set("cached1", model, [0.1] * 10)
        await cache.set("cached2", model, [0.3] * 10)

        # Get batch
        cached, uncached = await cache.get_batch(texts, model)

        assert len(cached) == 2
        assert len(uncached) == 1
        assert 0 in cached  # cached1
        assert 2 in cached  # cached2
        assert 1 in uncached  # uncached

    @pytest.mark.asyncio
    async def test_cache_memory_limit(self):
        """Test memory cache respects size limit"""
        cache = EmbeddingCache(redis_client=None)
        cache._max_memory_cache_size = 10

        model = "test-model"

        # Add more than max size
        for i in range(15):
            await cache.set(f"text{i}", model, [float(i)] * 10)

        # Cache should have exactly max size
        assert len(cache._memory_cache) == cache._max_memory_cache_size

        # Oldest entries should be evicted (FIFO)
        # Last 10 should be present
        for i in range(5, 15):
            result = await cache.get(f"text{i}", model)
            assert result is not None


# ============================================================================
# EmbedderService Tests - Basic Operations
# ============================================================================


class TestEmbedderServiceBasic:
    """Test basic EmbedderService operations"""

    @pytest.mark.asyncio
    async def test_initialization(self):
        """Test service initialization"""
        service = EmbedderService(
            default_model="text-embedding-3-small",
            openai_api_key="test-key",
            enable_caching=False,
        )

        assert service.default_model == "text-embedding-3-small"
        assert service.openai_api_key == "test-key"
        assert service.cache is None
        assert service.total_embeddings_generated == 0

    @pytest.mark.asyncio
    async def test_initialization_with_cache(self, mock_redis):
        """Test service initialization with cache"""
        service = EmbedderService(
            default_model="text-embedding-3-small",
            openai_api_key="test-key",
            redis_client=mock_redis,
            enable_caching=True,
        )

        assert service.cache is not None
        assert service.cache.enabled is True

    def test_get_model_config(self, embedder_service_no_cache):
        """Test getting model configuration"""
        config = embedder_service_no_cache._get_model_config("text-embedding-3-small")

        assert config.name == "text-embedding-3-small"
        assert config.provider == EmbeddingProvider.OPENAI
        assert config.dimension == 1536

    def test_get_model_config_by_full_name(self, embedder_service_no_cache):
        """Test getting model config by full name"""
        config = embedder_service_no_cache._get_model_config("all-MiniLM-L6-v2")

        assert config.provider == EmbeddingProvider.SENTENCE_TRANSFORMERS
        assert config.dimension == 384

    def test_get_available_models(self, embedder_service_no_cache):
        """Test listing available models"""
        models = embedder_service_no_cache.get_available_models()

        assert len(models) > 0
        assert any(m["id"] == "text-embedding-3-small" for m in models)
        assert any(m["provider"] == "openai" for m in models)
        assert any(m["free"] is True for m in models)


# ============================================================================
# EmbedderService Tests - Single Embedding
# ============================================================================


class TestEmbedderServiceSingleEmbedding:
    """Test single text embedding generation"""

    @pytest.mark.asyncio
    @patch("app.services.embedder.OpenAIEmbedding")
    async def test_embed_text_basic(self, mock_openai_class, embedder_service_no_cache):
        """Test basic single text embedding"""
        # Setup mock
        mock_instance = MagicMock()
        mock_instance.get_text_embedding = Mock(return_value=[0.1] * 1536)
        mock_openai_class.return_value = mock_instance

        # Embed text
        result = await embedder_service_no_cache.embed_text("Hello world")

        assert isinstance(result, EmbeddingResult)
        assert len(result.embedding) == 1536
        assert result.dimension == 1536
        assert result.model == "text-embedding-3-small"
        assert result.cached is False
        assert result.cost > 0

    @pytest.mark.asyncio
    async def test_embed_text_empty_raises_error(self, embedder_service_no_cache):
        """Test that empty text raises ValueError"""
        with pytest.raises(ValueError, match="Text cannot be empty"):
            await embedder_service_no_cache.embed_text("")

        with pytest.raises(ValueError, match="Text cannot be empty"):
            await embedder_service_no_cache.embed_text("   ")

    @pytest.mark.asyncio
    @patch("app.services.embedder.OpenAIEmbedding")
    async def test_embed_text_with_caching(
        self, mock_openai_class, embedder_service_with_cache
    ):
        """Test embedding with cache"""
        # Setup mock
        mock_instance = MagicMock()
        mock_instance.get_text_embedding = Mock(return_value=[0.1] * 1536)
        mock_openai_class.return_value = mock_instance

        text = "Cache test"

        # First call - should generate and cache
        result1 = await embedder_service_with_cache.embed_text(text)
        assert result1.cached is False
        assert embedder_service_with_cache.total_embeddings_generated == 1

        # Second call - should hit cache
        result2 = await embedder_service_with_cache.embed_text(text)
        assert result2.cached is True
        assert result2.embedding == result1.embedding
        assert embedder_service_with_cache.total_cache_hits == 1
        # Should not generate new embedding
        assert embedder_service_with_cache.total_embeddings_generated == 1

    @pytest.mark.asyncio
    @patch("app.services.embedder.OpenAIEmbedding")
    async def test_embed_text_different_models(
        self, mock_openai_class, embedder_service_no_cache
    ):
        """Test embedding with different models"""
        # Setup mocks for different models
        mock_small = MagicMock()
        mock_small.get_text_embedding = Mock(return_value=[0.1] * 1536)

        mock_large = MagicMock()
        mock_large.get_text_embedding = Mock(return_value=[0.2] * 3072)

        mock_openai_class.side_effect = [mock_small, mock_large]

        # Embed with small model
        result1 = await embedder_service_no_cache.embed_text(
            "test", model="text-embedding-3-small"
        )
        assert result1.dimension == 1536

        # Embed with large model
        result2 = await embedder_service_no_cache.embed_text(
            "test", model="text-embedding-3-large"
        )
        assert result2.dimension == 3072

    @pytest.mark.asyncio
    @patch("app.services.embedder.OpenAIEmbedding")
    async def test_embed_text_cost_estimation(
        self, mock_openai_class, embedder_service_no_cache
    ):
        """Test cost estimation for embeddings"""
        mock_instance = MagicMock()
        mock_instance.get_text_embedding = Mock(return_value=[0.1] * 1536)
        mock_openai_class.return_value = mock_instance

        # Longer text should cost more
        short_text = "Hi"
        long_text = "This is a much longer text " * 100

        result1 = await embedder_service_no_cache.embed_text(short_text)
        result2 = await embedder_service_no_cache.embed_text(long_text)

        assert result2.cost > result1.cost


# ============================================================================
# EmbedderService Tests - Query Embedding
# ============================================================================


class TestEmbedderServiceQueryEmbedding:
    """Test query embedding generation"""

    @pytest.mark.asyncio
    @patch("app.services.embedder.OpenAIEmbedding")
    async def test_embed_query_basic(
        self, mock_openai_class, embedder_service_no_cache
    ):
        """Test basic query embedding"""
        mock_instance = MagicMock()
        mock_instance.get_query_embedding = Mock(return_value=[0.3] * 1536)
        mock_openai_class.return_value = mock_instance

        result = await embedder_service_no_cache.embed_query("search query")

        assert isinstance(result, EmbeddingResult)
        assert len(result.embedding) == 1536
        assert result.cached is False

    @pytest.mark.asyncio
    async def test_embed_query_empty_raises_error(self, embedder_service_no_cache):
        """Test that empty query raises ValueError"""
        with pytest.raises(ValueError, match="Query cannot be empty"):
            await embedder_service_no_cache.embed_query("")

    @pytest.mark.asyncio
    @patch("app.services.embedder.OpenAIEmbedding")
    async def test_embed_query_with_cache(
        self, mock_openai_class, embedder_service_with_cache
    ):
        """Test query embedding with cache"""
        mock_instance = MagicMock()
        mock_instance.get_query_embedding = Mock(return_value=[0.3] * 1536)
        mock_openai_class.return_value = mock_instance

        query = "What is risk?"

        # First call - generate and cache
        result1 = await embedder_service_with_cache.embed_query(query)
        assert result1.cached is False

        # Second call - from cache
        result2 = await embedder_service_with_cache.embed_query(query)
        assert result2.cached is True
        assert result2.embedding == result1.embedding


# ============================================================================
# EmbedderService Tests - Batch Embedding
# ============================================================================


class TestEmbedderServiceBatchEmbedding:
    """Test batch embedding generation"""

    @pytest.mark.asyncio
    @patch("app.services.embedder.OpenAIEmbedding")
    async def test_embed_batch_basic(
        self, mock_openai_class, embedder_service_no_cache
    ):
        """Test basic batch embedding"""
        mock_instance = MagicMock()
        mock_instance.get_text_embedding_batch = Mock(
            return_value=[[0.1] * 1536, [0.2] * 1536, [0.3] * 1536]
        )
        mock_openai_class.return_value = mock_instance

        texts = ["text1", "text2", "text3"]
        result = await embedder_service_no_cache.embed_batch(texts)

        assert isinstance(result, BatchEmbeddingResult)
        assert len(result.embeddings) == 3
        assert result.total_embeddings == 3
        assert result.dimension == 1536
        assert result.cached_count == 0

    @pytest.mark.asyncio
    async def test_embed_batch_empty_raises_error(self, embedder_service_no_cache):
        """Test that empty batch raises ValueError"""
        with pytest.raises(ValueError, match="Texts list cannot be empty"):
            await embedder_service_no_cache.embed_batch([])

        with pytest.raises(ValueError, match="All texts are empty"):
            await embedder_service_no_cache.embed_batch(["", "  ", ""])

    @pytest.mark.asyncio
    @patch("app.services.embedder.OpenAIEmbedding")
    async def test_embed_batch_with_cache(
        self, mock_openai_class, embedder_service_with_cache
    ):
        """Test batch embedding with caching"""
        mock_instance = MagicMock()
        mock_instance.get_text_embedding_batch = Mock(
            return_value=[[0.1] * 1536, [0.2] * 1536, [0.3] * 1536]
        )
        mock_openai_class.return_value = mock_instance

        texts = ["text1", "text2", "text3"]

        # First batch - all generated
        result1 = await embedder_service_with_cache.embed_batch(texts)
        assert result1.cached_count == 0
        assert result1.total_embeddings == 3

        # Second batch - all from cache
        result2 = await embedder_service_with_cache.embed_batch(texts)
        assert result2.cached_count == 3
        assert result2.total_embeddings == 3
        assert result2.embeddings == result1.embeddings

    @pytest.mark.asyncio
    @patch("app.services.embedder.OpenAIEmbedding")
    async def test_embed_batch_partial_cache_hit(
        self, mock_openai_class, embedder_service_with_cache
    ):
        """Test batch with partial cache hits"""
        mock_instance = MagicMock()
        # Only return embeddings for uncached texts
        mock_instance.get_text_embedding_batch = Mock(
            return_value=[[0.2] * 1536, [0.4] * 1536]
        )
        mock_openai_class.return_value = mock_instance

        # Pre-cache some texts
        await embedder_service_with_cache.cache.set(
            "text1", "text-embedding-3-small", [0.1] * 1536
        )
        await embedder_service_with_cache.cache.set(
            "text3", "text-embedding-3-small", [0.3] * 1536
        )

        texts = ["text1", "text2", "text3", "text4"]

        # Batch should use cached for text1 and text3, generate text2 and text4
        result = await embedder_service_with_cache.embed_batch(texts)

        assert result.total_embeddings == 4
        assert result.cached_count == 2
        # Verify cached embeddings are in correct positions
        assert result.embeddings[0] == [0.1] * 1536  # text1 from cache
        assert result.embeddings[2] == [0.3] * 1536  # text3 from cache

    @pytest.mark.asyncio
    @patch("app.services.embedder.OpenAIEmbedding")
    async def test_embed_batch_custom_batch_size(
        self, mock_openai_class, embedder_service_no_cache
    ):
        """Test batch embedding with custom batch size"""
        mock_instance = MagicMock()
        # Will be called multiple times for smaller batches
        mock_instance.get_text_embedding_batch = Mock(
            side_effect=[
                [[0.1] * 1536, [0.2] * 1536],  # First batch
                [[0.3] * 1536, [0.4] * 1536],  # Second batch
                [[0.5] * 1536],  # Third batch
            ]
        )
        mock_openai_class.return_value = mock_instance

        texts = ["t1", "t2", "t3", "t4", "t5"]

        result = await embedder_service_no_cache.embed_batch(texts, batch_size=2)

        assert result.total_embeddings == 5
        # Should have called get_text_embedding_batch 3 times
        assert mock_instance.get_text_embedding_batch.call_count == 3

    @pytest.mark.asyncio
    @patch("app.services.embedder.OpenAIEmbedding")
    async def test_embed_batch_filters_empty_texts(
        self, mock_openai_class, embedder_service_no_cache
    ):
        """Test that batch filters out empty texts"""
        mock_instance = MagicMock()
        mock_instance.get_text_embedding_batch = Mock(
            return_value=[[0.1] * 1536, [0.2] * 1536]
        )
        mock_openai_class.return_value = mock_instance

        texts = ["text1", "", "text2", "  "]

        result = await embedder_service_no_cache.embed_batch(texts)

        # Should only embed non-empty texts
        assert result.total_embeddings == 2


# ============================================================================
# EmbedderService Tests - Metrics
# ============================================================================


class TestEmbedderServiceMetrics:
    """Test metrics tracking"""

    @pytest.mark.asyncio
    @patch("app.services.embedder.OpenAIEmbedding")
    async def test_metrics_tracking(self, mock_openai_class):
        """Test that metrics are tracked correctly"""
        mock_instance = MagicMock()
        mock_instance.get_text_embedding = Mock(return_value=[0.1] * 1536)
        mock_openai_class.return_value = mock_instance

        service = EmbedderService(
            default_model="text-embedding-3-small",
            openai_api_key="test-key",
            enable_caching=False,
        )

        # Generate some embeddings
        await service.embed_text("text1")
        await service.embed_text("text2")

        metrics = service.get_metrics()

        assert metrics["total_embeddings_generated"] == 2
        assert metrics["total_cache_hits"] == 0
        assert metrics["cache_hit_rate"] == 0.0
        assert metrics["total_cost_usd"] > 0

    @pytest.mark.asyncio
    @patch("app.services.embedder.OpenAIEmbedding")
    async def test_metrics_with_cache_hits(
        self, mock_openai_class, embedder_service_with_cache
    ):
        """Test metrics with cache hits"""
        mock_instance = MagicMock()
        mock_instance.get_text_embedding = Mock(return_value=[0.1] * 1536)
        mock_openai_class.return_value = mock_instance

        # Generate and cache
        await embedder_service_with_cache.embed_text("test")
        # Hit cache
        await embedder_service_with_cache.embed_text("test")

        metrics = embedder_service_with_cache.get_metrics()

        assert metrics["total_embeddings_generated"] == 1
        assert metrics["total_cache_hits"] == 1
        assert metrics["cache_hit_rate"] == 0.5

    @pytest.mark.asyncio
    async def test_reset_metrics(self, embedder_service_no_cache):
        """Test metrics reset"""
        embedder_service_no_cache.total_embeddings_generated = 10
        embedder_service_no_cache.total_cache_hits = 5
        embedder_service_no_cache.total_cost = 1.5

        embedder_service_no_cache.reset_metrics()

        assert embedder_service_no_cache.total_embeddings_generated == 0
        assert embedder_service_no_cache.total_cache_hits == 0
        assert embedder_service_no_cache.total_cost == 0.0


# ============================================================================
# EmbedderService Tests - Model Management
# ============================================================================


class TestEmbedderServiceModelManagement:
    """Test model instance management"""

    @pytest.mark.asyncio
    @patch("app.services.embedder.OpenAIEmbedding")
    async def test_model_instance_caching(
        self, mock_openai_class, embedder_service_no_cache
    ):
        """Test that model instances are cached"""
        mock_instance = MagicMock()
        mock_instance.get_text_embedding = Mock(return_value=[0.1] * 1536)
        mock_openai_class.return_value = mock_instance

        # Call twice with same model
        await embedder_service_no_cache.embed_text("text1")
        await embedder_service_no_cache.embed_text("text2")

        # Should only create model once
        assert mock_openai_class.call_count == 1
        assert len(embedder_service_no_cache._model_instances) == 1

    @pytest.mark.asyncio
    @patch("app.services.embedder.OpenAIEmbedding")
    async def test_multiple_model_instances(
        self, mock_openai_class, embedder_service_no_cache
    ):
        """Test multiple model instances"""
        mock_instance = MagicMock()
        mock_instance.get_text_embedding = Mock(return_value=[0.1] * 1536)
        mock_openai_class.return_value = mock_instance

        # Use different models
        await embedder_service_no_cache.embed_text(
            "text1", model="text-embedding-3-small"
        )
        await embedder_service_no_cache.embed_text(
            "text2", model="text-embedding-3-large"
        )

        # Should create two model instances
        assert mock_openai_class.call_count == 2
        assert len(embedder_service_no_cache._model_instances) == 2

    @pytest.mark.asyncio
    async def test_model_without_api_key_raises_error(self):
        """Test that OpenAI model without API key raises error"""
        service = EmbedderService(
            default_model="text-embedding-3-small",
            openai_api_key=None,
            enable_caching=False,
        )

        with pytest.raises(ValueError, match="OpenAI API key required"):
            await service.embed_text("test")


# ============================================================================
# EmbedderService Tests - Error Handling
# ============================================================================


class TestEmbedderServiceErrorHandling:
    """Test error handling"""

    @pytest.mark.asyncio
    @patch("app.services.embedder.OpenAIEmbedding")
    async def test_embedding_generation_error_propagates(
        self, mock_openai_class, embedder_service_no_cache
    ):
        """Test that embedding generation errors are propagated"""
        mock_instance = MagicMock()
        mock_instance.get_text_embedding = Mock(side_effect=Exception("API Error"))
        mock_openai_class.return_value = mock_instance

        with pytest.raises(Exception, match="API Error"):
            await embedder_service_no_cache.embed_text("test")

    @pytest.mark.asyncio
    @patch("app.services.embedder.OpenAIEmbedding")
    async def test_batch_embedding_error_propagates(
        self, mock_openai_class, embedder_service_no_cache
    ):
        """Test that batch embedding errors are propagated"""
        mock_instance = MagicMock()
        mock_instance.get_text_embedding_batch = Mock(
            side_effect=Exception("Batch API Error")
        )
        mock_openai_class.return_value = mock_instance

        with pytest.raises(Exception, match="Batch API Error"):
            await embedder_service_no_cache.embed_batch(["test1", "test2"])


# ============================================================================
# Integration Tests
# ============================================================================


class TestEmbedderServiceIntegration:
    """Integration tests for EmbedderService"""

    @pytest.mark.asyncio
    @patch("app.services.embedder.OpenAIEmbedding")
    async def test_full_workflow_with_caching(
        self, mock_openai_class, embedder_service_with_cache
    ):
        """Test complete workflow with caching"""
        mock_instance = MagicMock()
        mock_instance.get_text_embedding = Mock(return_value=[0.1] * 1536)
        mock_instance.get_text_embedding_batch = Mock(
            return_value=[[0.2] * 1536, [0.3] * 1536]
        )
        mock_instance.get_query_embedding = Mock(return_value=[0.4] * 1536)
        mock_openai_class.return_value = mock_instance

        # Single embedding
        result1 = await embedder_service_with_cache.embed_text("hello")
        assert result1.cached is False

        # Same text again - should be cached
        result2 = await embedder_service_with_cache.embed_text("hello")
        assert result2.cached is True

        # Batch embedding
        batch_result = await embedder_service_with_cache.embed_batch(["hello", "world"])
        assert batch_result.cached_count == 1  # "hello" was cached
        assert batch_result.total_embeddings == 2

        # Query embedding
        query_result = await embedder_service_with_cache.embed_query("search query")
        assert query_result.cached is False

        # Check metrics
        metrics = embedder_service_with_cache.get_metrics()
        assert metrics["total_embeddings_generated"] == 3  # hello, world, query
        assert metrics["total_cache_hits"] == 2  # hello twice (text + batch)

    @pytest.mark.asyncio
    @patch("app.services.embedder.OpenAIEmbedding")
    async def test_cost_tracking_across_operations(
        self, mock_openai_class, embedder_service_no_cache
    ):
        """Test cost tracking across multiple operations"""
        mock_instance = MagicMock()
        mock_instance.get_text_embedding = Mock(return_value=[0.1] * 1536)
        mock_instance.get_text_embedding_batch = Mock(return_value=[[0.1] * 1536] * 5)
        mock_openai_class.return_value = mock_instance

        initial_cost = embedder_service_no_cache.total_cost

        # Generate some embeddings
        await embedder_service_no_cache.embed_text("test text")
        await embedder_service_no_cache.embed_batch(["t1", "t2", "t3", "t4", "t5"])

        final_cost = embedder_service_no_cache.total_cost

        # Cost should have increased
        assert final_cost > initial_cost

        # Get metrics
        metrics = embedder_service_no_cache.get_metrics()
        assert metrics["total_cost_usd"] > 0
