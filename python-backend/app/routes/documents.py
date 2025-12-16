"""
Document processing routes
"""

import time
import logging
from typing import Optional
from pathlib import Path

from fastapi import APIRouter, UploadFile, File, HTTPException, Depends, Query
from fastapi.responses import JSONResponse

from ..models import (
    ProcessingRequest,
    ProcessingResponse,
    ChunkedDocument,
    ParsedDocument,
    ChunkingStrategy,
    ErrorResponse,
)
from ..services import DocumentParser, DocumentChunker, VectorStoreService
from ..config import get_settings, Settings

logger = logging.getLogger(__name__)
router = APIRouter(prefix="/documents", tags=["documents"])


# Dependency injection for services
def get_parser():
    return DocumentParser()


def get_chunker():
    return DocumentChunker()


def get_vector_store():
    return VectorStoreService()


@router.post(
    "/upload",
    response_model=ProcessingResponse,
    responses={400: {"model": ErrorResponse}, 500: {"model": ErrorResponse}},
    summary="Upload and process a document",
    description="Upload a PDF or document file, parse it with Docling, chunk it with LlamaIndex, and optionally store in vector DB"
)
async def upload_document(
    file: UploadFile = File(...),
    chunking_strategy: ChunkingStrategy = Query(default=ChunkingStrategy.SENTENCE),
    chunk_size: int = Query(default=512, ge=100, le=4096),
    chunk_overlap: int = Query(default=50, ge=0, le=500),
    store_in_vector_db: bool = Query(default=False),
    collection_name: Optional[str] = Query(default=None),
    settings: Settings = Depends(get_settings),
    parser: DocumentParser = Depends(get_parser),
    chunker: DocumentChunker = Depends(get_chunker),
    vector_store: VectorStoreService = Depends(get_vector_store),
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
            detail=f"Unsupported file type: {file_ext}. Supported: {supported}"
        )
    
    # Check file size
    file_bytes = await file.read()
    max_size = settings.max_file_size_mb * 1024 * 1024
    
    if len(file_bytes) > max_size:
        raise HTTPException(
            status_code=400,
            detail=f"File too large. Maximum size: {settings.max_file_size_mb}MB"
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
            chunk_overlap=chunk_overlap
        )
        
        # Step 3: Optionally store in vector DB
        vector_db_stored = False
        if store_in_vector_db:
            logger.info("Storing chunks in vector DB")
            vector_store.store_chunks(chunked_doc, collection_name)
            vector_db_stored = True
        
        processing_time = time.time() - start_time
        
        return ProcessingResponse(
            success=True,
            document_id=chunked_doc.document_id,
            message=f"Successfully processed {file.filename}",
            metadata=chunked_doc.metadata,
            chunk_count=chunked_doc.total_chunks,
            vector_db_stored=vector_db_stored,
            processing_time_seconds=round(processing_time, 3)
        )
        
    except Exception as e:
        logger.exception(f"Error processing document: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.post(
    "/parse",
    response_model=ParsedDocument,
    summary="Parse a document without chunking",
    description="Parse a document with Docling and return extracted content"
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
    description="Parse a document and return chunks without storing in vector DB"
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
            chunk_overlap=chunk_overlap
        )
        return chunked_doc
    except Exception as e:
        logger.exception(f"Error chunking document: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.delete(
    "/{document_id}",
    summary="Delete a document from vector store",
    description="Remove all chunks for a document from the vector database"
)
async def delete_document(
    document_id: str,
    collection_name: Optional[str] = None,
    vector_store: VectorStoreService = Depends(get_vector_store),
):
    """Delete a document's chunks from the vector store"""
    
    deleted_count = vector_store.delete_document(document_id, collection_name)
    
    return {
        "success": True,
        "document_id": document_id,
        "deleted_chunks": deleted_count
    }


@router.get(
    "/strategies",
    summary="Get available chunking strategies",
    description="List all available chunking strategies"
)
async def get_chunking_strategies(chunker: DocumentChunker = Depends(get_chunker)):
    """Get list of available chunking strategies"""
    return {
        "strategies": chunker.get_available_strategies()
    }


@router.get(
    "/formats",
    summary="Get supported file formats",
    description="List all supported file formats for parsing"
)
async def get_supported_formats(parser: DocumentParser = Depends(get_parser)):
    """Get list of supported file formats"""
    return {
        "formats": parser.get_supported_formats()
    }
