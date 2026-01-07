"""
Vector search routes
"""

import logging
import time
from typing import Any, Dict, Optional

from fastapi import APIRouter, Body, Depends, HTTPException, Query

from app.models import (
    VectorSearchRequest,
    VectorSearchResponse,
    VectorSearchResult,
)
from app.services import VectorStoreService

logger = logging.getLogger(__name__)
router = APIRouter(prefix="/search", tags=["search"])


def get_vector_store():
    return VectorStoreService()


@router.post(
    "/",
    response_model=VectorSearchResponse,
    summary="Search vector database",
    description="Perform semantic search across stored document chunks",
)
async def search_vectors(
    request: VectorSearchRequest,
    vector_store: VectorStoreService = Depends(get_vector_store),
):
    """
    Search for similar document chunks using semantic similarity.
    """
    start_time = time.time()

    try:
        results = vector_store.search(
            query=request.query,
            collection_name=request.collection_name,
            top_k=request.top_k,
            filter_metadata=request.filter_metadata,
        )

        search_time = time.time() - start_time

        return VectorSearchResponse(
            query=request.query,
            results=results,
            total_results=len(results),
            search_time_seconds=round(search_time, 3),
        )

    except Exception as e:
        logger.exception(f"Search error: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.get(
    "/query",
    response_model=VectorSearchResponse,
    summary="Quick search",
    description="Simple GET-based search endpoint",
)
async def quick_search(
    q: str = Query(..., description="Search query"),
    top_k: int = Query(default=5, ge=1, le=100),
    collection: Optional[str] = Query(default=None, description="Collection name"),
    document_id: Optional[str] = Query(
        default=None, description="Filter by document ID"
    ),
    vector_store: VectorStoreService = Depends(get_vector_store),
):
    """Quick search endpoint for simple queries"""
    start_time = time.time()

    filter_metadata = None
    if document_id:
        filter_metadata = {"document_id": document_id}

    try:
        results = vector_store.search(
            query=q,
            collection_name=collection,
            top_k=top_k,
            filter_metadata=filter_metadata,
        )

        search_time = time.time() - start_time

        return VectorSearchResponse(
            query=q,
            results=results,
            total_results=len(results),
            search_time_seconds=round(search_time, 3),
        )

    except Exception as e:
        logger.exception(f"Search error: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.get(
    "/collections",
    summary="List collections",
    description="Get all vector store collections",
)
async def list_collections(
    vector_store: VectorStoreService = Depends(get_vector_store),
):
    """List all collections in the vector store"""
    return {"collections": vector_store.list_collections()}


@router.get(
    "/collections/{collection_name}/stats",
    summary="Collection statistics",
    description="Get statistics for a collection",
)
async def get_collection_stats(
    collection_name: str,
    vector_store: VectorStoreService = Depends(get_vector_store),
):
    """Get statistics for a specific collection"""
    try:
        return vector_store.get_collection_stats(collection_name)
    except Exception as e:
        raise HTTPException(
            status_code=404, detail=f"Collection not found: {collection_name}"
        )


@router.delete(
    "/collections/{collection_name}",
    summary="Reset collection",
    description="Delete and recreate a collection",
)
async def reset_collection(
    collection_name: str,
    vector_store: VectorStoreService = Depends(get_vector_store),
):
    """Reset (delete and recreate) a collection"""
    vector_store.reset_collection(collection_name)
    return {
        "success": True,
        "message": f"Collection '{collection_name}' has been reset",
    }
