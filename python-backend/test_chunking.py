"""
Test chunking with LlamaIndex using the parsed PDF output
"""

import json
from pathlib import Path
from datetime import datetime

from llama_index.core import Document
from llama_index.core.node_parser import SentenceSplitter


def chunk_document(input_file: str, output_dir: str, chunk_size: int = 512, chunk_overlap: int = 50):
    """Chunk the parsed document using LlamaIndex"""
    
    input_path = Path(input_file)
    output_dir = Path(output_dir)
    
    print(f"Loading parsed document: {input_path}")
    
    # Load the JSON file
    with open(input_path, "r", encoding="utf-8") as f:
        data = json.load(f)
    
    raw_text = data.get("raw_text", "")
    metadata = data.get("metadata", {})
    
    print(f"Text length: {len(raw_text)} characters")
    print(f"Chunk size: {chunk_size}, Overlap: {chunk_overlap}")
    
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
    
    end_time = datetime.now()
    
    print(f"Chunking completed in {(end_time - start_time).total_seconds():.2f} seconds")
    print(f"Created {len(nodes)} chunks")
    
    # Convert to serializable format
    chunks = []
    for i, node in enumerate(nodes):
        chunk_text = node.get_content()
        chunks.append({
            "chunk_id": i,
            "text": chunk_text,
            "char_count": len(chunk_text),
            "token_estimate": len(chunk_text) // 4,  # Rough estimate
            "start_char": node.start_char_idx,
            "end_char": node.end_char_idx,
        })
    
    # Output data
    output_data = {
        "metadata": {
            "source_file": input_path.name,
            "chunked_at": datetime.now().isoformat(),
            "chunking_time_seconds": (end_time - start_time).total_seconds(),
            "chunk_size": chunk_size,
            "chunk_overlap": chunk_overlap,
            "total_chunks": len(chunks),
            "original_text_length": len(raw_text),
        },
        "chunks": chunks,
    }
    
    # Save JSON output
    output_file = output_dir / f"chunked_{input_path.stem}.json"
    with open(output_file, "w", encoding="utf-8") as f:
        json.dump(output_data, f, indent=2, ensure_ascii=False)
    
    print(f"Output saved to: {output_file}")
    
    # Also save a readable text version
    txt_file = output_dir / f"chunked_{input_path.stem}.txt"
    with open(txt_file, "w", encoding="utf-8") as f:
        f.write(f"Total chunks: {len(chunks)}\n")
        f.write(f"Chunk size: {chunk_size}, Overlap: {chunk_overlap}\n")
        f.write("=" * 80 + "\n\n")
        
        for chunk in chunks:
            f.write(f"--- CHUNK {chunk['chunk_id']} ({chunk['char_count']} chars) ---\n")
            f.write(chunk['text'])
            f.write("\n\n" + "-" * 40 + "\n\n")
    
    print(f"Text version saved to: {txt_file}")
    
    # Print sample
    print("\n" + "=" * 60)
    print("SAMPLE - First 3 chunks:")
    print("=" * 60)
    for chunk in chunks[:3]:
        print(f"\n--- Chunk {chunk['chunk_id']} ({chunk['char_count']} chars) ---")
        print(chunk['text'][:300] + "..." if len(chunk['text']) > 300 else chunk['text'])
    
    return output_data


if __name__ == "__main__":
    input_file = "/Users/glennmartin/Projects/GO-Projects/risk-analyzer-go/python-backend/output/114-the-art-of-software-testing-3-edition_first20pages.json"
    output_dir = "/Users/glennmartin/Projects/GO-Projects/risk-analyzer-go/python-backend/output"
    
    # Chunk with default settings
    chunk_document(input_file, output_dir, chunk_size=512, chunk_overlap=50)
