"""
RAG (Retrieval) routes - for Go backend to call
"""

import json
import logging
import time
from pathlib import Path
from typing import List, Optional

import chromadb
from fastapi import APIRouter, HTTPException, Query
from pydantic import BaseModel, Field
from sentence_transformers import SentenceTransformer

from app.config import get_settings

logger = logging.getLogger(__name__)
router = APIRouter(prefix="/rag", tags=["rag"])

# Global instances (initialized on first use)
_embedding_model = None
_chroma_client = None


def get_embedding_model():
    global _embedding_model
    if _embedding_model is None:
        settings = get_settings()
        logger.info(f"Loading embedding model: {settings.embedding_model}")
        _embedding_model = SentenceTransformer(settings.embedding_model)
    return _embedding_model


def get_chroma_client():
    global _chroma_client
    if _chroma_client is None:
        settings = get_settings()
        logger.info(
            f"Connecting to ChromaDB at: {settings.chroma_host}:{settings.chroma_port}"
        )
        _chroma_client = chromadb.HttpClient(
            host=settings.chroma_host,
            port=settings.chroma_port,
        )
    return _chroma_client


# === Request/Response Models ===


class RAGSearchRequest(BaseModel):
    query: str = Field(..., description="Search query")
    collection: str = Field(default="documents", description="Collection to search")
    top_k: int = Field(default=5, ge=1, le=20, description="Number of results")
    filter_domain: Optional[str] = Field(default=None, description="Filter by domain")


class RAGSearchResult(BaseModel):
    chunk_id: str
    text: str
    score: float
    page_number: Optional[int] = None
    section_header: Optional[str] = None
    keywords: Optional[str] = None
    doc_name: Optional[str] = None


class RAGSearchResponse(BaseModel):
    query: str
    results: List[RAGSearchResult]
    total_results: int
    search_time_ms: float


class LoadChunksRequest(BaseModel):
    chunks_file: str = Field(..., description="Path to chunked JSON file")
    collection: str = Field(default="documents", description="Collection name")
    domain: str = Field(default="general", description="Domain classification")
    doc_name: str = Field(default="unknown", description="Document name")


class LoadChunksResponse(BaseModel):
    success: bool
    message: str
    chunks_loaded: int
    collection: str


# === Endpoints ===


@router.post(
    "/search",
    response_model=RAGSearchResponse,
    summary="RAG Search",
    description="Search for relevant chunks to use in RAG pipeline",
)
async def rag_search(request: RAGSearchRequest):
    """
    Semantic search for RAG - returns chunks to feed to LLM.
    Called by Go backend before LLM call.
    """
    start_time = time.time()

    try:
        model = get_embedding_model()
        client = get_chroma_client()

        # Get collection
        try:
            collection = client.get_collection(request.collection)
        except Exception:
            raise HTTPException(
                status_code=404, detail=f"Collection '{request.collection}' not found"
            )

        # Embed query
        query_embedding = model.encode([request.query])[0].tolist()

        # Build filter
        where_filter = None
        if request.filter_domain:
            where_filter = {"domain": request.filter_domain}

        # Search
        results = collection.query(
            query_embeddings=[query_embedding],
            n_results=request.top_k,
            where=where_filter,
            include=["documents", "metadatas", "distances"],
        )

        # Format results
        search_results = []
        if results["ids"] and results["ids"][0]:
            for i, chunk_id in enumerate(results["ids"][0]):
                dist = results["distances"][0][i] if results["distances"] else 0
                score = 1 - dist  # Convert distance to similarity

                meta = results["metadatas"][0][i] if results["metadatas"] else {}

                search_results.append(
                    RAGSearchResult(
                        chunk_id=chunk_id,
                        text=results["documents"][0][i] if results["documents"] else "",
                        score=round(score, 4),
                        page_number=meta.get("page_number"),
                        section_header=meta.get("section_header"),
                        keywords=meta.get("keywords"),
                        doc_name=meta.get("doc_name"),
                    )
                )

        elapsed_ms = (time.time() - start_time) * 1000

        return RAGSearchResponse(
            query=request.query,
            results=search_results,
            total_results=len(search_results),
            search_time_ms=round(elapsed_ms, 2),
        )

    except HTTPException:
        raise
    except Exception as e:
        logger.exception(f"RAG search error: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.get(
    "/search",
    response_model=RAGSearchResponse,
    summary="Quick RAG Search (GET)",
    description="Simple GET-based search for testing",
)
async def rag_search_get(
    q: str = Query(..., description="Search query"),
    collection: str = Query(default="documents"),
    top_k: int = Query(default=5, ge=1, le=20),
):
    """GET-based search for easy testing"""
    return await rag_search(
        RAGSearchRequest(query=q, collection=collection, top_k=top_k)
    )


@router.post(
    "/load",
    response_model=LoadChunksResponse,
    summary="Load chunks into ChromaDB",
    description="Load pre-processed chunks from JSON file into vector database",
)
async def load_chunks(request: LoadChunksRequest):
    """
    Load enriched chunks into ChromaDB.
    Used to index new documents.
    """
    try:
        chunks_path = Path(request.chunks_file)

        if not chunks_path.exists():
            raise HTTPException(
                status_code=404, detail=f"File not found: {request.chunks_file}"
            )

        # Load chunks
        with open(chunks_path, "r", encoding="utf-8") as f:
            data = json.load(f)

        chunks = data.get("chunks", [])
        if not chunks:
            raise HTTPException(status_code=400, detail="No chunks found in file")

        logger.info(
            f"Loading {len(chunks)} chunks into collection '{request.collection}'"
        )

        model = get_embedding_model()
        client = get_chroma_client()

        # Create/get collection
        collection = client.get_or_create_collection(
            name=request.collection, metadata={"hnsw:space": "cosine"}
        )

        # Prepare data
        ids = []
        documents = []
        metadatas = []

        texts = [chunk["text"] for chunk in chunks]
        embeddings = model.encode(texts, show_progress_bar=True)

        for chunk, embedding in zip(chunks, embeddings):
            chunk_id = f"{request.doc_name}_{chunk['chunk_id']}"
            ids.append(chunk_id)
            documents.append(chunk["text"])

            metadata = {
                "chunk_id": chunk["chunk_id"],
                "char_count": chunk.get("char_count", len(chunk["text"])),
                "doc_name": request.doc_name,
                "domain": request.domain,
            }

            if chunk.get("page_number"):
                metadata["page_number"] = chunk["page_number"]
            if chunk.get("section_header"):
                metadata["section_header"] = chunk["section_header"]
            if chunk.get("keywords"):
                metadata["keywords"] = ", ".join(
                    [k["keyword"] for k in chunk["keywords"]]
                )

            metadatas.append(metadata)

        # Add to collection
        collection.add(
            ids=ids,
            documents=documents,
            metadatas=metadatas,
            embeddings=[e.tolist() for e in embeddings],
        )

        logger.info(f"Successfully loaded {len(chunks)} chunks")

        return LoadChunksResponse(
            success=True,
            message=f"Loaded {len(chunks)} chunks into '{request.collection}'",
            chunks_loaded=len(chunks),
            collection=request.collection,
        )

    except HTTPException:
        raise
    except Exception as e:
        logger.exception(f"Load chunks error: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.get(
    "/collections",
    summary="List collections",
    description="List all available collections",
)
async def list_collections():
    """List all ChromaDB collections"""
    client = get_chroma_client()
    collections = client.list_collections()

    return {
        "collections": [
            {"name": c.name, "count": c.count(), "metadata": c.metadata}
            for c in collections
        ]
    }


@router.delete(
    "/collections/{collection_name}",
    summary="Delete collection",
    description="Delete a collection and all its data",
)
async def delete_collection(collection_name: str):
    """Delete a collection"""
    client = get_chroma_client()

    try:
        client.delete_collection(collection_name)
        return {"success": True, "message": f"Deleted collection '{collection_name}'"}
    except Exception:
        raise HTTPException(
            status_code=404, detail=f"Collection not found: {collection_name}"
        )
