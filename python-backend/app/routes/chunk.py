"""
Pure Chunk Endpoint - Stateless Text Chunking
This endpoint only chunks text into smaller pieces.
No persistence, no side effects - pure computation.
"""

import json
import logging
from enum import Enum
from typing import List, Optional

# from _typeshed import ExcInfo
from fastapi import APIRouter, HTTPException
from pydantic import BaseModel, Field

from ..services.chunker import ChunkingStrategy, DocumentChunker

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/chunk", tags=["compute"])


class ChunkRequest(BaseModel):
    """Request model for text chunking"""

    text: str = Field(..., description="Text to chunk")
    strategy: str = Field(
        "sentence", description="Chunking strategy (sentence/semantic/token/fixed)"
    )
    chunk_size: int = Field(512, description="Target chunk size in tokens/characters")
    chunk_overlap: int = Field(50, description="Overlap between chunks")
    extract_metadata: bool = Field(
        False, description="Extract metadata (title, keywords) using LLM"
    )
    num_questions: int = Field(
        3, description="Number of questions to generate per chunk (if extract_metadata)"
    )


class SimpleChunkRequest(BaseModel):
    """Simple request model for basic text chunking"""

    text: str = Field(..., description="Text to chunk")
    chunk_size: int = Field(512, description="Target chunk size")
    chunk_overlap: int = Field(50, description="Overlap between chunks")


class ChunkMetadata(BaseModel):
    """Metadata for a single chunk"""

    chunk_index: int = Field(..., description="Index of this chunk")
    title: Optional[str] = Field(None, description="Extracted title")
    keywords: Optional[List[str]] = Field(None, description="Extracted keywords")
    questions: Optional[List[str]] = Field(
        None, description="Questions this chunk answers"
    )
    token_count: Optional[int] = Field(None, description="Number of tokens in chunk")


class TextChunkResponse(BaseModel):
    """Response model for a single chunk"""

    text: str = Field(..., description="Chunk text content")
    index: int = Field(..., description="Chunk index")
    metadata: Optional[ChunkMetadata] = Field(None, description="Chunk metadata")


class ChunkResponse(BaseModel):
    """Response model for text chunking"""

    chunks: List[TextChunkResponse] = Field(..., description="List of text chunks")
    total_chunks: int = Field(..., description="Total number of chunks")
    strategy_used: str = Field(..., description="Chunking strategy used")
    chunk_size: int = Field(..., description="Chunk size used")
    chunk_overlap: int = Field(..., description="Overlap used")


# Initialize chunker
chunker = DocumentChunker()


@router.post("/text", response_model=ChunkResponse)
async def chunk_text(request: ChunkRequest) -> ChunkResponse:
    """
    Chunk text into smaller pieces using specified strategy.

    This is a pure computation endpoint - no persistence.

    Args:
        request: ChunkRequest with text and chunking parameters

    Returns:
        ChunkResponse with list of text chunks

    Raises:
        HTTPException: If chunking fails
    """
    try:
        logger.info(
            f"Chunking text: {len(request.text)} chars, "
            f"strategy={request.strategy}, "
            f"size={request.chunk_size}, "
            f"overlap={request.chunk_overlap}"
        )

        # Validate text
        if not request.text or not request.text.strip():
            raise HTTPException(status_code=400, detail="Empty text provided")

        # Map strategy string to enum
        strategy_map = {
            "sentence": ChunkingStrategy.SENTENCE,
            "semantic": ChunkingStrategy.SEMANTIC,
            "token": ChunkingStrategy.TOKEN,
            "fixed": ChunkingStrategy.RECURSIVE,
            "markdown": ChunkingStrategy.MARKDOWN,
            "hierarchical": ChunkingStrategy.HIERARCHICAL,
        }

        strategy = strategy_map.get(request.strategy.lower())
        if not strategy:
            raise HTTPException(
                status_code=400,
                detail=f"Invalid strategy. Must be one of: {list(strategy_map.keys())}",
            )

        # Chunk the text
        chunked_doc = chunker.chunk_text(
            text=request.text,
            strategy=strategy,
            chunk_size=request.chunk_size,
            chunk_overlap=request.chunk_overlap,
            extract_metadata=request.extract_metadata,
            num_questions=request.num_questions if request.extract_metadata else 0,
        )

        # Build response chunks
        response_chunks = []
        for idx, chunk in enumerate(chunked_doc):
            chunk_metadata = None

            if request.extract_metadata and chunk.metadata:
                # DEBUG: Log all metadata keys for first few chunks
                if idx < 3:
                    logger.info(
                        f"DEBUG Chunk {idx} metadata keys: {list(chunk.metadata.keys())}"
                    )
                    logger.info(f"DEBUG Chunk {idx} full metadata: {chunk.metadata}")

                # LlamaIndex extractors use different key names:
                # - TitleExtractor: "document_title"
                # - KeywordExtractor: "excerpt_keywords"
                # - QuestionsAnsweredExtractor: "questions_this_excerpt_can_answer"
                title = chunk.metadata.get("document_title") or chunk.metadata.get(
                    "title"
                )
                keywords_raw = chunk.metadata.get(
                    "excerpt_keywords"
                ) or chunk.metadata.get("keywords")

                # Check for questions under various possible keys
                questions = chunk.metadata.get("questions")
                if questions is None:
                    questions = chunk.metadata.get("questions_this_excerpt_can_answer")
                if questions is None:
                    # Try to find any key containing 'question'
                    for key in chunk.metadata.keys():
                        if "question" in key.lower():
                            questions = chunk.metadata.get(key)
                            if idx < 3:
                                logger.info(
                                    f"DEBUG Found questions under key '{key}': {questions}"
                                )
                            break

                # Parse keywords if it's a comma-separated string
                keywords = None
                if keywords_raw:
                    # if isinstance(keywords_raw, str):
                    #     keywords = [
                    #         k.strip() for k in keywords_raw.split(",") if k.strip()
                    #     ]
                    # elif isinstance(keywords_raw, list):
                    #     keywords = keywords_raw
                    try:
                        parsed = json.loads(keywords_raw)
                        if isinstance(parsed, list):
                            keywords = [
                                str(k).strip() for k in parsed if str(k).strip()
                            ]
                            logger.info(
                                f"DEBUG Parsed keywords as JSON list: {keywords}"
                            )
                        else:
                            keywords = [
                                k.strip() for k in str(parsed).split(",") if k.strip()
                            ]
                            logger.info(
                                f"DEBUG Parsed keywords as JSON non-list: {keywords}"
                            )
                    except Exception:
                        # Fallback to comma-split
                        keywords = [
                            k.strip() for k in str(keywords_raw).split(",") if k.strip()
                        ]
                        logger.info(
                            f"DEBUG Fallback parsed keywords by comma: {keywords}"
                        )
                else:
                    keywords = keywords_raw
                    logger.info(f"DEBUG Keywords raw used as is: {keywords}")
                # Ensure questions is a list
                # if questions and isinstance(questions, str):
                #     questions = [q.strip() for q in questions.split("\n") if q.strip()]

                if questions:
                    if isinstance(questions, str):
                        # Try to parse as JSON array first
                        try:
                            parsed = json.loads(questions)
                            if isinstance(parsed, list):
                                questions = [
                                    str(q).strip() for q in parsed if str(q).strip()
                                ]
                            else:
                                # Fallback: split by newline
                                questions = [
                                    q.strip()
                                    for q in questions.split("\n")
                                    if q.strip()
                                ]
                        except Exception:
                            # Fallback: split by newline
                            questions = [
                                q.strip() for q in questions.split("\n") if q.strip()
                            ]
                    elif isinstance(questions, list):
                        questions = [
                            str(q).strip() for q in questions if str(q).strip()
                        ]

                if idx < 3:
                    logger.info(
                        f"DEBUG Chunk {idx} - title={title}, keywords={keywords}, questions={questions}"
                    )

                chunk_metadata = ChunkMetadata(
                    chunk_index=chunk.chunk_index,
                    title=title,
                    keywords=keywords,
                    questions=questions,
                    token_count=chunk.metadata.get("token_count") or chunk.token_count,
                )

            response_chunks.append(
                TextChunkResponse(
                    text=chunk.text,
                    index=chunk.chunk_index,
                    metadata=chunk_metadata,
                )
            )

        response = ChunkResponse(
            chunks=response_chunks,
            total_chunks=len(response_chunks),
            strategy_used=request.strategy,
            chunk_size=request.chunk_size,
            chunk_overlap=request.chunk_overlap,
        )

        logger.info(
            f"Successfully chunked text into {len(response_chunks)} chunks "
            f"using {request.strategy} strategy"
        )

        return response

    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error chunking text: {e}", exc_info=True)
        raise HTTPException(status_code=500, detail=f"Failed to chunk text: {str(e)}")


@router.post("/simple", response_model=ChunkResponse)
async def chunk_text_simple(request: SimpleChunkRequest) -> ChunkResponse:
    """
    Simple text chunking endpoint with minimal parameters.

    Uses sentence-based chunking strategy by default.

    Args:
        request: SimpleChunkRequest with text and chunking parameters

    Returns:
        ChunkResponse with text chunks (no metadata)
    """
    try:
        logger.info(
            f"Simple chunking: {len(request.text)} chars, "
            f"size={request.chunk_size}, overlap={request.chunk_overlap}"
        )

        # Validate text
        if not request.text or not request.text.strip():
            raise HTTPException(status_code=400, detail="Empty text provided")

        # Use sentence strategy for simple chunking
        chunked_doc = chunker.chunk_text(
            text=request.text,
            strategy=ChunkingStrategy.SENTENCE,
            chunk_size=request.chunk_size,
            chunk_overlap=request.chunk_overlap,
            extract_metadata=False,
            num_questions=0,
        )

        # Build minimal response
        response_chunks = [
            TextChunkResponse(text=chunk.text, index=chunk.chunk_index, metadata=None)
            for chunk in chunked_doc
        ]

        response = ChunkResponse(
            chunks=response_chunks,
            total_chunks=len(response_chunks),
            strategy_used="sentence",
            chunk_size=request.chunk_size,
            chunk_overlap=request.chunk_overlap,
        )

        logger.info(f"Successfully chunked text into {len(response_chunks)} chunks")

        return response

    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error in simple chunking: {e}", exc_info=True)
        raise HTTPException(status_code=500, detail=f"Failed to chunk text: {str(e)}")


@router.get("/strategies")
async def list_strategies():
    """Get available chunking strategies"""
    return {
        "strategies": [
            {
                "name": "sentence",
                "description": "Split by sentences, respecting sentence boundaries",
            },
            {
                "name": "semantic",
                "description": "Split based on semantic similarity (requires embeddings)",
            },
            {
                "name": "token",
                "description": "Split by token count using tokenizer",
            },
            {
                "name": "fixed",
                "description": "Fixed-size chunks by character count",
            },
            {
                "name": "markdown",
                "description": "Split by markdown structure (headers, sections)",
            },
            {
                "name": "hierarchical",
                "description": "Multi-level hierarchical chunking",
            },
        ]
    }


@router.get("/health")
async def chunk_health():
    """Health check for chunk service"""
    return {
        "status": "healthy",
        "service": "chunk",
        "strategies": [
            "sentence",
            "semantic",
            "token",
            "fixed",
            "markdown",
            "hierarchical",
        ],
    }
