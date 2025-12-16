"""
API Routes package
"""

from .documents import router as documents_router
from .search import router as search_router
from .health import router as health_router

__all__ = ["documents_router", "search_router", "health_router"]
