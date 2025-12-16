"""
Vector Store Service using ChromaDB
Handles embedding storage and similarity search
"""

import logging
from typing import List, Optional, Dict, Any

import chromadb
from chromadb.config import Settings as ChromaSettings
from sentence_transformers import SentenceTransformer

from ..models import (
    ChunkedDocument, 
    TextChunk,
    VectorSearchResult,
)
from ..config import get_settings

logger = logging.getLogger(__name__)


class VectorStoreService:
    """
    Manages vector embeddings using ChromaDB.
    Supports storing chunks and similarity search.
    """
    
    def __init__(self):
        """Initialize the vector store service"""
        self.settings = get_settings()
        
        # Initialize embedding model
        self.embedding_model = SentenceTransformer(self.settings.embedding_model)
        logger.info(f"Loaded embedding model: {self.settings.embedding_model}")
        
        # Initialize ChromaDB client with persistence
        self.chroma_client = chromadb.PersistentClient(
            path=self.settings.chroma_persist_dir,
            settings=ChromaSettings(
                anonymized_telemetry=False,
                allow_reset=True,
            )
        )
        
        logger.info(f"ChromaDB initialized at: {self.settings.chroma_persist_dir}")
    
    def get_or_create_collection(self, name: Optional[str] = None) -> chromadb.Collection:
        """Get or create a ChromaDB collection"""
        collection_name = name or self.settings.chroma_collection_name
        
        return self.chroma_client.get_or_create_collection(
            name=collection_name,
            metadata={"hnsw:space": "cosine"}  # Use cosine similarity
        )
    
    def store_chunks(
        self,
        chunked_doc: ChunkedDocument,
        collection_name: Optional[str] = None
    ) -> int:
        """
        Store document chunks in the vector database.
        
        Args:
            chunked_doc: The chunked document to store
            collection_name: Optional collection name (uses default if not provided)
            
        Returns:
            Number of chunks stored
        """
        collection = self.get_or_create_collection(collection_name)
        
        # Prepare data for ChromaDB
        ids = []
        documents = []
        metadatas = []
        embeddings = []
        
        # Generate embeddings for all chunks
        texts = [chunk.text for chunk in chunked_doc.chunks]
        chunk_embeddings = self.embedding_model.encode(texts, show_progress_bar=True)
        
        for chunk, embedding in zip(chunked_doc.chunks, chunk_embeddings):
            ids.append(chunk.id)
            documents.append(chunk.text)
            
            # Prepare metadata (ChromaDB only supports string, int, float, bool)
            metadata = {
                "document_id": chunked_doc.document_id,
                "filename": chunked_doc.metadata.filename,
                "chunk_index": chunk.chunk_index,
            }
            
            if chunk.page_number is not None:
                metadata["page_number"] = chunk.page_number
            if chunk.token_count is not None:
                metadata["token_count"] = chunk.token_count
            if chunked_doc.metadata.title:
                metadata["title"] = chunked_doc.metadata.title
            
            # Add any string/numeric values from chunk metadata
            for key, value in chunk.metadata.items():
                if isinstance(value, (str, int, float, bool)):
                    metadata[key] = value
            
            metadatas.append(metadata)
            embeddings.append(embedding.tolist())
        
        # Add to collection
        collection.add(
            ids=ids,
            documents=documents,
            metadatas=metadatas,
            embeddings=embeddings
        )
        
        logger.info(f"Stored {len(ids)} chunks in collection '{collection.name}'")
        
        return len(ids)
    
    def search(
        self,
        query: str,
        collection_name: Optional[str] = None,
        top_k: int = 5,
        filter_metadata: Optional[Dict[str, Any]] = None
    ) -> List[VectorSearchResult]:
        """
        Search for similar chunks.
        
        Args:
            query: Search query text
            collection_name: Collection to search in
            top_k: Number of results to return
            filter_metadata: Optional metadata filter
            
        Returns:
            List of search results
        """
        collection = self.get_or_create_collection(collection_name)
        
        # Generate query embedding
        query_embedding = self.embedding_model.encode([query])[0].tolist()
        
        # Build where clause if filter provided
        where = None
        if filter_metadata:
            where = {k: v for k, v in filter_metadata.items() if v is not None}
        
        # Query ChromaDB
        results = collection.query(
            query_embeddings=[query_embedding],
            n_results=top_k,
            where=where if where else None,
            include=["documents", "metadatas", "distances"]
        )
        
        # Convert to our format
        search_results = []
        
        if results["ids"] and results["ids"][0]:
            for idx, chunk_id in enumerate(results["ids"][0]):
                # Convert distance to similarity score (1 - distance for cosine)
                distance = results["distances"][0][idx] if results["distances"] else 0
                score = 1 - distance
                
                result = VectorSearchResult(
                    chunk_id=chunk_id,
                    text=results["documents"][0][idx] if results["documents"] else "",
                    score=score,
                    metadata=results["metadatas"][0][idx] if results["metadatas"] else {}
                )
                search_results.append(result)
        
        logger.info(f"Search returned {len(search_results)} results for query: '{query[:50]}...'")
        
        return search_results
    
    def delete_document(self, document_id: str, collection_name: Optional[str] = None) -> int:
        """
        Delete all chunks for a document.
        
        Args:
            document_id: Document ID to delete
            collection_name: Collection name
            
        Returns:
            Number of chunks deleted
        """
        collection = self.get_or_create_collection(collection_name)
        
        # Get chunks for this document
        results = collection.get(
            where={"document_id": document_id},
            include=["metadatas"]
        )
        
        if results["ids"]:
            collection.delete(ids=results["ids"])
            logger.info(f"Deleted {len(results['ids'])} chunks for document {document_id}")
            return len(results["ids"])
        
        return 0
    
    def list_collections(self) -> List[str]:
        """List all collections"""
        collections = self.chroma_client.list_collections()
        return [c.name for c in collections]
    
    def get_collection_stats(self, collection_name: Optional[str] = None) -> Dict[str, Any]:
        """Get statistics for a collection"""
        collection = self.get_or_create_collection(collection_name)
        
        return {
            "name": collection.name,
            "count": collection.count(),
            "metadata": collection.metadata
        }
    
    def reset_collection(self, collection_name: Optional[str] = None):
        """Delete and recreate a collection"""
        name = collection_name or self.settings.chroma_collection_name
        
        try:
            self.chroma_client.delete_collection(name)
            logger.info(f"Deleted collection: {name}")
        except ValueError:
            pass  # Collection doesn't exist
        
        self.get_or_create_collection(name)
        logger.info(f"Created new collection: {name}")
