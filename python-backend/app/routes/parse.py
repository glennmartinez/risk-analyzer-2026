"""
Pure Parse Endpoint - Stateless Document Parsing
This endpoint only parses documents and returns the result.
No persistence, no side effects - pure computation.
"""

import logging
from pathlib import Path
from typing import Optional

from fastapi import APIRouter, File, Form, HTTPException, UploadFile
from pydantic import BaseModel, Field

from ..config import get_settings
from ..services.parser import DocumentParser

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/parse", tags=["compute"])


class ParseRequest(BaseModel):
    """Request model for parsing (when using URL or existing file)"""

    file_path: Optional[str] = Field(
        None, description="Path to file on server (for testing)"
    )
    extract_metadata: bool = Field(
        True, description="Whether to extract document metadata"
    )
    max_pages: int = Field(0, description="Maximum pages to process (0 = all pages)")


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


# Initialize parser
parser = DocumentParser()


@router.post("/document", response_model=ParseResponse)
async def parse_document(
    file: UploadFile = File(...),
    extract_metadata: bool = Form(default=True),
    max_pages: int = Form(default=0),
) -> ParseResponse:
    """
    Parse a document and extract text, tables, and figures.

    This is a pure computation endpoint - no persistence.

    Args:
        file: Uploaded document file
        extract_metadata: Whether to extract metadata (title, author, etc.)
        max_pages: Maximum pages to process (0 = all pages)

    Returns:
        ParseResponse with extracted content and metadata

    Raises:
        HTTPException: If parsing fails
    """
    try:
        logger.info(
            f"Parsing document: {file.filename} "
            f"(extract_metadata={extract_metadata}, max_pages={max_pages})"
        )

        # Read file content
        content = await file.read()

        if not content:
            raise HTTPException(status_code=400, detail="Empty file uploaded")

        # Get settings for upload directory
        settings = get_settings()

        # Temporarily override max_pages if specified
        original_max_pages = parser.settings.max_pdf_pages
        if max_pages > 0:
            parser.settings.max_pdf_pages = max_pages

        try:
            # Parse the document
            parsed_doc = parser.parse_bytes(
                file_bytes=content,
                filename=file.filename,
                upload_dir=settings.upload_dir,
            )

            # Build response
            response = ParseResponse(
                text=parsed_doc.raw_text,
                markdown=parsed_doc.markdown_text,
                metadata={
                    "filename": parsed_doc.metadata.filename,
                    "file_type": parsed_doc.metadata.file_type,
                    "page_count": parsed_doc.metadata.page_count,
                    "title": parsed_doc.metadata.title,
                    "author": parsed_doc.metadata.author,
                    "file_size_bytes": parsed_doc.metadata.file_size_bytes,
                    "extraction_method": parsed_doc.metadata.extraction_method,
                }
                if extract_metadata
                else {},
                pages=parsed_doc.pages if extract_metadata else [],
                tables=parsed_doc.tables if extract_metadata else [],
                figures=parsed_doc.figures if extract_metadata else [],
                extraction_method=parsed_doc.metadata.extraction_method,
            )

            logger.info(
                f"Successfully parsed {file.filename}: "
                f"{len(parsed_doc.raw_text)} chars, "
                f"{len(parsed_doc.tables)} tables, "
                f"{len(parsed_doc.figures)} figures"
            )

            return response

        finally:
            # Restore original setting
            parser.settings.max_pdf_pages = original_max_pages

    except Exception as e:
        logger.error(f"Error parsing document {file.filename}: {e}", exc_info=True)
        raise HTTPException(
            status_code=500, detail=f"Failed to parse document: {str(e)}"
        )


@router.post("/text", response_model=ParseResponse)
async def parse_text_only(
    file: UploadFile = File(...), max_pages: int = 0
) -> ParseResponse:
    """
    Parse a document and extract only text (no tables/figures).

    Faster endpoint for text-only extraction.

    Args:
        file: Uploaded document file
        max_pages: Maximum pages to process (0 = all pages)

    Returns:
        ParseResponse with text content only
    """
    try:
        logger.info(f"Parsing text only from: {file.filename} (max_pages={max_pages})")

        # Read file content
        content = await file.read()

        if not content:
            raise HTTPException(status_code=400, detail="Empty file uploaded")

        # Get settings
        settings = get_settings()

        # Temporarily override max_pages if specified
        original_max_pages = parser.settings.max_pdf_pages
        if max_pages > 0:
            parser.settings.max_pdf_pages = max_pages

        try:
            # Parse the document
            parsed_doc = parser.parse_bytes(
                file_bytes=content,
                filename=file.filename,
                upload_dir=settings.upload_dir,
            )

            # Return minimal response with just text
            response = ParseResponse(
                text=parsed_doc.raw_text,
                markdown=None,
                metadata={
                    "filename": parsed_doc.metadata.filename,
                    "extraction_method": parsed_doc.metadata.extraction_method,
                },
                pages=[],
                tables=[],
                figures=[],
                extraction_method=parsed_doc.metadata.extraction_method,
            )

            logger.info(
                f"Successfully parsed text from {file.filename}: "
                f"{len(parsed_doc.raw_text)} chars"
            )

            return response

        finally:
            # Restore original setting
            parser.settings.max_pdf_pages = original_max_pages

    except Exception as e:
        logger.error(f"Error parsing text from {file.filename}: {e}", exc_info=True)
        raise HTTPException(status_code=500, detail=f"Failed to parse text: {str(e)}")


@router.get("/health")
async def parse_health():
    """Health check for parse service"""
    return {
        "status": "healthy",
        "service": "parse",
        "supported_formats": parser.get_supported_formats(),
    }
