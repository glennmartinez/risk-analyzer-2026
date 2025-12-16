"""
Test script to parse a PDF using Docling and output to JSON
Run this directly to test the parsing functionality
"""

import json
import sys
import os
from pathlib import Path
from datetime import datetime

# Add the app directory to the path
sys.path.insert(0, str(Path(__file__).parent.parent))

from docling.document_converter import DocumentConverter
from docling.datamodel.base_models import InputFormat
from docling.datamodel.pipeline_options import PdfPipelineOptions
from docling.document_converter import PdfFormatOption


def parse_pdf_to_json(pdf_path: str, output_dir: str = "./output") -> dict:
    """
    Parse a PDF file and output the results to JSON
    
    Args:
        pdf_path: Path to the PDF file
        output_dir: Directory to save output files
        
    Returns:
        Dictionary with parsed content
    """
    pdf_path = Path(pdf_path)
    output_dir = Path(output_dir)
    
    if not pdf_path.exists():
        raise FileNotFoundError(f"PDF not found: {pdf_path}")
    
    # Create output directory
    output_dir.mkdir(parents=True, exist_ok=True)
    
    print(f"📄 Parsing PDF: {pdf_path.name}")
    print(f"📁 Output directory: {output_dir}")
    print("-" * 50)
    
    # Configure Docling
    pipeline_options = PdfPipelineOptions()
    pipeline_options.do_ocr = False  # Disable OCR for faster processing
    pipeline_options.do_table_structure = True
    
    converter = DocumentConverter(
        format_options={
            InputFormat.PDF: PdfFormatOption(pipeline_options=pipeline_options)
        }
    )
    
    # Convert the PDF
    print("🔄 Converting PDF with Docling...")
    result = converter.convert(str(pdf_path))
    doc = result.document
    
    # Extract content
    print("📝 Extracting text content...")
    raw_text = doc.export_to_text()
    markdown_text = doc.export_to_markdown()
    
    # Extract tables
    tables = []
    try:
        for idx, table in enumerate(doc.tables):
            table_data = {
                "index": idx,
                "markdown": table.export_to_markdown() if hasattr(table, 'export_to_markdown') else str(table),
            }
            tables.append(table_data)
        print(f"📊 Found {len(tables)} tables")
    except Exception as e:
        print(f"⚠️  Error extracting tables: {e}")
    
    # Build output structure
    output = {
        "metadata": {
            "filename": pdf_path.name,
            "file_size_bytes": pdf_path.stat().st_size,
            "parsed_at": datetime.now().isoformat(),
            "parser": "docling",
        },
        "content": {
            "raw_text": raw_text,
            "markdown": markdown_text,
            "text_length": len(raw_text),
            "markdown_length": len(markdown_text),
        },
        "tables": tables,
        "table_count": len(tables),
    }
    
    # Generate output filename
    base_name = pdf_path.stem
    json_output_path = output_dir / f"{base_name}_parsed.json"
    markdown_output_path = output_dir / f"{base_name}_content.md"
    
    # Save JSON
    print(f"💾 Saving JSON to: {json_output_path}")
    with open(json_output_path, "w", encoding="utf-8") as f:
        json.dump(output, f, indent=2, ensure_ascii=False)
    
    # Save Markdown separately for easier reading
    print(f"💾 Saving Markdown to: {markdown_output_path}")
    with open(markdown_output_path, "w", encoding="utf-8") as f:
        f.write(f"# {pdf_path.name}\n\n")
        f.write(f"Parsed at: {output['metadata']['parsed_at']}\n\n")
        f.write("---\n\n")
        f.write(markdown_text)
    
    # Print summary
    print("-" * 50)
    print("✅ Parsing complete!")
    print(f"   📄 Text length: {len(raw_text):,} characters")
    print(f"   📝 Markdown length: {len(markdown_text):,} characters")
    print(f"   📊 Tables found: {len(tables)}")
    print(f"   📁 Output files:")
    print(f"      - {json_output_path}")
    print(f"      - {markdown_output_path}")
    
    return output


if __name__ == "__main__":
    # Default paths
    script_dir = Path(__file__).parent
    default_pdf = script_dir / "example_data" / "114-the-art-of-software-testing-3-edition.pdf"
    default_output = script_dir.parent / "output"
    
    # Allow command line override
    pdf_path = sys.argv[1] if len(sys.argv) > 1 else str(default_pdf)
    output_dir = sys.argv[2] if len(sys.argv) > 2 else str(default_output)
    
    try:
        result = parse_pdf_to_json(pdf_path, output_dir)
    except Exception as e:
        print(f"❌ Error: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)
