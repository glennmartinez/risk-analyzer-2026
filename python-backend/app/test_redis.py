
import sys
import os
from pathlib import Path
import logging

# Add app to path
sys.path.insert(0, str(Path(__file__).parent.parent))

from app.services.redis_service import DocumentRegistry

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

def test_redis_service():
    print("Testing Redis DocumentRegistry...")
    try:
        registry = DocumentRegistry()
        print("✅ Connected to Redis")
        
        # Test 1: Register
        doc_id = "test_doc_123"
        metadata = {
            "filename": "test.pdf",
            "chunk_count": 10,
            "collection": "test_docs",
            "stored_in_vector_db": True
        }
        
        success = registry.register_document(doc_id, metadata)
        if success:
            print(f"✅ Registered document {doc_id}")
        else:
            print(f"❌ Failed to register document {doc_id}")
            return # Stop if failed
            
        # Test 2: Get
        fetched = registry.get_document(doc_id)
        if fetched:
            print(f"✅ Fetched document: {fetched}")
            if fetched["filename"] == "test.pdf":
                 print("   Metadata matches")
        else:
             print("❌ Failed to fetch document")
             
        # Test 3: List
        docs = registry.list_documents()
        found = False
        for d in docs:
            if d.get("document_id") == doc_id:
                found = True
                break
        
        if found:
            print(f"✅ Found document in list. Total docs: {len(docs)}")
        else:
            print("❌ Document not found in list")
            
        # Test 4: Delete
        # success = registry.delete_document(doc_id)
        # if success:
        #    print(f"✅ Deleted document {doc_id}")
        # else:
        #    print("❌ Failed to delete document")
        
    except Exception as e:
        print(f"❌ Error: {e}")
        import traceback
        traceback.print_exc()

if __name__ == "__main__":
    test_redis_service()
