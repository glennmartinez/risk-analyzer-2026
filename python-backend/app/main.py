"""
Document Processing Microservice
FastAPI application for PDF parsing and chunking
"""

import logging
from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from .config import get_settings
from .routes import documents_router, health_router, rag_router, search_router

# Configure logging
logging.basicConfig(
    level=logging.INFO, format="%(asctime)s - %(name)s - %(levelname)s - %(message)s"
)
logger = logging.getLogger(__name__)


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Application lifespan handler for startup/shutdown"""
    settings = get_settings()
    logger.info(f"Starting {settings.app_name} v{settings.app_version}")
    logger.info(f"Upload directory: {settings.upload_dir}")
    logger.info(f"ChromaDB server: {settings.chroma_host}:{settings.chroma_port}")

    yield

    logger.info("Shutting down...")


def create_app() -> FastAPI:
    """Create and configure the FastAPI application"""
    settings = get_settings()

    app = FastAPI(
        title=settings.app_name,
        version=settings.app_version,
        description="""
## Document Processing Microservice

This service provides:
- **PDF Parsing** with Docling - extracts text, tables, and figures
- **Document Chunking** with LlamaIndex - multiple chunking strategies
- **Vector Storage** with ChromaDB - semantic search capabilities

### Endpoints:
- `/documents/upload` - Upload and process documents
- `/documents/parse` - Parse without chunking
- `/documents/chunk` - Parse and chunk without storing
- `/search/` - Semantic search across stored documents
- `/health` - Service health check
        """,
        lifespan=lifespan,
        docs_url="/docs",
        redoc_url="/redoc",
    )

    # Configure CORS
    app.add_middleware(
        CORSMiddleware,
        allow_origins=settings.cors_origins,
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
    )

    # Include routers
    app.include_router(health_router)
    app.include_router(documents_router)
    app.include_router(search_router)
    app.include_router(rag_router)

    return app


# Create application instance
app = create_app()
