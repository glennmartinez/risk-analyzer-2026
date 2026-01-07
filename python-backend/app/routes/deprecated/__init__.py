"""
Deprecated API Routes

These routes are kept for backward compatibility but are deprecated.
New code should use the stateless compute endpoints instead:
- /parse (instead of /documents/parse)
- /chunk (instead of /documents/chunk)
- /embed (instead of vector embeddings)
- /metadata (instead of embedded metadata extraction)

These routes will be removed in a future version.
"""

from .documents import router as documents_router
from .rag import router as rag_router
from .search import router as search_router

__all__ = [
    "documents_router",
    "search_router",
    "rag_router",
]
