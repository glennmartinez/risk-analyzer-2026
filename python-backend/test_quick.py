"""
Quick test script - parse only first N pages of a PDF using pypdf
"""

import json
from pathlib import Path
from datetime import datetime

from pypdf import PdfReader


def parse_pdf_pages(pdf_path: str, output_dir: str, max_pages: int = 20):
    """Parse first N pages of a PDF using pypdf"""
    
    pdf_path = Path(pdf_path)
    output_dir = Path(output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)
    
    print(f"Parsing PDF: {pdf_path}")
    print(f"Max pages: {max_pages}")
    
    start_time = datetime.now()
    
    # Use pypdf for simple text extraction
    reader = PdfReader(str(pdf_path))
    total_pages = len(reader.pages)
    pages_to_read = min(max_pages, total_pages)
    
    print(f"Total pages in PDF: {total_pages}")
    print(f"Reading first {pages_to_read} pages...")
    
    # Extract text from each page
    pages_text = []
    for i in range(pages_to_read):
        print(f"  Processing page {i+1}/{pages_to_read}...")
        page = reader.pages[i]
        text = page.extract_text() or ""
        pages_text.append({
            "page_number": i + 1,
            "text": text,
            "char_count": len(text)
        })
    
    end_time = datetime.now()
    
    # Combine all text
    raw_text = "\n\n".join([f"--- Page {p['page_number']} ---\n{p['text']}" for p in pages_text])
    
    print(f"Parsing completed in {(end_time - start_time).total_seconds():.2f} seconds")
    
    output_data = {
        "metadata": {
            "filename": pdf_path.name,
            "parsed_at": datetime.now().isoformat(),
            "parsing_time_seconds": (end_time - start_time).total_seconds(),
            "total_pages": total_pages,
            "pages_extracted": pages_to_read,
            "text_length": len(raw_text),
        },
        "pages": pages_text,
        "raw_text": raw_text,
    }
    
    # Save JSON
    output_file = output_dir / f"{pdf_path.stem}_first{max_pages}pages.json"
    with open(output_file, "w", encoding="utf-8") as f:
        json.dump(output_data, f, indent=2, ensure_ascii=False)
    
    print(f"Output saved to: {output_file}")
    print(f"Text length: {len(raw_text)} characters")
    
    # Save text file
    txt_file = output_dir / f"{pdf_path.stem}_first{max_pages}pages.txt"
    with open(txt_file, "w", encoding="utf-8") as f:
        f.write(raw_text)
    
    print(f"Text saved to: {txt_file}")


if __name__ == "__main__":
    pdf_path = "/Users/glennmartin/Projects/GO-Projects/risk-analyzer-go/python-backend/app/example_data/114-the-art-of-software-testing-3-edition.pdf"
    output_dir = "/Users/glennmartin/Projects/GO-Projects/risk-analyzer-go/python-backend/output"
    
    parse_pdf_pages(pdf_path, output_dir, max_pages=20)
