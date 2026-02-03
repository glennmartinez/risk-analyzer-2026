
import logging
from app.services.redis_service import DocumentRegistry

# Configure logging
logging.basicConfig(level=logging.ERROR)

def check_doc(doc_id):
    registry = DocumentRegistry()
    print("Listing all docs...")
    docs = registry.list_documents()
    print(f"Found {len(docs)} docs")
    for d in docs:
        print(f"ID: {d.get('document_id')}")
        if d.get('document_id') == doc_id:
            print(f"MATCH FOUND: {d}")

