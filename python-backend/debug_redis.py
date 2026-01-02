
import sys
import logging
import uuid
from app.services.redis_service import DocumentRegistry

# Configure logging
logging.basicConfig(level=logging.INFO)

def test_registry():
    print("Initializing Registry...")
    try:
        registry = DocumentRegistry()
    except Exception as e:
        print(f"Failed to connect: {e}")
        return

    doc_id = f"test_{uuid.uuid4()}"
    print(f"Testing with Doc ID: {doc_id}")

    metadata = {
        "filename": "debug_test.pdf",
        "chunk_count": 42,
        "collection": "debug_collection",
        "file_size": 12345,
        "stored_in_vector_db": True,
        "chunking_strategy": "sentence",
        "chunk_size": 512,
        "chunk_overlap": 50,
        "extract_metadata": True,
        "num_questions": 3,
        "max_pages": 30,
        "llm_provider": "openai",
        "llm_model": "llama-3.2-3b-instruct",
    }

    print("Registering document...")
    registry.register_document(doc_id, metadata)

    print("Reading back document...")
    doc = registry.get_document(doc_id)
    
    # Sort keys for easier reading
    print("\n--- Redis Content ---")
    for k in sorted(doc.keys()):
        print(f"{k}: {doc[k]}")
    print("---------------------\n")

    # Verify fields
    missing = []
    for k in metadata.keys():
        if k not in doc:
            missing.append(k)
    
    if missing:
        print(f"❌ MISSING FIELDS: {missing}")
    else:
        print("✅ ALL FIELDS PRESENT")

    # Cleanup
    # registry.delete_document(doc_id)

if __name__ == "__main__":
    test_registry()
