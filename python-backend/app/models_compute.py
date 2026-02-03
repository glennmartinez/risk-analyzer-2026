"""
Centralized Pydantic Models for Compute Endpoints

This file contains all request/response models for stateless compute endpoints:
- Parse (document parsing)
- Chunk (text chunking)
- Embed (embedding generation)
- Metadata (metadata extraction)

All models include validation rules and comprehensive field descriptions.
"""

from typing import List, Optional

from pydantic import BaseModel, Field, field_validator

# ============================================================================
# Parse Models
# ============================================================================


class ParseRequest(BaseModel):
    """Request model for document parsing"""

    file_path: Optional[str] = Field(
        None, description="Path to file on server (for testing)"
    )
    extract_metadata: bool = Field(
        True, description="Whether to extract document metadata"
    )
    max_pages: int = Field(
        0, ge=0, description="Maximum pages to process (0 = all pages)"
    )

    @field_validator("max_pages")
    @classmethod
    def validate_max_pages(cls, v):
        if v < 0:
            raise ValueError("max_pages must be >= 0")
        return v


class ParseResponse(BaseModel):
    """Response model for document parsing"""

    text: str = Field(..., description="Extracted text content")
    markdown: Optional[str] = Field(None, description="Content as markdown")
    metadata: dict = Field(default_factory=dict, description="Document metadata")
    pages: list = Field(default_factory=list, description="Page-level information")
    tables: list = Field(default_factory=list, description="Extracted tables")
    figures: list = Field(default_factory=list, description="Extracted figures")
    extraction_method: str = Field(
        ..., description="Extraction method used (docling/pymupdf)"
    )
    total_pages: int = Field(0, description="Total number of pages processed")


# ============================================================================
# Chunk Models
# ============================================================================


class ChunkRequest(BaseModel):
    """Request model for text chunking"""

    text: str = Field(..., min_length=1, description="Text to chunk")
    strategy: str = Field(
        "sentence",
        description="Chunking strategy (sentence/semantic/token/fixed)",
    )
    chunk_size: int = Field(
        512, ge=50, le=4096, description="Target chunk size in tokens/characters"
    )
    chunk_overlap: int = Field(50, ge=0, le=512, description="Overlap between chunks")
    extract_metadata: bool = Field(
        False, description="Extract metadata (title, keywords) using LLM"
    )
    num_questions: int = Field(
        3,
        ge=1,
        le=10,
        description="Number of questions to generate per chunk (if extract_metadata)",
    )

    @field_validator("text")
    @classmethod
    def validate_text(cls, v):
        if not v or not v.strip():
            raise ValueError("Text cannot be empty")
        return v

    @field_validator("strategy")
    @classmethod
    def validate_strategy(cls, v):
        allowed = ["sentence", "semantic", "token", "fixed"]
        if v not in allowed:
            raise ValueError(f"Strategy must be one of: {', '.join(allowed)}")
        return v

    @field_validator("chunk_overlap")
    @classmethod
    def validate_overlap(cls, v, info):
        chunk_size = info.data.get("chunk_size", 512)
        if v >= chunk_size:
            raise ValueError("chunk_overlap must be less than chunk_size")
        return v


class SimpleChunkRequest(BaseModel):
    """Simple request model for basic text chunking"""

    text: str = Field(..., min_length=1, description="Text to chunk")
    chunk_size: int = Field(512, ge=50, le=4096, description="Target chunk size")
    chunk_overlap: int = Field(50, ge=0, le=512, description="Overlap between chunks")

    @field_validator("text")
    @classmethod
    def validate_text(cls, v):
        if not v or not v.strip():
            raise ValueError("Text cannot be empty")
        return v


class ChunkMetadata(BaseModel):
    """Metadata for a single chunk"""

    chunk_index: int = Field(..., ge=0, description="Index of this chunk")
    title: Optional[str] = Field(None, description="Extracted title")
    keywords: Optional[List[str]] = Field(None, description="Extracted keywords")
    questions: Optional[List[str]] = Field(
        None, description="Questions this chunk answers"
    )
    token_count: Optional[int] = Field(
        None, ge=0, description="Number of tokens in chunk"
    )


class TextChunkResponse(BaseModel):
    """Response model for a single chunk"""

    text: str = Field(..., description="Chunk text content")
    index: int = Field(..., ge=0, description="Chunk index")
    metadata: Optional[ChunkMetadata] = Field(None, description="Chunk metadata")


class ChunkResponse(BaseModel):
    """Response model for text chunking"""

    chunks: List[TextChunkResponse] = Field(..., description="List of text chunks")
    total_chunks: int = Field(..., ge=0, description="Total number of chunks")
    strategy_used: str = Field(..., description="Chunking strategy used")
    chunk_size: int = Field(..., ge=0, description="Chunk size used")
    chunk_overlap: int = Field(..., ge=0, description="Overlap used")


# ============================================================================
# Embed Models
# ============================================================================


class EmbedSingleRequest(BaseModel):
    """Request model for single text embedding"""

    text: str = Field(..., min_length=1, description="Text to embed")
    model: Optional[str] = Field(
        None, description="Embedding model to use (default from config)"
    )
    use_cache: bool = Field(True, description="Whether to use embedding cache")

    @field_validator("text")
    @classmethod
    def validate_text(cls, v):
        if not v or not v.strip():
            raise ValueError("Text cannot be empty")
        return v


class EmbedRequest(BaseModel):
    """Request model for batch embedding generation"""

    texts: List[str] = Field(..., min_length=1, description="List of texts to embed")
    model: Optional[str] = Field(
        None, description="Embedding model to use (default from config)"
    )
    batch_size: int = Field(
        32, ge=1, le=100, description="Batch size for embedding generation"
    )
    use_cache: bool = Field(True, description="Whether to use embedding cache")

    @field_validator("texts")
    @classmethod
    def validate_texts(cls, v):
        if not v:
            raise ValueError("texts list cannot be empty")
        # Check for at least one non-empty text
        if not any(text and text.strip() for text in v):
            raise ValueError("All texts are empty")
        return v


class EmbedQueryRequest(BaseModel):
    """Request model for query embedding"""

    query: str = Field(..., min_length=1, description="Query text to embed")
    model: Optional[str] = Field(
        None, description="Embedding model to use (default from config)"
    )
    use_cache: bool = Field(True, description="Whether to use embedding cache")

    @field_validator("query")
    @classmethod
    def validate_query(cls, v):
        if not v or not v.strip():
            raise ValueError("Query cannot be empty")
        return v


class EmbeddingResponse(BaseModel):
    """Response model for a single embedding"""

    embedding: List[float] = Field(..., description="Embedding vector")
    dimension: int = Field(..., ge=1, description="Embedding dimension")
    model: str = Field(..., description="Model used for embedding")
    cached: bool = Field(False, description="Whether result was from cache")


class EmbedBatchResponse(BaseModel):
    """Response model for batch embedding generation"""

    embeddings: List[List[float]] = Field(..., description="List of embedding vectors")
    dimension: int = Field(..., ge=1, description="Embedding dimension")
    model: str = Field(..., description="Model used for embedding")
    total_embeddings: int = Field(
        ..., ge=0, description="Total number of embeddings generated"
    )
    cached_count: int = Field(0, ge=0, description="Number of cached results")


# ============================================================================
# Metadata Models
# ============================================================================


class MetadataRequest(BaseModel):
    """Request model for metadata extraction"""

    text: str = Field(..., min_length=1, description="Text to extract metadata from")
    extract_title: bool = Field(True, description="Extract document title")
    extract_keywords: bool = Field(True, description="Extract keywords")
    extract_questions: bool = Field(
        True, description="Extract questions answered by text"
    )
    num_questions: int = Field(
        3, ge=1, le=10, description="Number of questions to generate"
    )
    num_keywords: int = Field(
        5, ge=1, le=20, description="Number of keywords to extract"
    )

    @field_validator("text")
    @classmethod
    def validate_text(cls, v):
        if not v or not v.strip():
            raise ValueError("Text cannot be empty")
        return v


class TitleRequest(BaseModel):
    """Request model for title extraction"""

    text: str = Field(..., min_length=1, description="Text to extract title from")

    @field_validator("text")
    @classmethod
    def validate_text(cls, v):
        if not v or not v.strip():
            raise ValueError("Text cannot be empty")
        return v


class KeywordsRequest(BaseModel):
    """Request model for keywords extraction"""

    text: str = Field(..., min_length=1, description="Text to extract keywords from")
    num_keywords: int = Field(
        5, ge=1, le=20, description="Number of keywords to extract"
    )

    @field_validator("text")
    @classmethod
    def validate_text(cls, v):
        if not v or not v.strip():
            raise ValueError("Text cannot be empty")
        return v


class QuestionsRequest(BaseModel):
    """Request model for questions extraction"""

    text: str = Field(..., min_length=1, description="Text to extract questions from")
    num_questions: int = Field(
        3, ge=1, le=10, description="Number of questions to generate"
    )

    @field_validator("text")
    @classmethod
    def validate_text(cls, v):
        if not v or not v.strip():
            raise ValueError("Text cannot be empty")
        return v


class MetadataResponse(BaseModel):
    """Response model for metadata extraction"""

    title: Optional[str] = Field(None, description="Extracted title")
    keywords: Optional[List[str]] = Field(None, description="Extracted keywords")
    questions: Optional[List[str]] = Field(
        None, description="Questions this text answers"
    )
    metadata: dict = Field(
        default_factory=dict, description="Additional extracted metadata"
    )


# ============================================================================
# Health Check Models
# ============================================================================


class HealthResponse(BaseModel):
    """Response model for health checks"""

    status: str = Field(..., description="Health status (healthy/unhealthy)")
    service: str = Field(..., description="Service name")
    details: dict = Field(default_factory=dict, description="Additional details")
