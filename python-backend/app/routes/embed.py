"""
Pure Embed Endpoint - Stateless Embedding Generation
This endpoint only generates embeddings for text.
No persistence, no side effects - pure computation.

Now uses the consolidated EmbedderService for better performance,
caching, and provider management.
"""

import logging
from typing import List, Optional

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel, Field

from ..services.embedder import (
    EmbedderService,
    get_embedder_service,
)

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/embed", tags=["compute"])


class EmbedRequest(BaseModel):
    """Request model for embedding generation"""

    texts: List[str] = Field(..., description="List of texts to embed")
    model: Optional[str] = Field(
        None, description="Embedding model to use (default from config)"
    )
    batch_size: int = Field(100, description="Batch size for embedding generation")
    use_cache: bool = Field(True, description="Whether to use embedding cache")


class EmbedSingleRequest(BaseModel):
    """Request model for single text embedding"""

    text: str = Field(..., description="Text to embed")
    model: Optional[str] = Field(
        None, description="Embedding model to use (default from config)"
    )
    use_cache: bool = Field(True, description="Whether to use embedding cache")


class EmbedQueryRequest(BaseModel):
    """Request model for query embedding"""

    query: str = Field(..., description="Query text to embed")
    model: Optional[str] = Field(
        None, description="Embedding model to use (default from config)"
    )
    use_cache: bool = Field(True, description="Whether to use embedding cache")


class EmbeddingResponse(BaseModel):
    """Response model for a single embedding"""

    embedding: List[float] = Field(..., description="Embedding vector")
    dimension: int = Field(..., description="Embedding dimension")
    model: str = Field(..., description="Model used for embedding")
    cached: bool = Field(False, description="Whether result was from cache")


class EmbedBatchResponse(BaseModel):
    """Response model for batch embedding generation"""

    embeddings: List[List[float]] = Field(..., description="List of embedding vectors")
    dimension: int = Field(..., description="Embedding dimension")
    model: str = Field(..., description="Model used for embedding")
    total_embeddings: int = Field(
        ..., description="Total number of embeddings generated"
    )
    cached_count: int = Field(0, description="Number of cached results")


@router.post("/text", response_model=EmbeddingResponse)
async def embed_single_text(request: EmbedSingleRequest) -> EmbeddingResponse:
    """
    Generate embedding for a single text.

    This is a pure computation endpoint - no persistence.

    Args:
        request: EmbedSingleRequest with text and optional model

    Returns:
        EmbeddingResponse with embedding vector

    Raises:
        HTTPException: If embedding generation fails
    """
    try:
        logger.info(
            f"Generating embedding for text: {len(request.text)} chars, "
            f"model={request.model or 'default'}, "
            f"use_cache={request.use_cache}"
        )

        # Get embedder service
        embedder = get_embedder_service()

        # Generate embedding
        result = await embedder.embed_text(
            text=request.text,
            model=request.model,
            use_cache=request.use_cache,
        )

        response = EmbeddingResponse(
            embedding=result.embedding,
            dimension=result.dimension,
            model=result.model,
            cached=result.cached,
        )

        logger.info(
            f"Successfully generated embedding: dimension={result.dimension}, "
            f"cached={result.cached}"
        )

        return response

    except ValueError as e:
        logger.warning(f"Validation error: {e}")
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        logger.error(f"Error generating embedding: {e}", exc_info=True)
        raise HTTPException(
            status_code=500, detail=f"Failed to generate embedding: {str(e)}"
        )


@router.post("/batch", response_model=EmbedBatchResponse)
async def embed_batch_texts(request: EmbedRequest) -> EmbedBatchResponse:
    """
    Generate embeddings for multiple texts in batch.

    This is a pure computation endpoint - no persistence.
    Automatically uses caching and optimized batching.

    Args:
        request: EmbedRequest with list of texts and optional model

    Returns:
        EmbedBatchResponse with list of embedding vectors

    Raises:
        HTTPException: If embedding generation fails
    """
    try:
        logger.info(
            f"Generating embeddings for {len(request.texts)} texts, "
            f"model={request.model or 'default'}, "
            f"batch_size={request.batch_size}, "
            f"use_cache={request.use_cache}"
        )

        # Get embedder service
        embedder = get_embedder_service()

        # Generate embeddings
        result = await embedder.embed_batch(
            texts=request.texts,
            model=request.model,
            batch_size=request.batch_size,
            use_cache=request.use_cache,
        )

        response = EmbedBatchResponse(
            embeddings=result.embeddings,
            dimension=result.dimension,
            model=result.model,
            total_embeddings=result.total_embeddings,
            cached_count=result.cached_count,
        )

        logger.info(
            f"Successfully generated {result.total_embeddings} embeddings, "
            f"dimension={result.dimension}, "
            f"cached={result.cached_count}/{result.total_embeddings}"
        )

        return response

    except ValueError as e:
        logger.warning(f"Validation error: {e}")
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        logger.error(f"Error generating batch embeddings: {e}", exc_info=True)
        raise HTTPException(
            status_code=500, detail=f"Failed to generate embeddings: {str(e)}"
        )


@router.post("/query", response_model=EmbeddingResponse)
async def embed_query(request: EmbedQueryRequest) -> EmbeddingResponse:
    """
    Generate embedding for a search query.

    Optimized for query embeddings (may use different processing than documents).

    Args:
        request: EmbedQueryRequest with query text and optional model

    Returns:
        EmbeddingResponse with query embedding
    """
    try:
        logger.info(
            f"Generating query embedding: {len(request.query)} chars, "
            f"model={request.model or 'default'}, "
            f"use_cache={request.use_cache}"
        )

        # Get embedder service
        embedder = get_embedder_service()

        # Generate query embedding
        result = await embedder.embed_query(
            query=request.query,
            model=request.model,
            use_cache=request.use_cache,
        )

        response = EmbeddingResponse(
            embedding=result.embedding,
            dimension=result.dimension,
            model=result.model,
            cached=result.cached,
        )

        logger.info(
            f"Successfully generated query embedding: dimension={result.dimension}, "
            f"cached={result.cached}"
        )

        return response

    except ValueError as e:
        logger.warning(f"Validation error: {e}")
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        logger.error(f"Error generating query embedding: {e}", exc_info=True)
        raise HTTPException(
            status_code=500, detail=f"Failed to generate query embedding: {str(e)}"
        )


@router.get("/models")
async def list_embedding_models():
    """Get available embedding models"""
    try:
        embedder = get_embedder_service()
        models = embedder.get_available_models()

        return {
            "models": models,
            "default": embedder.default_model,
        }
    except Exception as e:
        logger.error(f"Error listing models: {e}", exc_info=True)
        raise HTTPException(status_code=500, detail=f"Failed to list models: {str(e)}")


@router.get("/metrics")
async def get_embedding_metrics():
    """Get embedding service metrics"""
    try:
        embedder = get_embedder_service()
        metrics = embedder.get_metrics()

        return {
            "metrics": metrics,
            "status": "healthy",
        }
    except Exception as e:
        logger.error(f"Error getting metrics: {e}", exc_info=True)
        raise HTTPException(status_code=500, detail=f"Failed to get metrics: {str(e)}")


@router.post("/metrics/reset")
async def reset_embedding_metrics():
    """Reset embedding service metrics"""
    try:
        embedder = get_embedder_service()
        embedder.reset_metrics()

        return {
            "status": "success",
            "message": "Metrics reset successfully",
        }
    except Exception as e:
        logger.error(f"Error resetting metrics: {e}", exc_info=True)
        raise HTTPException(
            status_code=500, detail=f"Failed to reset metrics: {str(e)}"
        )


@router.get("/health")
async def embed_health():
    """Health check for embed service"""
    try:
        embedder = get_embedder_service()
        metrics = embedder.get_metrics()

        return {
            "status": "healthy",
            "service": "embed",
            "default_model": embedder.default_model,
            "caching_enabled": embedder.cache is not None,
            "models_loaded": metrics["models_loaded"],
            "total_embeddings_generated": metrics["total_embeddings_generated"],
        }
    except Exception as e:
        logger.error(f"Health check failed: {e}", exc_info=True)
        return {
            "status": "unhealthy",
            "service": "embed",
            "error": str(e),
        }
