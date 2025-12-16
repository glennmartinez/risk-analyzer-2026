"""
Enhanced chunking with page numbers, section headers, and keyword extraction
"""

import json
import re
from pathlib import Path
from datetime import datetime
from collections import Counter

from llama_index.core import Document
from llama_index.core.node_parser import SentenceSplitter

# For keyword extraction
import yake


class KeywordExtractor:
    """Simple keyword extractor using YAKE"""
    
    def __init__(self):
        self.extractor = yake.KeywordExtractor(
            lan="en",
            n=2,  # max ngram size
            dedupLim=0.7,
            top=5,  # top 5 keywords per chunk
            features=None
        )
    
    def extract(self, text: str) -> list[dict]:
        """Extract keywords from text"""
        if not text or len(text) < 20:
            return []
        
        keywords = self.extractor.extract_keywords(text)
        # YAKE returns (keyword, score) - lower score = more relevant
        return [{"keyword": kw, "score": round(1 - score, 3)} for kw, score in keywords]


def detect_section_header(text: str) -> str | None:
    """Try to detect section/chapter headers from text"""
    lines = text.strip().split('\n')
    
    for line in lines[:5]:  # Check first 5 lines
        line = line.strip()
        if not line:
            continue
            
        # Match common header patterns
        patterns = [
            r'^(Chapter\s+\d+[:\s].*)$',
            r'^(CHAPTER\s+\d+[:\s].*)$',
            r'^(\d+\.\d*\s+[A-Z].*)$',  # 1.1 Section Name
            r'^(Part\s+\w+[:\s].*)$',
            r'^([A-Z][A-Z\s]{5,50})$',  # ALL CAPS HEADERS
        ]
        
        for pattern in patterns:
            match = re.match(pattern, line, re.IGNORECASE)
            if match:
                return match.group(1).strip()
    
    return None


def extract_page_number(text: str) -> int | None:
    """Extract page number from chunk text"""
    # Look for our page markers
    match = re.search(r'--- Page (\d+) ---', text)
    if match:
        return int(match.group(1))
    return None


def get_page_for_position(pages_data: list, char_position: int) -> int | None:
    """Get page number for a character position"""
    cumulative = 0
    for page in pages_data:
        page_len = page.get("char_count", len(page.get("text", "")))
        if cumulative + page_len > char_position:
            return page.get("page_number")
        cumulative += page_len + 20  # Account for page separators
    return None


def chunk_with_enrichment(input_file: str, output_dir: str, chunk_size: int = 512, chunk_overlap: int = 50):
    """Chunk document with page numbers, headers, and keywords"""
    
    input_path = Path(input_file)
    output_dir = Path(output_dir)
    
    print(f"Loading parsed document: {input_path}")
    
    # Load the JSON file
    with open(input_path, "r", encoding="utf-8") as f:
        data = json.load(f)
    
    raw_text = data.get("raw_text", "")
    metadata = data.get("metadata", {})
    pages_data = data.get("pages", [])
    
    print(f"Text length: {len(raw_text)} characters")
    print(f"Pages available: {len(pages_data)}")
    
    # Initialize keyword extractor
    print("Initializing keyword extractor...")
    kw_extractor = KeywordExtractor()
    
    start_time = datetime.now()
    
    # Create LlamaIndex document
    doc = Document(
        text=raw_text,
        metadata={
            "filename": metadata.get("filename", "unknown"),
            "pages_extracted": metadata.get("pages_extracted", 0),
        }
    )
    
    # Create sentence splitter
    splitter = SentenceSplitter(
        chunk_size=chunk_size,
        chunk_overlap=chunk_overlap,
        paragraph_separator="\n\n",
    )
    
    # Split into nodes/chunks
    nodes = splitter.get_nodes_from_documents([doc])
    
    print(f"Created {len(nodes)} chunks, now enriching...")
    
    # Track all keywords for global stats
    all_keywords = []
    
    # Convert to enriched format
    chunks = []
    current_section = None
    
    for i, node in enumerate(nodes):
        chunk_text = node.get_content()
        
        # Detect section header
        detected_header = detect_section_header(chunk_text)
        if detected_header:
            current_section = detected_header
        
        # Get page number
        page_num = extract_page_number(chunk_text)
        if page_num is None and node.start_char_idx is not None:
            page_num = get_page_for_position(pages_data, node.start_char_idx)
        
        # Extract keywords
        keywords = kw_extractor.extract(chunk_text)
        all_keywords.extend([k["keyword"] for k in keywords])
        
        # Clean text (remove page markers for cleaner chunks)
        clean_text = re.sub(r'--- Page \d+ ---\n?', '', chunk_text).strip()
        
        chunk_data = {
            "chunk_id": i,
            "text": clean_text,
            "char_count": len(clean_text),
            "token_estimate": len(clean_text) // 4,
            "page_number": page_num,
            "section_header": current_section,
            "keywords": keywords,
            "start_char": node.start_char_idx,
            "end_char": node.end_char_idx,
        }
        chunks.append(chunk_data)
        
        if (i + 1) % 10 == 0:
            print(f"  Processed {i + 1}/{len(nodes)} chunks...")
    
    end_time = datetime.now()
    
    # Calculate keyword statistics
    keyword_counts = Counter(all_keywords)
    top_keywords = keyword_counts.most_common(20)
    
    print(f"\nEnrichment completed in {(end_time - start_time).total_seconds():.2f} seconds")
    print(f"Total chunks: {len(chunks)}")
    print(f"Unique keywords extracted: {len(keyword_counts)}")
    
    # Output data
    output_data = {
        "metadata": {
            "source_file": input_path.name,
            "chunked_at": datetime.now().isoformat(),
            "processing_time_seconds": (end_time - start_time).total_seconds(),
            "chunk_size": chunk_size,
            "chunk_overlap": chunk_overlap,
            "total_chunks": len(chunks),
            "original_text_length": len(raw_text),
            "enrichment": {
                "keywords_extracted": True,
                "page_numbers": True,
                "section_headers": True,
            }
        },
        "global_keywords": [{"keyword": kw, "count": count} for kw, count in top_keywords],
        "chunks": chunks,
    }
    
    # Save JSON output
    output_file = output_dir / f"chunked_{input_path.stem}.json"
    with open(output_file, "w", encoding="utf-8") as f:
        json.dump(output_data, f, indent=2, ensure_ascii=False)
    
    print(f"\nOutput saved to: {output_file}")
    
    # Print sample
    print("\n" + "=" * 70)
    print("SAMPLE - First 3 enriched chunks:")
    print("=" * 70)
    
    for chunk in chunks[:3]:
        print(f"\n--- Chunk {chunk['chunk_id']} ---")
        print(f"📄 Page: {chunk['page_number']}")
        print(f"📑 Section: {chunk['section_header']}")
        print(f"🏷️  Keywords: {[k['keyword'] for k in chunk['keywords']]}")
        print(f"📝 Text preview: {chunk['text'][:200]}...")
    
    print("\n" + "=" * 70)
    print("TOP 10 GLOBAL KEYWORDS:")
    print("=" * 70)
    for kw, count in top_keywords[:10]:
        print(f"  • {kw}: {count}")
    
    return output_data


if __name__ == "__main__":
    input_file = "/Users/glennmartin/Projects/GO-Projects/risk-analyzer-go/python-backend/output/114-the-art-of-software-testing-3-edition_first20pages.json"
    output_dir = "/Users/glennmartin/Projects/GO-Projects/risk-analyzer-go/python-backend/output"
    
    chunk_with_enrichment(input_file, output_dir, chunk_size=512, chunk_overlap=50)
