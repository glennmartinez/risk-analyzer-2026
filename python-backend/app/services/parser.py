"""
Document Parser Service using Docling
Handles PDF and document parsing with structure extraction
"""

import os
import uuid
import logging
from pathlib import Path
from typing import Optional, Dict, Any, List
from datetime import datetime

from docling.document_converter import DocumentConverter
from docling.datamodel.base_models import InputFormat
from docling.datamodel.pipeline_options import PdfPipelineOptions
from docling.document_converter import PdfFormatOption

from ..models import ParsedDocument, DocumentMetadata

logger = logging.getLogger(__name__)


class DocumentParser:
    """
    Parses documents using Docling library.
    Supports PDF, DOCX, and other formats with table/figure extraction.
    """
    
    def __init__(self):
        """Initialize the document parser with Docling converter"""
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
        
        logger.info("DocumentParser initialized with Docling")
    
    def parse_file(self, file_path: str) -> ParsedDocument:
        """
        Parse a document file and extract content.
        
        Args:
            file_path: Path to the document file
            
        Returns:
            ParsedDocument with extracted content and metadata
        """
        file_path = Path(file_path)
        
        if not file_path.exists():
            raise FileNotFoundError(f"File not found: {file_path}")
        
        logger.info(f"Parsing document: {file_path}")
        
        # Get file metadata
        file_stats = file_path.stat()
        file_type = file_path.suffix.lower().lstrip(".")
        
        # Convert document using Docling
        result = self.converter.convert(str(file_path))
        doc = result.document
        
        # Extract text content
        raw_text = doc.export_to_text()
        markdown_text = doc.export_to_markdown()
        
        # Extract tables
        tables = self._extract_tables(doc)
        
        # Extract figures/images info
        figures = self._extract_figures(doc)
        
        # Extract pages info
        pages = self._extract_pages(doc)
        
        # Get page count
        page_count = len(pages) if pages else None
        
        # Create metadata
        metadata = DocumentMetadata(
            filename=file_path.name,
            file_type=file_type,
            page_count=page_count,
            title=self._extract_title(doc),
            author=None,  # Docling doesn't always extract author
            created_at=datetime.fromtimestamp(file_stats.st_ctime),
            file_size_bytes=file_stats.st_size,
            extraction_method="docling"
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
            figures=figures
        )
        
        logger.info(f"Successfully parsed document: {file_path.name}, "
                   f"{len(raw_text)} chars, {len(tables)} tables, {len(figures)} figures")
        
        return parsed_doc
    
    def parse_bytes(self, file_bytes: bytes, filename: str, upload_dir: str) -> ParsedDocument:
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
        """Extract tables from the document"""
        tables = []
        
        try:
            for idx, table in enumerate(doc.tables):
                table_data = {
                    "index": idx,
                    "markdown": table.export_to_markdown() if hasattr(table, 'export_to_markdown') else str(table),
                    "page": getattr(table, 'page_no', None),
                }
                
                # Try to get table as dataframe if available
                if hasattr(table, 'export_to_dataframe'):
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
        """Extract figure/image information from the document"""
        figures = []
        
        try:
            if hasattr(doc, 'pictures'):
                for idx, figure in enumerate(doc.pictures):
                    figure_data = {
                        "index": idx,
                        "page": getattr(figure, 'page_no', None),
                        "caption": getattr(figure, 'caption', None),
                    }
                    figures.append(figure_data)
        except Exception as e:
            logger.warning(f"Error extracting figures: {e}")
        
        return figures
    
    def _extract_pages(self, doc) -> List[Dict[str, Any]]:
        """Extract page-level information"""
        pages = []
        
        try:
            if hasattr(doc, 'pages'):
                for idx, page in enumerate(doc.pages):
                    page_data = {
                        "page_number": idx + 1,
                        "text": page.export_to_text() if hasattr(page, 'export_to_text') else None,
                    }
                    pages.append(page_data)
        except Exception as e:
            logger.warning(f"Error extracting pages: {e}")
        
        return pages
    
    def _extract_title(self, doc) -> Optional[str]:
        """Try to extract document title"""
        try:
            # Try to get title from document metadata
            if hasattr(doc, 'title') and doc.title:
                return doc.title
            
            # Try to get first heading
            if hasattr(doc, 'headings') and doc.headings:
                return doc.headings[0].text if doc.headings else None
                
        except Exception as e:
            logger.warning(f"Error extracting title: {e}")
        
        return None
    
    def get_supported_formats(self) -> List[str]:
        """Get list of supported file formats"""
        return ["pdf", "docx", "doc", "pptx", "html", "md", "txt"]
