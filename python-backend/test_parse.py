"""
Simple test script to parse a PDF with Docling and output to JSON
"""

import json
import os
from pathlib import Path
from datetime import datetime

# Add parent to path
import sys
sys.path.insert(0, str(Path(__file__).parent.parent))

from docling.document_converter import DocumentConverter


def parse_pdf_to_json(pdf_path: str, output_dir: str):
    """Parse a PDF and save results to JSON"""
    
    pdf_path = Path(pdf_path)
    output_dir = Path(output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)
    
    print(f"Parsing PDF: {pdf_path}")
    print("This may take a few minutes for large PDFs...")
    
    # Initialize converter
    converter = DocumentConverter()
    
    # Convert the document
    start_time = datetime.now()
    result = converter.convert(str(pdf_path))
    doc = result.document
    end_time = datetime.now()
    
    print(f"Parsing completed in {(end_time - start_time).total_seconds():.2f} seconds")
    
    # Extract content
    raw_text = doc.export_to_text()
    markdown_text = doc.export_to_markdown()
    
    # Get basic info
    output_data = {
        "metadata": {
            "filename": pdf_path.name,
            "parsed_at": datetime.now().isoformat(),
            "parsing_time_seconds": (end_time - start_time).total_seconds(),
            "text_length": len(raw_text),
            "markdown_length": len(markdown_text),
        },
        "raw_text": raw_text,
        "markdown_text": markdown_text,
    }
    
    # Save JSON output
    output_file = output_dir / f"{pdf_path.stem}_parsed.json"
    with open(output_file, "w", encoding="utf-8") as f:
        json.dump(output_data, f, indent=2, ensure_ascii=False)
    
    print(f"Output saved to: {output_file}")
    print(f"Text length: {len(raw_text)} characters")
    
    # Also save markdown separately for easier reading
    md_file = output_dir / f"{pdf_path.stem}_content.md"
    with open(md_file, "w", encoding="utf-8") as f:
        f.write(markdown_text)
    
    print(f"Markdown saved to: {md_file}")
    
    return output_data


if __name__ == "__main__":
    # PDF path
    pdf_path = "/Users/glennmartin/Projects/GO-Projects/risk-analyzer-go/python-backend/app/example_data/114-the-art-of-software-testing-3-edition.pdf"
    
    # Output directory
    output_dir = "/Users/glennmartin/Projects/GO-Projects/risk-analyzer-go/python-backend/output"
    
    # Check if PDF exists
    if not Path(pdf_path).exists():
        print(f"ERROR: PDF not found at {pdf_path}")
        sys.exit(1)
    
    # Parse and save
    parse_pdf_to_json(pdf_path, output_dir)
