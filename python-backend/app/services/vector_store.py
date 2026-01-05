"""
Vector Store Service using ChromaDB HTTP Client
Handles embedding storage and similarity search
Connects to ChromaDB server running in Docker
"""

import json
import logging
from typing import Any, Dict, List, Optional

import chromadb
from sentence_transformers import SentenceTransformer

from ..config import get_settings
from ..models import (
    ChunkedDocument,
    TextChunk,
    VectorSearchResult,
)

logger = logging.getLogger(__name__)


class VectorStoreService:
    """
    Manages vector embeddings using ChromaDB HTTP Client.
    Connects to a separate ChromaDB server for persistence.
    """

    def __init__(self):
        """Initialize the vector store service"""
        self.settings = get_settings()

        # Initialize embedding model
        self.embedding_model = SentenceTransformer(self.settings.embedding_model)
        logger.info(f"Loaded embedding model: {self.settings.embedding_model}")

        # Initialize ChromaDB HTTP client (connects to ChromaDB server)
        self.chroma_client = chromadb.HttpClient(
            host=self.settings.chroma_host,
            port=self.settings.chroma_port,
        )

        logger.info(
            f"ChromaDB client connected to: {self.settings.chroma_host}:{self.settings.chroma_port}"
        )

    def get_or_create_collection(
        self, name: Optional[str] = None
    ) -> chromadb.Collection:
        """Get or create a ChromaDB collection"""
        collection_name = name or self.settings.chroma_collection_name

        return self.chroma_client.get_or_create_collection(
            name=collection_name,
            metadata={"hnsw:space": "cosine"},  # Use cosine similarity
        )

    def store_chunks(
        self, chunked_doc: ChunkedDocument, collection_name: Optional[str] = None
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
                elif isinstance(value, (list, dict)):
                    # Serialize complex types to JSON string for ChromaDB
                    try:
                        metadata[key] = json.dumps(value)
                    except Exception:
                        pass

            metadatas.append(metadata)
            embeddings.append(embedding.tolist())

        # Add to collection
        collection.add(
            ids=ids, documents=documents, metadatas=metadatas, embeddings=embeddings
        )

        logger.info(f"Stored {len(ids)} chunks in collection '{collection.name}'")

        return len(ids)

    def search(
        self,
        query: str,
        collection_name: Optional[str] = None,
        top_k: int = 5,
        filter_metadata: Optional[Dict[str, Any]] = None,
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
        # Get collection - fail if it doesn't exist
        collection_name_to_use = collection_name or self.settings.chroma_collection_name
        try:
            collection = self.chroma_client.get_collection(collection_name_to_use)
        except Exception as e:
            raise ValueError(
                f"Collection '{collection_name_to_use}' does not exist. Available collections: {', '.join(self.list_collections())}"
            )

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
            include=["documents", "metadatas", "distances"],
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
                    metadata=results["metadatas"][0][idx]
                    if results["metadatas"]
                    else {},
                )
                search_results.append(result)

        logger.info(
            f"Search returned {len(search_results)} results for query: '{query[:50]}...'"
        )

        return search_results

    def delete_document(
        self, document_id: str, collection_name: Optional[str] = None
    ) -> int:
        """
        Delete all chunks for a document.

        Args:
            document_id: Document ID to delete
            collection_name: Collection name

        Returns:
            Number of chunks deleted
        """
        # Get collection - must exist before deleting from it
        collection_name_to_use = collection_name or self.settings.chroma_collection_name
        try:
            collection = self.chroma_client.get_collection(collection_name_to_use)
        except Exception as e:
            raise ValueError(
                f"Collection '{collection_name_to_use}' does not exist. Available collections: {', '.join(self.list_collections())}"
            )

        # Get chunks for this document
        results = collection.get(
            where={"document_id": document_id}, include=["metadatas"]
        )

        if results["ids"]:
            collection.delete(ids=results["ids"])
            logger.info(
                f"Deleted {len(results['ids'])} chunks for document {document_id}"
            )
            return len(results["ids"])

        return 0

    def list_collections(self) -> List[str]:
        """List all collections"""
        collections = self.chroma_client.list_collections()
        return [c.name for c in collections]

    def get_collection_stats(
        self, collection_name: Optional[str] = None
    ) -> Dict[str, Any]:
        """Get statistics for a collection"""
        # Get collection - must exist to get stats
        collection_name_to_use = collection_name or self.settings.chroma_collection_name
        try:
            collection = self.chroma_client.get_collection(collection_name_to_use)
        except Exception as e:
            raise ValueError(
                f"Collection '{collection_name_to_use}' does not exist. Available collections: {', '.join(self.list_collections())}"
            )

        return {
            "name": collection.name,
            "count": collection.count(),
            "metadata": collection.metadata,
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

    def list_documents(
        self, collection_name: Optional[str] = None
    ) -> List[Dict[str, Any]]:
        """
        List all unique documents in the collection(s).

        If collection_name is provided, only lists documents from that collection.
        If collection_name is None, lists documents from ALL collections.

        Returns:
            List of documents with their metadata and chunk counts
        """
        # If a specific collection is requested, only search that one
        if collection_name:
            return self._list_documents_from_collection(collection_name)

        # Otherwise, aggregate from all collections
        all_documents: Dict[str, Dict[str, Any]] = {}
        collections = self.chroma_client.list_collections()

        logger.info(f"Listing documents from {len(collections)} collections")

        for coll in collections:
            try:
                total_count = coll.count()
                if total_count == 0:
                    continue

                results = coll.get(include=["metadatas"], limit=total_count)

                for metadata in results["metadatas"] or []:
                    doc_id = metadata.get("document_id", "unknown")

                    if doc_id not in all_documents:
                        all_documents[doc_id] = {
                            "document_id": doc_id,
                            "filename": metadata.get("filename"),
                            "title": metadata.get("title"),
                            "chunk_count": 0,
                            "collection": coll.name,
                        }

                    all_documents[doc_id]["chunk_count"] += 1

            except Exception as e:
                logger.warning(
                    f"Error listing documents from collection '{coll.name}': {e}"
                )
                continue

        logger.info(
            f"Found {len(all_documents)} unique documents across all collections"
        )
        return list(all_documents.values())

    def _list_documents_from_collection(
        self, collection_name: str
    ) -> List[Dict[str, Any]]:
        """List documents from a specific collection"""
        try:
            collection = self.chroma_client.get_collection(collection_name)
        except Exception as e:
            raise ValueError(
                f"Collection '{collection_name}' does not exist. Available collections: {', '.join(self.list_collections())}"
            )

        total_count = collection.count()
        logger.info(f"Collection '{collection.name}' has {total_count} total chunks")

        if total_count == 0:
            logger.info("No chunks found in collection")
            return []

        results = collection.get(include=["metadatas"], limit=total_count)

        logger.info(f"Retrieved {len(results.get('ids', []))} chunks from collection")

        # Group by document_id
        documents: Dict[str, Dict[str, Any]] = {}

        for metadata in results["metadatas"] or []:
            doc_id = metadata.get("document_id", "unknown")

            if doc_id not in documents:
                documents[doc_id] = {
                    "document_id": doc_id,
                    "filename": metadata.get("filename"),
                    "title": metadata.get("title"),
                    "chunk_count": 0,
                    "collection": collection_name,
                }

            documents[doc_id]["chunk_count"] += 1

        logger.info(f"Found {len(documents)} unique documents in collection")
        return list(documents.values())

    def get_all_chunks(
        self,
        collection_name: Optional[str] = None,
        limit: int = 100,
        offset: int = 0,
        document_id: Optional[str] = None,
    ) -> Dict[str, Any]:
        """
        Get all chunks from the collection with pagination.

        Args:
            collection_name: Collection to query
            limit: Maximum number of chunks to return
            offset: Number of chunks to skip
            document_id: Optional filter by document_id

        Returns:
            Dict with chunks and pagination info
        """
        # Get collection - must exist to get chunks
        collection_name_to_use = collection_name or self.settings.chroma_collection_name
        try:
            collection = self.chroma_client.get_collection(collection_name_to_use)
        except Exception as e:
            raise ValueError(
                f"Collection '{collection_name_to_use}' does not exist. Available collections: {', '.join(self.list_collections())}"
            )

        # Build where clause if document_id provided
        where = {"document_id": document_id} if document_id else None

        # Get all items
        results = collection.get(
            where=where,
            include=["documents", "metadatas"],
            limit=limit,
            offset=offset,
        )

        chunks = []
        if results["ids"]:
            for idx, chunk_id in enumerate(results["ids"]):
                metadata = results["metadatas"][idx] if results["metadatas"] else {}

                # Auto-parse JSON strings back to objects
                for key, value in metadata.items():
                    if isinstance(value, str) and (
                        value.startswith("[") or value.startswith("{")
                    ):
                        try:
                            metadata[key] = json.loads(value)
                        except (json.JSONDecodeError, TypeError):
                            pass

                chunk = {
                    "id": chunk_id,
                    "text": results["documents"][idx] if results["documents"] else "",
                    "metadata": metadata,
                }
                chunks.append(chunk)

        return {
            "chunks": chunks,
            "count": len(chunks),
            "limit": limit,
            "offset": offset,
        }
