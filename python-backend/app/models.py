"""
Pydantic models for API request/response schemas
"""

from typing import List, Optional, Dict, Any
from pydantic import BaseModel, Field
from datetime import datetime
from enum import Enum


class ChunkingStrategy(str, Enum):
    """Available chunking strategies"""
    SENTENCE = "sentence"
    SEMANTIC = "semantic"
    TOKEN = "token"
    RECURSIVE = "recursive"
    MARKDOWN = "markdown"
    HIERARCHICAL = "hierarchical"


class DocumentMetadata(BaseModel):
    """Metadata extracted from a document"""
    filename: str
    file_type: str
    page_count: Optional[int] = None
    title: Optional[str] = None
    author: Optional[str] = None
    created_at: Optional[datetime] = None
    file_size_bytes: int
    extraction_method: str = "docling"


class TextChunk(BaseModel):
    """A chunk of text with metadata"""
    id: str
    text: str
    metadata: Dict[str, Any] = Field(default_factory=dict)
    page_number: Optional[int] = None
    start_char: Optional[int] = None
    end_char: Optional[int] = None
    chunk_index: int
    token_count: Optional[int] = None


class ParsedDocument(BaseModel):
    """Result of parsing a document"""
    document_id: str
    metadata: DocumentMetadata
    raw_text: str
    markdown_text: Optional[str] = None
    pages: List[Dict[str, Any]] = Field(default_factory=list)
    tables: List[Dict[str, Any]] = Field(default_factory=list)
    figures: List[Dict[str, Any]] = Field(default_factory=list)


class ChunkedDocument(BaseModel):
    """Result of chunking a document"""
    document_id: str
    metadata: DocumentMetadata
    chunks: List[TextChunk]
    total_chunks: int
    chunking_strategy: ChunkingStrategy
    chunk_size: int
    chunk_overlap: int


class ProcessingRequest(BaseModel):
    """Request to process a document"""
    chunking_strategy: ChunkingStrategy = ChunkingStrategy.SENTENCE
    chunk_size: int = Field(default=512, ge=100, le=4096)
    chunk_overlap: int = Field(default=50, ge=0, le=500)
    extract_tables: bool = True
    extract_figures: bool = True
    store_in_vector_db: bool = False
    collection_name: Optional[str] = None


class ProcessingResponse(BaseModel):
    """Response after processing a document"""
    success: bool
    document_id: str
    message: str
    metadata: Optional[DocumentMetadata] = None
    chunk_count: int = 0
    vector_db_stored: bool = False
    processing_time_seconds: float


class VectorSearchRequest(BaseModel):
    """Request to search vectors"""
    query: str
    collection_name: Optional[str] = None
    top_k: int = Field(default=5, ge=1, le=100)
    filter_metadata: Optional[Dict[str, Any]] = None


class VectorSearchResult(BaseModel):
    """A single search result"""
    chunk_id: str
    text: str
    score: float
    metadata: Dict[str, Any] = Field(default_factory=dict)


class VectorSearchResponse(BaseModel):
    """Response from vector search"""
    query: str
    results: List[VectorSearchResult]
    total_results: int
    search_time_seconds: float


class HealthResponse(BaseModel):
    """Health check response"""
    status: str
    version: str
    services: Dict[str, bool]


class ErrorResponse(BaseModel):
    """Error response"""
    error: str
    detail: Optional[str] = None
    status_code: int
