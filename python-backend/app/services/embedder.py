"""
Embedder Service - Local Embedding Generation with Sentence Transformers

This service provides embedding generation using local sentence-transformers models.
No API keys required - everything runs locally.

Features:
- Local sentence-transformers models (no API calls)
- Redis-backed caching for efficiency
- Intelligent batching
- Model management and lazy initialization
- Metrics tracking

Usage:
    embedder = EmbedderService()
    embedding = await embedder.embed_text("Hello world")
    embeddings = await embedder.embed_batch(["text1", "text2", "text3"])
"""

import asyncio
import hashlib
import json
import logging
from datetime import timedelta
from enum import Enum
from typing import Dict, List, Optional, Tuple

from pydantic import BaseModel, Field

logger = logging.getLogger(__name__)

# Import sentence-transformers
try:
    from sentence_transformers import SentenceTransformer

    SENTENCE_TRANSFORMERS_AVAILABLE = True
except ImportError:
    SENTENCE_TRANSFORMERS_AVAILABLE = False
    SentenceTransformer = None
    logger.error(
        "sentence-transformers not available! "
        "Install with: pip install sentence-transformers"
    )


class EmbeddingProvider(str, Enum):
    """Supported embedding providers"""

    SENTENCE_TRANSFORMERS = "sentence-transformers"


class EmbeddingModelConfig(BaseModel):
    """Configuration for an embedding model"""

    name: str = Field(..., description="Model name/identifier")
    provider: EmbeddingProvider = Field(..., description="Embedding provider")
    dimension: int = Field(..., description="Embedding dimension")
    max_batch_size: int = Field(32, description="Maximum batch size for this model")
    max_input_length: int = Field(512, description="Maximum input token length")

    class Config:
        use_enum_values = True


class EmbeddingResult(BaseModel):
    """Result of embedding generation"""

    embedding: List[float] = Field(..., description="Embedding vector")
    dimension: int = Field(..., description="Embedding dimension")
    model: str = Field(..., description="Model used")
    cached: bool = Field(False, description="Whether result was cached")


class BatchEmbeddingResult(BaseModel):
    """Result of batch embedding generation"""

    embeddings: List[List[float]] = Field(..., description="List of embedding vectors")
    dimension: int = Field(..., description="Embedding dimension")
    model: str = Field(..., description="Model used")
    total_embeddings: int = Field(..., description="Number of embeddings generated")
    cached_count: int = Field(0, description="Number of cached embeddings")


# Predefined model configurations (all local/free)
EMBEDDING_MODELS: Dict[str, EmbeddingModelConfig] = {
    "all-MiniLM-L6-v2": EmbeddingModelConfig(
        name="sentence-transformers/all-MiniLM-L6-v2",
        provider=EmbeddingProvider.SENTENCE_TRANSFORMERS,
        dimension=384,
        max_batch_size=32,
        max_input_length=512,
    ),
    "all-mpnet-base-v2": EmbeddingModelConfig(
        name="sentence-transformers/all-mpnet-base-v2",
        provider=EmbeddingProvider.SENTENCE_TRANSFORMERS,
        dimension=768,
        max_batch_size=32,
        max_input_length=512,
    ),
    "paraphrase-MiniLM-L6-v2": EmbeddingModelConfig(
        name="sentence-transformers/paraphrase-MiniLM-L6-v2",
        provider=EmbeddingProvider.SENTENCE_TRANSFORMERS,
        dimension=384,
        max_batch_size=32,
        max_input_length=512,
    ),
}


class EmbeddingCache:
    """
    Redis-backed cache for embeddings.
    Uses content hash as cache key.
    """

    def __init__(self, redis_client=None, ttl_seconds: int = 86400):
        """
        Initialize embedding cache.

        Args:
            redis_client: Redis client instance (optional)
            ttl_seconds: Cache TTL in seconds (default 24 hours)
        """
        self.redis = redis_client
        self.ttl = ttl_seconds
        self.enabled = redis_client is not None

        # In-memory cache as fallback
        self._memory_cache: Dict[str, List[float]] = {}
        self._max_memory_cache_size = 1000

        logger.info(
            f"Embedding cache initialized: "
            f"redis={'enabled' if self.enabled else 'disabled'}, "
            f"ttl={ttl_seconds}s"
        )

    def _generate_cache_key(self, text: str, model: str) -> str:
        """Generate cache key from text and model"""
        content = f"{model}:{text}"
        hash_digest = hashlib.sha256(content.encode()).hexdigest()
        return f"embed:v1:{hash_digest}"

    async def get(self, text: str, model: str) -> Optional[List[float]]:
        """
        Get cached embedding for text.

        Args:
            text: Input text
            model: Model name

        Returns:
            Cached embedding or None
        """
        cache_key = self._generate_cache_key(text, model)

        # Try Redis first
        if self.enabled and self.redis:
            try:
                cached_data = await asyncio.to_thread(self.redis.get, cache_key)
                if cached_data:
                    embedding = json.loads(cached_data)
                    logger.debug(f"Cache hit (Redis): {cache_key[:16]}...")
                    return embedding
            except Exception as e:
                logger.warning(f"Redis cache get error: {e}")

        # Fallback to memory cache
        if cache_key in self._memory_cache:
            logger.debug(f"Cache hit (memory): {cache_key[:16]}...")
            return self._memory_cache[cache_key]

        return None

    async def set(self, text: str, model: str, embedding: List[float]) -> None:
        """
        Cache an embedding.

        Args:
            text: Input text
            model: Model name
            embedding: Embedding vector
        """
        cache_key = self._generate_cache_key(text, model)
        embedding_json = json.dumps(embedding)

        # Store in Redis
        if self.enabled and self.redis:
            try:
                await asyncio.to_thread(
                    self.redis.setex, cache_key, self.ttl, embedding_json
                )
                logger.debug(f"Cached to Redis: {cache_key[:16]}...")
            except Exception as e:
                logger.warning(f"Redis cache set error: {e}")

        # Also store in memory cache (with size limit)
        if len(self._memory_cache) >= self._max_memory_cache_size:
            # Remove oldest entry (simple FIFO)
            first_key = next(iter(self._memory_cache))
            del self._memory_cache[first_key]

        self._memory_cache[cache_key] = embedding
        logger.debug(f"Cached to memory: {cache_key[:16]}...")

    async def get_batch(
        self, texts: List[str], model: str
    ) -> Tuple[Dict[int, List[float]], List[int]]:
        """
        Get cached embeddings for a batch of texts.

        Args:
            texts: List of input texts
            model: Model name

        Returns:
            Tuple of (cached_embeddings_dict, uncached_indices)
        """
        cached = {}
        uncached_indices = []

        for i, text in enumerate(texts):
            embedding = await self.get(text, model)
            if embedding is not None:
                cached[i] = embedding
            else:
                uncached_indices.append(i)

        return cached, uncached_indices

    async def set_batch(
        self, texts: List[str], model: str, embeddings: List[List[float]]
    ) -> None:
        """
        Cache a batch of embeddings.

        Args:
            texts: List of input texts
            model: Model name
            embeddings: List of embedding vectors
        """
        if len(texts) != len(embeddings):
            raise ValueError("Texts and embeddings must have same length")

        # Cache in parallel
        tasks = [
            self.set(text, model, embedding)
            for text, embedding in zip(texts, embeddings)
        ]
        await asyncio.gather(*tasks, return_exceptions=True)


class SentenceTransformerWrapper:
    """Wrapper for SentenceTransformer with async support"""

    def __init__(self, model_name: str):
        if not SENTENCE_TRANSFORMERS_AVAILABLE:
            raise ImportError(
                "sentence-transformers not available. "
                "Install with: pip install sentence-transformers"
            )
        logger.info(f"Loading sentence-transformer model: {model_name}")
        self.model = SentenceTransformer(model_name)
        self.model_name = model_name

    def get_text_embedding(self, text: str) -> List[float]:
        """Get embedding for a single text"""
        return self.model.encode(text, show_progress_bar=False).tolist()

    def get_text_embedding_batch(self, texts: List[str]) -> List[List[float]]:
        """Get embeddings for a batch of texts"""
        embeddings = self.model.encode(texts, show_progress_bar=False)
        return [emb.tolist() for emb in embeddings]

    def get_query_embedding(self, query: str) -> List[float]:
        """Get embedding for a query (same as text for sentence-transformers)"""
        return self.get_text_embedding(query)


class EmbedderService:
    """
    Unified embedder service using local sentence-transformers models.
    No API keys required - everything runs locally and is free.
    """

    def __init__(
        self,
        default_model: str = "all-MiniLM-L6-v2",
        redis_client=None,
        enable_caching: bool = True,
    ):
        """
        Initialize embedder service.

        Args:
            default_model: Default embedding model to use
            redis_client: Redis client for caching
            enable_caching: Whether to enable embedding caching
        """
        self.default_model = default_model

        # Model instances cache
        self._model_instances: Dict[str, SentenceTransformerWrapper] = {}

        # Embedding cache
        self.cache = EmbeddingCache(redis_client) if enable_caching else None

        # Metrics
        self.total_embeddings_generated = 0
        self.total_cache_hits = 0

        logger.info(
            f"EmbedderService initialized: "
            f"default_model={default_model}, "
            f"caching={'enabled' if enable_caching else 'disabled'}, "
            f"provider=local (sentence-transformers)"
        )

    def _get_model_config(self, model_name: str) -> EmbeddingModelConfig:
        """Get configuration for a model"""
        # Handle full model names like "sentence-transformers/all-MiniLM-L6-v2"
        if "/" in model_name:
            short_name = model_name.split("/")[-1]
        else:
            short_name = model_name

        # Try to find by short name
        if short_name in EMBEDDING_MODELS:
            return EMBEDDING_MODELS[short_name]

        # Try to find by full name
        for config in EMBEDDING_MODELS.values():
            if config.name == model_name:
                return config

        # Unknown model - use defaults for sentence-transformers
        logger.warning(f"Unknown model {model_name}, using defaults")
        return EmbeddingModelConfig(
            name=model_name,
            provider=EmbeddingProvider.SENTENCE_TRANSFORMERS,
            dimension=384,  # Common dimension
            max_batch_size=32,
            max_input_length=512,
        )

    def _get_or_create_model(self, model_name: str) -> SentenceTransformerWrapper:
        """
        Get or create an embedding model instance.

        Args:
            model_name: Model name

        Returns:
            Embedding model instance
        """
        # Return cached instance if exists
        if model_name in self._model_instances:
            return self._model_instances[model_name]

        # Get model config
        config = self._get_model_config(model_name)

        # Create model
        try:
            model = SentenceTransformerWrapper(model_name=config.name)
            logger.info(f"Created sentence-transformer model: {config.name}")
        except Exception as e:
            logger.error(f"Failed to load model {config.name}: {e}")
            # Try fallback to default
            if model_name != "all-MiniLM-L6-v2":
                logger.warning("Falling back to default model: all-MiniLM-L6-v2")
                return self._get_or_create_model("all-MiniLM-L6-v2")
            else:
                raise

        # Cache the instance
        self._model_instances[model_name] = model

        return model

    async def embed_text(
        self,
        text: str,
        model: Optional[str] = None,
        use_cache: bool = True,
    ) -> EmbeddingResult:
        """
        Generate embedding for a single text.

        Args:
            text: Text to embed
            model: Model to use (defaults to default_model)
            use_cache: Whether to use cache

        Returns:
            EmbeddingResult with embedding vector

        Raises:
            ValueError: If text is empty or model is invalid
        """
        # Validate input
        if not text or not text.strip():
            raise ValueError("Text cannot be empty")

        # Use default model if not specified
        model_name = model or self.default_model
        config = self._get_model_config(model_name)

        # Try cache first
        if use_cache and self.cache:
            cached_embedding = await self.cache.get(text, model_name)
            if cached_embedding is not None:
                self.total_cache_hits += 1
                return EmbeddingResult(
                    embedding=cached_embedding,
                    dimension=len(cached_embedding),
                    model=model_name,
                    cached=True,
                )

        # Generate embedding
        model_instance = self._get_or_create_model(model_name)

        try:
            # Run in thread pool to avoid blocking
            embedding = await asyncio.to_thread(model_instance.get_text_embedding, text)

            # Update metrics
            self.total_embeddings_generated += 1

            # Cache the result
            if use_cache and self.cache:
                await self.cache.set(text, model_name, embedding)

            return EmbeddingResult(
                embedding=embedding,
                dimension=len(embedding),
                model=model_name,
                cached=False,
            )

        except Exception as e:
            logger.error(f"Error generating embedding: {e}", exc_info=True)
            raise

    async def embed_query(
        self,
        query: str,
        model: Optional[str] = None,
        use_cache: bool = True,
    ) -> EmbeddingResult:
        """
        Generate embedding for a search query.

        Args:
            query: Query text
            model: Model to use
            use_cache: Whether to use cache

        Returns:
            EmbeddingResult with query embedding
        """
        # Validate input
        if not query or not query.strip():
            raise ValueError("Query cannot be empty")

        # Use default model if not specified
        model_name = model or self.default_model

        # Try cache first (with "query:" prefix to differentiate from documents)
        cache_key_text = f"query:{query}"
        if use_cache and self.cache:
            cached_embedding = await self.cache.get(cache_key_text, model_name)
            if cached_embedding is not None:
                self.total_cache_hits += 1
                return EmbeddingResult(
                    embedding=cached_embedding,
                    dimension=len(cached_embedding),
                    model=model_name,
                    cached=True,
                )

        # Generate query embedding
        model_instance = self._get_or_create_model(model_name)

        try:
            # Run in thread pool
            embedding = await asyncio.to_thread(
                model_instance.get_query_embedding, query
            )

            # Update metrics
            self.total_embeddings_generated += 1

            # Cache the result
            if use_cache and self.cache:
                await self.cache.set(cache_key_text, model_name, embedding)

            return EmbeddingResult(
                embedding=embedding,
                dimension=len(embedding),
                model=model_name,
                cached=False,
            )

        except Exception as e:
            logger.error(f"Error generating query embedding: {e}", exc_info=True)
            raise

    async def embed_batch(
        self,
        texts: List[str],
        model: Optional[str] = None,
        batch_size: Optional[int] = None,
        use_cache: bool = True,
    ) -> BatchEmbeddingResult:
        """
        Generate embeddings for multiple texts in batch.

        Automatically handles:
        - Cache lookups
        - Batch size optimization
        - Parallel processing

        Args:
            texts: List of texts to embed
            model: Model to use
            batch_size: Batch size (defaults to model's max_batch_size)
            use_cache: Whether to use cache

        Returns:
            BatchEmbeddingResult with embeddings

        Raises:
            ValueError: If texts list is empty
        """
        # Validate input
        if not texts:
            raise ValueError("Texts list cannot be empty")

        # Filter out empty texts
        valid_indices = [i for i, text in enumerate(texts) if text and text.strip()]
        if not valid_indices:
            raise ValueError("All texts are empty")

        valid_texts = [texts[i] for i in valid_indices]

        # Use default model if not specified
        model_name = model or self.default_model
        config = self._get_model_config(model_name)

        # Determine batch size
        effective_batch_size = batch_size or config.max_batch_size

        # Try cache first
        cached_embeddings = {}
        uncached_indices = list(range(len(valid_texts)))

        if use_cache and self.cache:
            cached_embeddings, uncached_indices = await self.cache.get_batch(
                valid_texts, model_name
            )
            self.total_cache_hits += len(cached_embeddings)
            logger.info(
                f"Cache stats: {len(cached_embeddings)} hits, "
                f"{len(uncached_indices)} misses out of {len(valid_texts)} texts"
            )

        # Generate embeddings for uncached texts
        new_embeddings = []

        if uncached_indices:
            uncached_texts = [valid_texts[i] for i in uncached_indices]

            # Get model instance
            model_instance = self._get_or_create_model(model_name)

            # Process in batches
            for i in range(0, len(uncached_texts), effective_batch_size):
                batch = uncached_texts[i : i + effective_batch_size]

                try:
                    # Generate batch embeddings (run in thread pool)
                    batch_embeddings = await asyncio.to_thread(
                        model_instance.get_text_embedding_batch, batch
                    )
                    new_embeddings.extend(batch_embeddings)

                    # Update metrics
                    self.total_embeddings_generated += len(batch_embeddings)

                    logger.debug(
                        f"Generated {len(batch_embeddings)} embeddings "
                        f"(batch {i // effective_batch_size + 1})"
                    )

                except Exception as e:
                    logger.error(
                        f"Error generating batch embeddings: {e}", exc_info=True
                    )
                    raise

            # Cache new embeddings
            if use_cache and self.cache:
                await self.cache.set_batch(uncached_texts, model_name, new_embeddings)

        # Combine cached and new embeddings
        all_embeddings = [None] * len(valid_texts)

        # Fill in cached embeddings
        for idx, embedding in cached_embeddings.items():
            all_embeddings[idx] = embedding

        # Fill in new embeddings
        for i, idx in enumerate(uncached_indices):
            all_embeddings[idx] = new_embeddings[i]

        return BatchEmbeddingResult(
            embeddings=all_embeddings,
            dimension=len(all_embeddings[0]) if all_embeddings else 0,
            model=model_name,
            total_embeddings=len(all_embeddings),
            cached_count=len(cached_embeddings),
        )

    def get_available_models(self) -> List[Dict]:
        """
        Get list of available embedding models.

        Returns:
            List of model information dictionaries
        """
        models = []
        for short_name, config in EMBEDDING_MODELS.items():
            models.append(
                {
                    "id": short_name,
                    "name": config.name,
                    "provider": config.provider,
                    "dimension": config.dimension,
                    "max_batch_size": config.max_batch_size,
                    "max_input_length": config.max_input_length,
                    "free": True,  # All local models are free
                    "local": True,  # All are local
                }
            )

        return models

    def get_metrics(self) -> Dict:
        """
        Get service metrics.

        Returns:
            Dictionary with metrics
        """
        cache_hit_rate = 0.0
        if self.total_embeddings_generated + self.total_cache_hits > 0:
            cache_hit_rate = self.total_cache_hits / (
                self.total_embeddings_generated + self.total_cache_hits
            )

        return {
            "total_embeddings_generated": self.total_embeddings_generated,
            "total_cache_hits": self.total_cache_hits,
            "cache_hit_rate": cache_hit_rate,
            "models_loaded": len(self._model_instances),
            "default_model": self.default_model,
            "caching_enabled": self.cache is not None,
            "provider": "sentence-transformers (local)",
        }

    def reset_metrics(self) -> None:
        """Reset service metrics"""
        self.total_embeddings_generated = 0
        self.total_cache_hits = 0
        logger.info("Metrics reset")


# Singleton instance (lazy initialization)
_embedder_service: Optional[EmbedderService] = None


def get_embedder_service(
    default_model: Optional[str] = None,
    redis_client=None,
    enable_caching: bool = True,
) -> EmbedderService:
    """
    Get or create the singleton embedder service instance.

    Args:
        default_model: Default embedding model
        redis_client: Redis client for caching
        enable_caching: Whether to enable caching

    Returns:
        EmbedderService instance
    """
    global _embedder_service

    if _embedder_service is None:
        from ..config import get_settings

        settings = get_settings()

        # Get Redis client if available
        if redis_client is None and enable_caching:
            try:
                from .redis_service import get_redis_client

                redis_client = get_redis_client()
            except Exception as e:
                logger.warning(f"Could not initialize Redis for caching: {e}")
                redis_client = None

        # Map config model name to short name
        model_name = default_model or settings.embedding_model or "all-MiniLM-L6-v2"

        # Handle "sentence-transformers/model-name" format from config
        if "/" in model_name:
            model_name = model_name.split("/")[-1]

        _embedder_service = EmbedderService(
            default_model=model_name,
            redis_client=redis_client,
            enable_caching=enable_caching,
        )

    return _embedder_service
