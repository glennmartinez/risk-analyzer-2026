"""
Document processing routes
"""

import logging
import time
from pathlib import Path
from typing import Optional

from fastapi import (
    APIRouter,
    BackgroundTasks,
    Depends,
    File,
    Form,
    HTTPException,
    Query,
    UploadFile,
)
from fastapi.responses import JSONResponse

from ..config import Settings, get_settings
from ..models import (
    ChunkedDocument,
    ChunkingStrategy,
    ErrorResponse,
    ParsedDocument,
    ProcessingRequest,
    ProcessingResponse,
)
from ..services import (
    DocumentChunker,
    DocumentParser,
    DocumentRegistry,
    VectorStoreService,
)

logger = logging.getLogger(__name__)
router = APIRouter(prefix="/documents", tags=["documents"])


# Dependency injection for services
def get_parser():
    return DocumentParser()


def get_chunker():
    return DocumentChunker()


def get_vector_store():
    return VectorStoreService()


def get_registry():
    return DocumentRegistry()


@router.post(
    "/upload",
    response_model=ProcessingResponse,
    responses={400: {"model": ErrorResponse}, 500: {"model": ErrorResponse}},
    summary="Upload and process a document",
    description="Upload a PDF or document file, parse it with Docling, chunk it with LlamaIndex, and optionally store in vector DB",
)
async def upload_document(
    file: UploadFile = File(...),
    chunking_strategy: ChunkingStrategy = Form(default=ChunkingStrategy.SENTENCE),
    chunk_size: int = Form(default=512, ge=100, le=4096),
    chunk_overlap: int = Form(default=50, ge=0, le=500),
    store_in_vector_db: bool = Form(default=False),
    extract_tables: bool = Form(default=True),
    extract_figures: bool = Form(default=True),
    extract_metadata: bool = Form(default=False),
    num_questions: int = Form(default=3, ge=1, le=10),
    max_pages: int = Form(default=30, ge=1, le=500),
    collection_name: Optional[str] = Form(default=None),
    settings: Settings = Depends(get_settings),
    parser: DocumentParser = Depends(get_parser),
    chunker: DocumentChunker = Depends(get_chunker),
    vector_store: VectorStoreService = Depends(get_vector_store),
    registry: DocumentRegistry = Depends(get_registry),
):
    """
    Process an uploaded document:
    1. Parse with Docling (extract text, tables, figures)
    2. Chunk with LlamaIndex
    3. Optionally store in ChromaDB vector store
    """
    start_time = time.time()

    # Validate file
    if not file.filename:
        raise HTTPException(status_code=400, detail="No filename provided")

    file_ext = Path(file.filename).suffix.lower().lstrip(".")
    supported = parser.get_supported_formats()

    if file_ext not in supported:
        raise HTTPException(
            status_code=400,
            detail=f"Unsupported file type: {file_ext}. Supported: {supported}",
        )

    # Check file size
    file_bytes = await file.read()
    max_size = settings.max_file_size_mb * 1024 * 1024

    if len(file_bytes) > max_size:
        raise HTTPException(
            status_code=400,
            detail=f"File too large. Maximum size: {settings.max_file_size_mb}MB",
        )

    try:
        # Step 1: Parse document with Docling
        logger.info(f"Parsing document: {file.filename}")
        parsed_doc = parser.parse_bytes(file_bytes, file.filename, settings.upload_dir)

        # Step 2: Chunk with LlamaIndex
        logger.info(f"Chunking document with strategy: {chunking_strategy}")
        chunked_doc = chunker.chunk_document(
            parsed_doc,
            strategy=chunking_strategy,
            chunk_size=chunk_size,
            chunk_overlap=chunk_overlap,
            extract_metadata=extract_metadata,
            num_questions=num_questions,
        )

        # Step 3: Optionally store in vector DB
        vector_db_stored = False
        if store_in_vector_db:
            logger.info("Storing chunks in vector DB")
            vector_store.store_chunks(chunked_doc, collection_name)
            vector_db_stored = True

            # Register in Redis with all request parameters
            reg_metadata = {
                "filename": file.filename,
                "chunk_count": chunked_doc.total_chunks,
                "collection": collection_name or settings.chroma_collection_name,
                "file_size": len(file_bytes),
                "stored_in_vector_db": True,
                "chunking_strategy": chunking_strategy.value,
                "chunk_size": chunk_size,
                "chunk_overlap": chunk_overlap,
                "extract_metadata": extract_metadata,
                "num_questions": num_questions,
                "max_pages": max_pages,
                "llm_provider": settings.llm_provider,
                "llm_model": settings.llm_model,
            }
            logger.info(f"Registering document metadata in Redis: {reg_metadata}")

            registry.register_document(
                document_id=chunked_doc.document_id, metadata=reg_metadata
            )

        processing_time = time.time() - start_time

        return ProcessingResponse(
            success=True,
            document_id=chunked_doc.document_id,
            message=f"Successfully processed {file.filename}",
            metadata=chunked_doc.metadata,
            chunk_count=chunked_doc.total_chunks,
            vector_db_stored=vector_db_stored,
            processing_time_seconds=round(processing_time, 3),
        )

    except Exception as e:
        logger.exception(f"Error processing document: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.post(
    "/parse",
    response_model=ParsedDocument,
    summary="Parse a document without chunking",
    description="Parse a document with Docling and return extracted content",
)
async def parse_document(
    file: UploadFile = File(...),
    settings: Settings = Depends(get_settings),
    parser: DocumentParser = Depends(get_parser),
):
    """Parse a document and return the extracted content without chunking"""

    if not file.filename:
        raise HTTPException(status_code=400, detail="No filename provided")

    file_bytes = await file.read()

    try:
        parsed_doc = parser.parse_bytes(file_bytes, file.filename, settings.upload_dir)
        return parsed_doc
    except Exception as e:
        logger.exception(f"Error parsing document: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.post(
    "/chunk",
    response_model=ChunkedDocument,
    summary="Parse and chunk a document",
    description="Parse a document and return chunks without storing in vector DB",
)
async def chunk_document(
    file: UploadFile = File(...),
    chunking_strategy: ChunkingStrategy = Query(default=ChunkingStrategy.SENTENCE),
    chunk_size: int = Query(default=512, ge=100, le=4096),
    chunk_overlap: int = Query(default=50, ge=0, le=500),
    settings: Settings = Depends(get_settings),
    parser: DocumentParser = Depends(get_parser),
    chunker: DocumentChunker = Depends(get_chunker),
):
    """Parse and chunk a document, returning all chunks"""

    if not file.filename:
        raise HTTPException(status_code=400, detail="No filename provided")

    file_bytes = await file.read()

    try:
        parsed_doc = parser.parse_bytes(file_bytes, file.filename, settings.upload_dir)
        chunked_doc = chunker.chunk_document(
            parsed_doc,
            strategy=chunking_strategy,
            chunk_size=chunk_size,
            chunk_overlap=chunk_overlap,
        )
        return chunked_doc
    except Exception as e:
        logger.exception(f"Error chunking document: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.delete(
    "/{document_id}",
    summary="Delete a document from vector store",
    description="Remove all chunks for a document from the vector database",
)
async def delete_document(
    document_id: str,
    collection_name: Optional[str] = None,
    vector_store: VectorStoreService = Depends(get_vector_store),
    registry: DocumentRegistry = Depends(get_registry),
):
    """Delete a document's chunks from the vector store and Redis registry"""

    # Delete from vector store
    deleted_count = vector_store.delete_document(document_id, collection_name)

    # Delete from Redis registry
    redis_deleted = registry.delete_document(document_id)

    return {
        "success": True,
        "document_id": document_id,
        "deleted_chunks": deleted_count,
        "deleted_from_registry": redis_deleted,
    }


@router.delete(
    "/collection/{collection_name}",
    summary="Delete a collection",
    description="Delete an entire collection and all its documents from vector store and Redis",
)
async def delete_collection(
    collection_name: str,
    vector_store: VectorStoreService = Depends(get_vector_store),
    registry: DocumentRegistry = Depends(get_registry),
):
    """Delete an entire collection and clean up Redis registry"""

    # First, get all documents in this collection so we can clean up Redis
    documents = vector_store.list_documents(collection_name=collection_name)

    # Delete each document from Redis
    redis_deleted_count = 0
    for doc in documents:
        if registry.delete_document(doc["document_id"]):
            redis_deleted_count += 1

    # Delete the collection from vector store
    try:
        vector_store.chroma_client.delete_collection(collection_name)
        collection_deleted = True
    except Exception as e:
        logger.warning(f"Error deleting collection '{collection_name}': {e}")
        collection_deleted = False

    return {
        "success": collection_deleted,
        "collection_name": collection_name,
        "documents_removed_from_registry": redis_deleted_count,
        "total_documents": len(documents),
    }


@router.get(
    "/strategies",
    summary="Get available chunking strategies",
    description="List all available chunking strategies",
)
async def get_chunking_strategies(chunker: DocumentChunker = Depends(get_chunker)):
    """Get list of available chunking strategies"""
    return {"strategies": chunker.get_available_strategies()}


@router.get(
    "/formats",
    summary="Get supported file formats",
    description="List all supported file formats for parsing",
)
async def get_supported_formats(parser: DocumentParser = Depends(get_parser)):
    """Get list of supported file formats"""
    return {"formats": parser.get_supported_formats()}


@router.post(
    "/process-example",
    response_model=ProcessingResponse,
    responses={400: {"model": ErrorResponse}, 500: {"model": ErrorResponse}},
    summary="Process the example PDF",
    description="Process the bundled example PDF (The Art of Software Testing) - useful for testing the full pipeline",
)
async def process_example_pdf(
    chunking_strategy: ChunkingStrategy = Query(default=ChunkingStrategy.SENTENCE),
    chunk_size: int = Query(default=512, ge=100, le=4096),
    chunk_overlap: int = Query(default=50, ge=0, le=500),
    store_in_vector_db: bool = Query(default=True),
    collection_name: Optional[str] = Query(default=None),
    settings: Settings = Depends(get_settings),
    parser: DocumentParser = Depends(get_parser),
    chunker: DocumentChunker = Depends(get_chunker),
    vector_store: VectorStoreService = Depends(get_vector_store),
    registry: DocumentRegistry = Depends(get_registry),
):
    """
    Process the bundled example PDF through the full pipeline:
    1. Parse with Docling (extract text, tables, figures)
    2. Chunk with LlamaIndex
    3. Store in ChromaDB vector store (enabled by default)

    This is useful for testing and demonstration purposes.
    """
    start_time = time.time()

    # Path to the example PDF
    example_pdf_path = (
        Path(__file__).parent.parent
        / "example_data"
        / "114-the-art-of-software-testing-3-edition.pdf"
    )

    if not example_pdf_path.exists():
        raise HTTPException(
            status_code=404, detail=f"Example PDF not found at: {example_pdf_path}"
        )

    try:
        # Step 1: Parse document with Docling
        logger.info(f"Parsing example PDF: {example_pdf_path.name}")
        parsed_doc = parser.parse_file(str(example_pdf_path))

        # Step 2: Chunk with LlamaIndex
        logger.info(f"Chunking document with strategy: {chunking_strategy}")
        chunked_doc = chunker.chunk_document(
            parsed_doc,
            strategy=chunking_strategy,
            chunk_size=chunk_size,
            chunk_overlap=chunk_overlap,
        )

        # Step 3: Store in vector DB
        vector_db_stored = False
        if store_in_vector_db:
            logger.info("Storing chunks in vector DB")
            vector_store.store_chunks(chunked_doc, collection_name)
            vector_db_stored = True

            # Register in Redis
            registry.register_document(
                document_id=chunked_doc.document_id,
                metadata={
                    "filename": example_pdf_path.name,
                    "chunk_count": chunked_doc.total_chunks,
                    "collection": collection_name or settings.chroma_collection_name,
                    "stored_in_vector_db": True,
                    "is_example": True,
                },
            )

        processing_time = time.time() - start_time

        return ProcessingResponse(
            success=True,
            document_id=chunked_doc.document_id,
            message=f"Successfully processed example PDF: {example_pdf_path.name}",
            metadata=chunked_doc.metadata,
            chunk_count=chunked_doc.total_chunks,
            vector_db_stored=vector_db_stored,
            processing_time_seconds=round(processing_time, 3),
        )

    except Exception as e:
        logger.exception(f"Error processing example PDF: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.get(
    "/collection-stats",
    summary="Get vector store collection statistics",
    description="Get stats about the current vector store collection",
)
async def get_collection_stats(
    collection_name: Optional[str] = Query(default=None),
    vector_store: VectorStoreService = Depends(get_vector_store),
):
    """Get statistics about the vector store collection"""
    try:
        stats = vector_store.get_collection_stats(collection_name)
        return stats
    except Exception as e:
        logger.exception(f"Error getting collection stats: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.get(
    "/list",
    summary="List all processed documents",
    description="Get a list of all documents from Redis registry",
)
async def list_documents(
    registry: DocumentRegistry = Depends(get_registry),
):
    """List all documents registered in the system"""
    try:
        documents = registry.list_documents()
        return {
            "documents": documents,
            "total": len(documents),
        }
    except Exception as e:
        logger.exception(f"Error listing documents: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.get(
    "/chunks",
    summary="Get all chunks from the vector store",
    description="Get chunks with pagination, optionally filtered by document_id",
)
async def get_chunks(
    collection_name: Optional[str] = Query(default=None),
    document_id: Optional[str] = Query(
        default=None, description="Filter by document ID"
    ),
    limit: int = Query(default=100, ge=1, le=1000),
    offset: int = Query(default=0, ge=0),
    vector_store: VectorStoreService = Depends(get_vector_store),
):
    """Get chunks from the vector store with pagination"""
    try:
        result = vector_store.get_all_chunks(
            collection_name=collection_name,
            limit=limit,
            offset=offset,
            document_id=document_id,
        )
        return result
    except Exception as e:
        logger.exception(f"Error getting chunks: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.get(
    "/vector",
    summary="List all documents in vector store",
    description="Get a list of all unique documents stored in the vector database",
)
async def list_vector_documents(
    collection_name: Optional[str] = Query(default=None),
    vector_store: VectorStoreService = Depends(get_vector_store),
):
    """List all documents stored in the vector database"""
    try:
        documents = vector_store.list_documents(collection_name=collection_name)
        return {
            "documents": documents,
            "total": len(documents),
            "collection": collection_name or "all",
        }
    except Exception as e:
        logger.exception(f"Error listing vector documents: {e}")
        raise HTTPException(status_code=500, detail=str(e))
