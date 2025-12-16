"""
Document Chunker Service using LlamaIndex
Provides multiple chunking strategies for text splitting
"""

import uuid
import logging
from typing import List, Optional, Dict, Any

from llama_index.core import Document
from llama_index.core.node_parser import (
    SentenceSplitter,
    TokenTextSplitter,
    MarkdownNodeParser,
)
from llama_index.core.schema import TextNode

from ..models import (
    ParsedDocument, 
    ChunkedDocument, 
    TextChunk, 
    ChunkingStrategy
)
from ..config import get_settings

logger = logging.getLogger(__name__)


class DocumentChunker:
    """
    Chunks documents using LlamaIndex node parsers.
    Supports multiple chunking strategies for different use cases.
    """
    
    def __init__(self):
        """Initialize the document chunker"""
        self.settings = get_settings()
        logger.info("DocumentChunker initialized with LlamaIndex")
    
    def chunk_document(
        self,
        parsed_doc: ParsedDocument,
        strategy: ChunkingStrategy = ChunkingStrategy.SENTENCE,
        chunk_size: int = 512,
        chunk_overlap: int = 50,
        include_metadata: bool = True
    ) -> ChunkedDocument:
        """
        Chunk a parsed document using the specified strategy.
        
        Args:
            parsed_doc: The parsed document to chunk
            strategy: Chunking strategy to use
            chunk_size: Target size for each chunk
            chunk_overlap: Overlap between chunks
            include_metadata: Whether to include document metadata in chunks
            
        Returns:
            ChunkedDocument with text chunks
        """
        logger.info(f"Chunking document {parsed_doc.document_id} with strategy: {strategy}")
        
        # Get the text to chunk (prefer markdown for better structure preservation)
        text_to_chunk = parsed_doc.markdown_text or parsed_doc.raw_text
        
        # Create LlamaIndex document
        doc_metadata = {
            "document_id": parsed_doc.document_id,
            "filename": parsed_doc.metadata.filename,
            "file_type": parsed_doc.metadata.file_type,
        }
        
        if parsed_doc.metadata.title:
            doc_metadata["title"] = parsed_doc.metadata.title
        
        llama_doc = Document(
            text=text_to_chunk,
            metadata=doc_metadata if include_metadata else {}
        )
        
        # Get the appropriate splitter
        splitter = self._get_splitter(strategy, chunk_size, chunk_overlap)
        
        # Split into nodes
        nodes = splitter.get_nodes_from_documents([llama_doc])
        
        # Convert to our TextChunk format
        chunks = self._nodes_to_chunks(nodes, parsed_doc.document_id)
        
        # Also chunk tables separately if present
        if parsed_doc.tables:
            table_chunks = self._chunk_tables(parsed_doc.tables, parsed_doc.document_id, len(chunks))
            chunks.extend(table_chunks)
        
        chunked_doc = ChunkedDocument(
            document_id=parsed_doc.document_id,
            metadata=parsed_doc.metadata,
            chunks=chunks,
            total_chunks=len(chunks),
            chunking_strategy=strategy,
            chunk_size=chunk_size,
            chunk_overlap=chunk_overlap
        )
        
        logger.info(f"Created {len(chunks)} chunks from document {parsed_doc.document_id}")
        
        return chunked_doc
    
    def chunk_text(
        self,
        text: str,
        strategy: ChunkingStrategy = ChunkingStrategy.SENTENCE,
        chunk_size: int = 512,
        chunk_overlap: int = 50,
        metadata: Optional[Dict[str, Any]] = None
    ) -> List[TextChunk]:
        """
        Chunk raw text directly.
        
        Args:
            text: Text to chunk
            strategy: Chunking strategy
            chunk_size: Target chunk size
            chunk_overlap: Overlap between chunks
            metadata: Optional metadata to attach to chunks
            
        Returns:
            List of TextChunk objects
        """
        doc_id = str(uuid.uuid4())
        
        llama_doc = Document(
            text=text,
            metadata=metadata or {}
        )
        
        splitter = self._get_splitter(strategy, chunk_size, chunk_overlap)
        nodes = splitter.get_nodes_from_documents([llama_doc])
        
        return self._nodes_to_chunks(nodes, doc_id)
    
    def _get_splitter(
        self, 
        strategy: ChunkingStrategy, 
        chunk_size: int, 
        chunk_overlap: int
    ):
        """Get the appropriate splitter for the strategy"""
        
        if strategy == ChunkingStrategy.SENTENCE:
            return SentenceSplitter(
                chunk_size=chunk_size,
                chunk_overlap=chunk_overlap,
                paragraph_separator="\n\n",
                secondary_chunking_regex="[^,.;。?!]+[,.;。?!]?",
            )
        
        elif strategy == ChunkingStrategy.TOKEN:
            return TokenTextSplitter(
                chunk_size=chunk_size,
                chunk_overlap=chunk_overlap,
            )
        
        elif strategy == ChunkingStrategy.MARKDOWN:
            return MarkdownNodeParser()
        
        elif strategy == ChunkingStrategy.RECURSIVE:
            # Use sentence splitter with more aggressive splitting
            return SentenceSplitter(
                chunk_size=chunk_size,
                chunk_overlap=chunk_overlap,
                paragraph_separator="\n\n",
            )
        
        elif strategy == ChunkingStrategy.SEMANTIC:
            # Semantic chunking requires embeddings - fall back to sentence for now
            # TODO: Implement SemanticSplitterNodeParser when embedding model is configured
            logger.warning("Semantic chunking not fully implemented, using sentence splitter")
            return SentenceSplitter(
                chunk_size=chunk_size,
                chunk_overlap=chunk_overlap,
            )
        
        else:
            # Default to sentence splitter
            return SentenceSplitter(
                chunk_size=chunk_size,
                chunk_overlap=chunk_overlap,
            )
    
    def _nodes_to_chunks(self, nodes: List[TextNode], document_id: str) -> List[TextChunk]:
        """Convert LlamaIndex nodes to our TextChunk format"""
        chunks = []
        
        for idx, node in enumerate(nodes):
            chunk = TextChunk(
                id=node.node_id or f"{document_id}_chunk_{idx}",
                text=node.get_content(),
                metadata={
                    **node.metadata,
                    "document_id": document_id,
                },
                page_number=node.metadata.get("page_label"),
                start_char=node.start_char_idx,
                end_char=node.end_char_idx,
                chunk_index=idx,
                token_count=self._estimate_tokens(node.get_content())
            )
            chunks.append(chunk)
        
        return chunks
    
    def _chunk_tables(
        self, 
        tables: List[Dict[str, Any]], 
        document_id: str,
        start_index: int
    ) -> List[TextChunk]:
        """Create chunks from extracted tables"""
        chunks = []
        
        for idx, table in enumerate(tables):
            table_text = table.get("markdown", str(table))
            
            chunk = TextChunk(
                id=f"{document_id}_table_{idx}",
                text=table_text,
                metadata={
                    "document_id": document_id,
                    "content_type": "table",
                    "table_index": idx,
                    "page": table.get("page"),
                },
                page_number=table.get("page"),
                chunk_index=start_index + idx,
                token_count=self._estimate_tokens(table_text)
            )
            chunks.append(chunk)
        
        return chunks
    
    def _estimate_tokens(self, text: str) -> int:
        """Estimate token count (rough approximation: ~4 chars per token)"""
        return len(text) // 4
    
    def get_available_strategies(self) -> List[str]:
        """Get list of available chunking strategies"""
        return [s.value for s in ChunkingStrategy]
