"""
Document Parser Service using Docling with PyMuPDF fallback
Handles PDF and document parsing with structure extraction
"""

import logging
import os
import uuid
from datetime import datetime
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple

import fitz  # PyMuPDF - fallback parser
from docling.datamodel.base_models import InputFormat
from docling.datamodel.pipeline_options import PdfPipelineOptions
from docling.document_converter import DocumentConverter, PdfFormatOption

from ..config import get_settings
from ..models import DocumentMetadata, ParsedDocument

logger = logging.getLogger(__name__)


class DocumentParser:
    """
    Parses documents using Docling library.
    Supports PDF, DOCX, and other formats with table/figure extraction.
    """

    def __init__(self):
        """Initialize the document parser with Docling converter"""
        self.settings = get_settings()

        # Configure PDF pipeline options
        pipeline_options = PdfPipelineOptions()
        pipeline_options.do_ocr = True  # Enable OCR for scanned PDFs
        pipeline_options.do_table_structure = True  # Extract table structure

        # Initialize converter with PDF options
        self.converter = DocumentConverter(
            format_options={
                InputFormat.PDF: PdfFormatOption(pipeline_options=pipeline_options)
            }
        )

        if self.settings.max_pdf_pages > 0:
            logger.info(
                f"PDF processing will be limited to first {self.settings.max_pdf_pages} pages"
            )

        logger.info("DocumentParser initialized with Docling")

    def parse_file(self, file_path: str) -> ParsedDocument:
        """
        Parse a document file and extract content.

        Args:
            file_path: Path to the document file

        Returns:
            ParsedDocument with extracted content and metadata
        """
        file_path = Path(file_path).absolute()

        if not file_path.exists():
            raise FileNotFoundError(f"File not found: {file_path}")

        logger.info(f"Parsing document: {file_path}")

        # Get file metadata
        file_stats = file_path.stat()
        file_type = file_path.suffix.lower().lstrip(".")

        # Initialize variables
        raw_text = ""
        markdown_text = ""
        tables: List[Dict[str, Any]] = []
        figures: List[Dict[str, Any]] = []
        pages: List[Dict[str, Any]] = []
        page_count: Optional[int] = None
        title: Optional[str] = None
        extraction_method = "docling"

        # Try Docling first, fall back to PyMuPDF if it fails
        try:
            # Use page_range to limit processing to first N pages
            if self.settings.max_pdf_pages > 0:
                result = self.converter.convert(
                    str(file_path),
                    page_range=(1, self.settings.max_pdf_pages),
                    raises_on_error=False,
                )
            else:
                result = self.converter.convert(str(file_path), raises_on_error=False)

            # Check if Docling succeeded
            if result.status.name == "SUCCESS" and result.document:
                doc = result.document
                raw_text = doc.export_to_text()
                markdown_text = doc.export_to_markdown()
                tables = self._extract_tables(doc)
                figures = self._extract_figures(doc)
                pages = self._extract_pages(doc)
                page_count = len(pages) if pages else None
                title = self._extract_title_from_docling(doc)
                extraction_method = "docling"
                logger.info("Successfully parsed with Docling")
            else:
                logger.warning(
                    f"Docling failed with status {result.status}, falling back to PyMuPDF"
                )
                raw_text, markdown_text, tables, figures, pages, page_count, title = (
                    self._parse_with_pymupdf(file_path)
                )
                extraction_method = "pymupdf"
        except Exception as e:
            logger.warning(f"Docling error: {e}, falling back to PyMuPDF")
            raw_text, markdown_text, tables, figures, pages, page_count, title = (
                self._parse_with_pymupdf(file_path)
            )
            extraction_method = "pymupdf"

        # Create metadata
        metadata = DocumentMetadata(
            filename=file_path.name,
            file_type=file_type,
            page_count=page_count,
            title=title,
            author=None,
            created_at=datetime.fromtimestamp(file_stats.st_ctime),
            file_size_bytes=file_stats.st_size,
            extraction_method=extraction_method,
        )

        # Generate document ID
        document_id = str(uuid.uuid4())

        parsed_doc = ParsedDocument(
            document_id=document_id,
            metadata=metadata,
            raw_text=raw_text,
            markdown_text=markdown_text,
            pages=pages,
            tables=tables,
            figures=figures,
        )

        logger.info(
            f"Successfully parsed document: {file_path.name}, "
            f"{len(raw_text)} chars, {len(tables)} tables, {len(figures)} figures"
        )

        return parsed_doc

    def _parse_with_pymupdf(
        self, file_path: Path
    ) -> Tuple[str, str, List[Dict], List[Dict], List[Dict], int, Optional[str]]:
        """
        Fallback parser using PyMuPDF for problematic PDFs.
        Returns: (raw_text, markdown_text, tables, figures, pages, page_count, title)
        """
        logger.info(f"Parsing with PyMuPDF: {file_path}")

        doc = fitz.open(str(file_path))
        max_pages = self.settings.max_pdf_pages
        total_pages = len(doc)
        page_count = min(total_pages, max_pages) if max_pages > 0 else total_pages

        raw_text_parts = []
        markdown_parts = []
        pages = []

        for i in range(page_count):
            page = doc[i]
            page_text = page.get_text()
            raw_text_parts.append(page_text)

            # Simple markdown conversion
            markdown_parts.append(f"## Page {i + 1}\n\n{page_text}")

            pages.append({"page_number": i + 1, "text": page_text})

        # Try to extract title from PDF metadata
        title = None
        pdf_metadata = doc.metadata
        if pdf_metadata and pdf_metadata.get("title"):
            title = pdf_metadata["title"]

        doc.close()

        raw_text = "\n\n".join(raw_text_parts)
        markdown_text = "\n\n".join(markdown_parts)

        # PyMuPDF doesn't extract tables/figures as structured data easily
        tables: List[Dict] = []
        figures: List[Dict] = []

        logger.info(f"PyMuPDF extracted {len(raw_text)} chars from {page_count} pages")

        return raw_text, markdown_text, tables, figures, pages, page_count, title

    def parse_bytes(
        self, file_bytes: bytes, filename: str, upload_dir: str
    ) -> ParsedDocument:
        """
        Parse a document from bytes (uploaded file).

        Args:
            file_bytes: Raw file bytes
            filename: Original filename
            upload_dir: Directory to temporarily save the file

        Returns:
            ParsedDocument with extracted content
        """
        # Save temporarily for Docling to process
        temp_path = Path(upload_dir) / f"temp_{uuid.uuid4()}_{filename}"

        try:
            temp_path.write_bytes(file_bytes)
            return self.parse_file(str(temp_path))
        finally:
            # Clean up temp file
            if temp_path.exists():
                temp_path.unlink()

    def _extract_tables(self, doc) -> List[Dict[str, Any]]:
        """Extract tables from the Docling document"""
        tables = []

        try:
            for idx, table in enumerate(doc.tables):
                table_data = {
                    "index": idx,
                    "markdown": table.export_to_markdown()
                    if hasattr(table, "export_to_markdown")
                    else str(table),
                    "page": getattr(table, "page_no", None),
                }

                # Try to get table as dataframe if available
                if hasattr(table, "export_to_dataframe"):
                    try:
                        df = table.export_to_dataframe()
                        table_data["rows"] = len(df)
                        table_data["columns"] = len(df.columns)
                        table_data["headers"] = list(df.columns)
                    except Exception:
                        pass

                tables.append(table_data)
        except Exception as e:
            logger.warning(f"Error extracting tables: {e}")

        return tables

    def _extract_figures(self, doc) -> List[Dict[str, Any]]:
        """Extract figure/image information from the Docling document"""
        figures = []

        try:
            if hasattr(doc, "pictures"):
                for idx, figure in enumerate(doc.pictures):
                    figure_data = {
                        "index": idx,
                        "page": getattr(figure, "page_no", None),
                        "caption": getattr(figure, "caption", None),
                    }
                    figures.append(figure_data)
        except Exception as e:
            logger.warning(f"Error extracting figures: {e}")

        return figures

    def _extract_pages(self, doc) -> List[Dict[str, Any]]:
        """Extract page-level information from the Docling document"""
        pages = []

        try:
            if hasattr(doc, "pages") and doc.pages:
                # doc.pages is a dict {page_no: PageItem}
                # PageItem doesn't have text directly - text is at document level
                # We store page metadata and can use markdown_text/raw_text for content
                for page_no, page in doc.pages.items():
                    page_data = {
                        "page_number": page_no,
                        "width": page.size.width if hasattr(page, "size") else None,
                        "height": page.size.height if hasattr(page, "size") else None,
                    }
                    pages.append(page_data)

                # Sort by page number
                pages.sort(key=lambda p: p["page_number"])
        except Exception as e:
            logger.warning(f"Error extracting pages: {e}")

        return pages

    def _extract_title_from_docling(self, doc) -> Optional[str]:
        """Try to extract document title from Docling document"""
        try:
            # Try to get title from document metadata
            if hasattr(doc, "title") and doc.title:
                return doc.title

            # Try to get first heading
            if hasattr(doc, "headings") and doc.headings:
                return doc.headings[0].text if doc.headings else None

        except Exception as e:
            logger.warning(f"Error extracting title: {e}")

        return None

    def get_supported_formats(self) -> List[str]:
        """Get list of supported file formats"""
        return ["pdf", "docx", "doc", "pptx", "html", "md", "txt"]
