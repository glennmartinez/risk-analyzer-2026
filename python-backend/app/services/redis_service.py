import json
import logging
from typing import Dict, List, Optional
from datetime import datetime

import redis
from app.config import get_settings

logger = logging.getLogger(__name__)

class DocumentRegistry:
    """
    Service to manage document metadata in Redis.
    Structure:
    - doc:{document_id} -> Hash of metadata
    - docs:all -> Set of all document_ids
    """

    def __init__(self):
        settings = get_settings()
        try:
            self.client = redis.Redis(
                host=settings.redis_host,
                port=settings.redis_port,
                db=settings.redis_db,
                password=settings.redis_password,
                decode_responses=True
            )
            self.client.ping() # Check connection
        except redis.ConnectionError as e:
            logger.error(f"Failed to connect to Redis: {e}")
            raise

    def register_document(self, document_id: str, metadata: Dict) -> bool:
        """Register a new document or update existing one"""
        try:
            # Ensure metadata is flat and stringified for Redis Hash
            flat_meta = {
                "document_id": document_id,
                "registered_at": datetime.now().isoformat(),
            }
            
            # Merge provided metadata
            for k, v in metadata.items():
                if isinstance(v, (dict, list, bool, int, float)):
                    flat_meta[k] = str(v)
                else:
                    flat_meta[k] = v

            # Transaction
            pipe = self.client.pipeline()
            pipe.hset(f"doc:{document_id}", mapping=flat_meta)
            pipe.sadd("docs:all", document_id)
            pipe.execute()
            
            logger.info(f"Registered document {document_id} in Redis")
            return True
        except Exception as e:
            logger.error(f"Error registering document {document_id}: {e}")
            return False

    def get_document(self, document_id: str) -> Optional[Dict]:
        """Get document metadata"""
        try:
            doc = self.client.hgetall(f"doc:{document_id}")
            if not doc:
                return None
            return doc
        except Exception as e:
            logger.error(f"Error getting document {document_id}: {e}")
            return None

    def list_documents(self) -> List[Dict]:
        """List all registered documents"""
        try:
            doc_ids = self.client.smembers("docs:all")
            documents = []
            
            for doc_id in doc_ids:
                doc = self.get_document(doc_id)
                if doc:
                    documents.append(doc)
            
            return documents
        except Exception as e:
            logger.error(f"Error listing documents: {e}")
            return []

    def delete_document(self, document_id: str) -> bool:
        """Delete a document from registry"""
        try:
            pipe = self.client.pipeline()
            pipe.delete(f"doc:{document_id}")
            pipe.srem("docs:all", document_id)
            pipe.execute()
            logger.info(f"Deleted document {document_id} from Redis")
            return True
        except Exception as e:
            logger.error(f"Error deleting document {document_id}: {e}")
            return False
