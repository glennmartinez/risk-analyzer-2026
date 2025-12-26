"""
Test script for the DocumentChunker with metadata extraction
Tests the new title and questions extraction features via LM Studio
"""

import json
import sys
from datetime import datetime
from pathlib import Path

# Add the app directory to the path
sys.path.insert(0, str(Path(__file__).parent.parent))

from app.services.chunker import DocumentChunker
from app.services.parser import DocumentParser
from app.services.vector_store import VectorStoreService
from app.models import ChunkingStrategy


def test_chunker(
    pdf_path: str,
    output_dir: str = "./output",
    extract_metadata: bool = True,
    num_questions: int = 3,
    store_in_db: bool = False,
    collection_name: str = None,
):
    """
    Test the chunker with metadata extraction.
    
    Args:
        pdf_path: Path to a PDF file to test
        output_dir: Directory to save output files
        extract_metadata: Whether to extract title/questions via LLM
        num_questions: Number of questions per chunk
        store_in_db: Whether to store chunks in ChromaDB
        collection_name: ChromaDB collection name (uses default if None)
    """
    pdf_path = Path(pdf_path)
    output_dir = Path(output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)
    
    if not pdf_path.exists():
        print(f"❌ File not found: {pdf_path}")
        return
    
    print(f"📄 Testing chunker with: {pdf_path.name}")
    print(f"📁 Output directory: {output_dir}")
    print(f"   Extract metadata: {extract_metadata}")
    print(f"   Questions per chunk: {num_questions}")
    print("-" * 60)
    
    # Step 1: Parse the document
    print("🔄 Step 1: Parsing document...")
    parser = DocumentParser()
    parsed_doc = parser.parse_file(str(pdf_path))
    print(f"   ✅ Parsed: {len(parsed_doc.raw_text):,} chars")
    
    # Step 2: Chunk the document
    print("🔄 Step 2: Chunking document...")
    chunker = DocumentChunker()
    
    chunked_doc = chunker.chunk_document(
        parsed_doc,
        strategy=ChunkingStrategy.SENTENCE,
        chunk_size=512,
        chunk_overlap=50,
        extract_metadata=extract_metadata,
        num_questions=num_questions,
    )
    
    print(f"   ✅ Created {chunked_doc.total_chunks} chunks")
    print("-" * 60)
    
    # Step 3: Display sample chunks
    print("📝 Sample chunks with metadata:")
    print("=" * 60)
    
    for i, chunk in enumerate(chunked_doc.chunks[:3]):
        print(f"\n--- Chunk {i + 1} ---")
        print(f"Text (first 200 chars): {chunk.text[:200]}...")
        print(f"Token count: {chunk.token_count}")
        
        if extract_metadata:
            title = chunk.metadata.get("document_title", "N/A")
            questions = chunk.metadata.get("questions_this_excerpt_can_answer", [])
            
            print(f"📌 Document Title: {title}")
            print(f"❓ Questions this chunk answers:")
            if questions:
                for q in questions:
                    print(f"   • {q}")
            else:
                print("   (none extracted)")
        
        print()
    
    # Step 4: Save results to JSON
    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    output_path = output_dir / f"{pdf_path.stem}_chunks_{timestamp}.json"
    
    chunks_data = [
        {
            "id": chunk.id,
            "text": chunk.text,
            "token_count": chunk.token_count,
            "chunk_index": chunk.chunk_index,
            "metadata": chunk.metadata,
        }
        for chunk in chunked_doc.chunks
    ]
    
    output_json = {
        "document_id": chunked_doc.document_id,
        "filename": pdf_path.name,
        "processed_at": datetime.now().isoformat(),
        "chunking_strategy": str(chunked_doc.chunking_strategy),
        "chunk_size": chunked_doc.chunk_size,
        "chunk_overlap": chunked_doc.chunk_overlap,
        "total_chunks": chunked_doc.total_chunks,
        "extract_metadata": extract_metadata,
        "chunks": chunks_data,
    }
    
    with open(output_path, "w") as f:
        json.dump(output_json, f, indent=2, default=str)
    
    print(f"💾 Saved {len(chunks_data)} chunks to: {output_path}")
    
    # Step 5: Optionally store in ChromaDB
    if store_in_db:
        print("🔄 Step 5: Storing chunks in ChromaDB...")
        try:
            vector_store = VectorStoreService()
            stored_count = vector_store.store_chunks(chunked_doc, collection_name)
            print(f"   ✅ Stored {stored_count} chunks in ChromaDB")
        except Exception as e:
            print(f"   ❌ Failed to store in ChromaDB: {e}")
    
    print("✅ Test complete!")


if __name__ == "__main__":
    # Default paths
    script_dir = Path(__file__).parent
    default_pdf = script_dir / "example_data" / "114-the-art-of-software-testing-3-edition.pdf"
    default_output = script_dir.parent / "output"
    
    # Allow command line args
    pdf_path = sys.argv[1] if len(sys.argv) > 1 else str(default_pdf)
    output_dir = sys.argv[2] if len(sys.argv) > 2 else str(default_output)
    extract = "--no-extract" not in sys.argv
    store_db = "--store-db" in sys.argv
    
    try:
        test_chunker(pdf_path, output_dir, extract_metadata=extract, store_in_db=store_db)
    except Exception as e:
        print(f"❌ Error: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)
