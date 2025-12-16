"""
Health check routes
"""

import logging
from fastapi import APIRouter

from ..models import HealthResponse
from ..config import get_settings

logger = logging.getLogger(__name__)
router = APIRouter(tags=["health"])


@router.get(
    "/health",
    response_model=HealthResponse,
    summary="Health check",
    description="Check the health of the service and its dependencies"
)
async def health_check():
    """
    Check service health and dependency status.
    """
    settings = get_settings()
    
    services = {
        "docling": True,
        "llama_index": True,
        "chromadb": True,
        "embeddings": True,
    }
    
    # Check Docling
    try:
        from docling.document_converter import DocumentConverter
        services["docling"] = True
    except ImportError:
        services["docling"] = False
    
    # Check LlamaIndex
    try:
        from llama_index.core import Document
        services["llama_index"] = True
    except ImportError:
        services["llama_index"] = False
    
    # Check ChromaDB
    try:
        import chromadb
        services["chromadb"] = True
    except ImportError:
        services["chromadb"] = False
    
    # Check embeddings
    try:
        from sentence_transformers import SentenceTransformer
        services["embeddings"] = True
    except ImportError:
        services["embeddings"] = False
    
    # Overall status
    all_healthy = all(services.values())
    status = "healthy" if all_healthy else "degraded"
    
    return HealthResponse(
        status=status,
        version=settings.app_version,
        services=services
    )


@router.get(
    "/",
    summary="Root endpoint",
    description="Welcome message and API info"
)
async def root():
    """Root endpoint with API information"""
    settings = get_settings()
    
    return {
        "name": settings.app_name,
        "version": settings.app_version,
        "description": "Document Processing Microservice - PDF parsing with Docling, chunking with LlamaIndex",
        "docs": "/docs",
        "health": "/health"
    }


@router.get(
    "/ready",
    summary="Readiness check",
    description="Check if the service is ready to accept requests"
)
async def readiness():
    """Kubernetes-style readiness probe"""
    return {"status": "ready"}


@router.get(
    "/live",
    summary="Liveness check", 
    description="Check if the service is alive"
)
async def liveness():
    """Kubernetes-style liveness probe"""
    return {"status": "alive"}
