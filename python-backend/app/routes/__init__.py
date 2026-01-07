"""
API Routes package
"""

# New stateless compute routers (Phase 2 - Primary)
from .chunk import router as chunk_router

# Deprecated routes (kept for backward compatibility)
from .deprecated import documents_router, rag_router, search_router
from .embed import router as embed_router
from .health import router as health_router
from .metadata import router as metadata_router
from .parse import router as parse_router

__all__ = [
    # Primary compute routers
    "parse_router",
    "chunk_router",
    "embed_router",
    "metadata_router",
    "health_router",
    # Deprecated routers (will be removed in future version)
    "documents_router",
    "search_router",
    "rag_router",
]
