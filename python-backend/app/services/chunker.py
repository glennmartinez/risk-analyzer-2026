"""
Document Chunker Service using LlamaIndex
Provides multiple chunking strategies for text splitting
Supports metadata extraction (title, questions) via LLM
"""

import logging
import re
import uuid
from typing import Any, Dict, List, Optional

from llama_index.core import Document, Settings
from llama_index.core.extractors import (
    KeywordExtractor,
    QuestionsAnsweredExtractor,
    TitleExtractor,
)
from llama_index.core.node_parser import (
    HierarchicalNodeParser,
    MarkdownNodeParser,
    SemanticSplitterNodeParser,
    SentenceSplitter,
    TokenTextSplitter,
)
from llama_index.core.schema import TextNode
from llama_index.core.utils import get_tokenizer
from llama_index.embeddings.openai import OpenAIEmbedding
from llama_index.llms.openai import OpenAI as OpenAILLM

from ..config import get_settings
from ..models import ChunkedDocument, ChunkingStrategy, ParsedDocument, TextChunk

logger = logging.getLogger(__name__)


class DocumentChunker:
    """
    Chunks documents using LlamaIndex node parsers.
    Supports multiple chunking strategies for different use cases.
    """

    def __init__(self):
        """Initialize the document chunker with optional LLM for metadata extraction"""
        self.settings = get_settings()
        self._llm = None  # Lazy initialization
        self._title_extractor = None
        self._questions_extractor = None
        self._keyword_extractor = None
        logger.info("DocumentChunker initialized with LlamaIndex")

    @property
    def llm(self):
        """Lazy-load LLM for metadata extraction (LM Studio or OpenAI)"""
        if self._llm is None:
            provider = self.settings.llm_provider.lower()

            if provider == "lmstudio":
                # LM Studio exposes an OpenAI-compatible API
                self._llm = OpenAILLM(
                    model=self.settings.llm_model,
                    api_base=self.settings.lmstudio_base_url,
                    api_key="lm-studio",  # LM Studio doesn't require a real key
                    timeout=120.0,
                )
                logger.info(
                    f"Initialized LM Studio LLM at {self.settings.lmstudio_base_url}"
                )
            elif provider == "openai":
                if not self.settings.openai_api_key:
                    raise ValueError(
                        "OpenAI API key required when llm_provider='openai'. "
                        "Set OPENAI_API_KEY environment variable."
                    )
                self._llm = OpenAILLM(
                    model=self.settings.llm_model or "gpt-3.5-turbo",
                    api_key=self.settings.openai_api_key,
                )
                logger.info(f"Initialized OpenAI LLM: {self.settings.llm_model}")
            else:
                raise ValueError(
                    f"Unknown llm_provider: {provider}. Use 'lmstudio' or 'openai'."
                )
        return self._llm

    def _get_title_extractor(self) -> TitleExtractor:
        """Get or create title extractor"""
        if self._title_extractor is None:
            self._title_extractor = TitleExtractor(
                nodes=3,  # Use first 3 nodes to infer title
                llm=self.llm,
            )
        return self._title_extractor

    def _get_questions_extractor(
        self, num_questions: int = 3
    ) -> QuestionsAnsweredExtractor:
        """Get or create questions extractor"""

        # Define strict prompt to prevent "chatty" output and including answers
        question_gen_template = (
            "You are a question generator. Your task is to generate {num_questions} questions "
            "that can be answered using ONLY the provided context.\n"
            "Context:\n"
            "----------------\n"
            "{context_str}\n"
            "----------------\n"
            "Rules:\n"
            "1. Output ONLY the questions, separated by newlines.\n"
            "2. Do NOT provide answers to the questions.\n"
            "3. Do NOT number the questions.\n"
            "4. Do NOT include any introductory text, labels like 'Question:', or headers.\n"
            "5. The output should contain strictly the questions strings and nothing else.\n"
            "\n"
            "Questions:"
        )

        # Always recreate if num_questions changes
        if self._questions_extractor is None or num_questions != 3:
            self._questions_extractor = QuestionsAnsweredExtractor(
                questions=num_questions,
                llm=self.llm,
                prompt_template=question_gen_template,
            )
        return self._questions_extractor

    def _get_keyword_extractor(self, keywords: int = 5) -> KeywordExtractor:
        """Get or create keyword extractor"""

        # Define strict prompt for keywords
        keyword_gen_template = (
            "You are a keyword extractor. Your task is to extract {keywords} distinctive keywords "
            "from the provided context.\n"
            "Context:\n"
            "----------------\n"
            "{context_str}\n"
            "----------------\n"
            "Rules:\n"
            "1. Output ONLY the keywords.\n"
            "2. Separate keywords with commas.\n"
            "3. Do NOT number the keywords.\n"
            "4. Do NOT include any introductory or concluding text.\n"
            "\n"
            "Keywords:"
        )

        if self._keyword_extractor is None:
            self._keyword_extractor = KeywordExtractor(
                keywords=keywords,
                llm=self.llm,
                prompt_template=keyword_gen_template,
            )
        return self._keyword_extractor

    def chunk_document(
        self,
        parsed_doc: ParsedDocument,
        strategy: ChunkingStrategy = ChunkingStrategy.SENTENCE,
        chunk_size: int = 512,
        chunk_overlap: int = 50,
        include_metadata: bool = True,
        extract_metadata: bool = False,
        num_questions: int = 3,
        num_keywords: int = 5,
    ) -> ChunkedDocument:
        """
        Chunk a parsed document using the specified strategy.

        Args:
            parsed_doc: The parsed document to chunk
            strategy: Chunking strategy to use
            chunk_size: Target size for each chunk
            chunk_overlap: Overlap between chunks
            include_metadata: Whether to include document metadata in chunks
            extract_metadata: Whether to extract title/questions/keywords via LLM
            num_questions: Number of questions to generate per chunk (default: 3)
            num_keywords: Number of keywords to extract per chunk (default: 5)
        """
        logger.info(
            f"Chunking document {parsed_doc.document_id} with strategy: {strategy}"
        )

        # Get the text to chunk (prefer markdown for better structure preservation)
        # Note: Page limiting is already handled by the parser via Docling's page_range parameter
        text_to_chunk = parsed_doc.markdown_text or parsed_doc.raw_text

        if not text_to_chunk:
            logger.warning("No text content found in parsed document")
            text_to_chunk = ""

        # Create LlamaIndex document
        doc_metadata = {
            "document_id": parsed_doc.document_id,
            "filename": parsed_doc.metadata.filename,
            "file_type": parsed_doc.metadata.file_type,
        }

        if parsed_doc.metadata.title:
            doc_metadata["title"] = parsed_doc.metadata.title

        llama_doc = Document(
            text=text_to_chunk, metadata=doc_metadata if include_metadata else {}
        )

        # Get the appropriate splitter
        splitter = self._get_splitter(strategy, chunk_size, chunk_overlap)

        # Split into nodes
        nodes = splitter.get_nodes_from_documents([llama_doc])

        # Add parent info
        for node in nodes:
            if getattr(node, "parent_node", None):
                node.metadata["parent_id"] = node.parent_node.node_id
            if getattr(node, "child_nodes", None):
                node.metadata["child_ids"] = [c.node_id for c in node.child_nodes]

        # Extract metadata via LLM if enabled
        if extract_metadata:
            nodes = self._extract_metadata(nodes, num_questions, num_keywords)

        # Convert to our TextChunk format
        chunks = self._nodes_to_chunks(nodes, parsed_doc.document_id)

        # Also chunk tables separately if present
        if parsed_doc.tables:
            table_chunks = self._chunk_tables(
                parsed_doc.tables, parsed_doc.document_id, len(chunks)
            )
            chunks.extend(table_chunks)

        chunked_doc = ChunkedDocument(
            document_id=parsed_doc.document_id,
            metadata=parsed_doc.metadata,
            chunks=chunks,
            total_chunks=len(chunks),
            chunking_strategy=strategy,
            chunk_size=chunk_size,
            chunk_overlap=chunk_overlap,
        )

        logger.info(
            f"Created {len(chunks)} chunks from document {parsed_doc.document_id}"
        )

        return chunked_doc

    def _extract_metadata(
        self, nodes: List[TextNode], num_questions: int = 3, num_keywords: int = 5
    ) -> List[TextNode]:
        """
        Extract title, questions, and keywords metadata from nodes using LLM.

        Args:
            nodes: List of text nodes to enrich
            num_questions: Number of questions to generate per chunk
            num_keywords: Number of keywords to extract per chunk

        Returns:
            Nodes enriched with metadata
        """
        if not nodes:
            return nodes

        logger.info(f"Extracting metadata from {len(nodes)} nodes via LLM...")
        logger.info(
            f"Using LLM provider: {self.settings.llm_provider}, model: {self.settings.llm_model}"
        )

        try:
            # Extract title (uses first N nodes to infer document title)
            logger.info("Starting title extraction...")
            title_extractor = self._get_title_extractor()
            nodes = title_extractor.process_nodes(nodes)

            # Clean up the extracted title
            for node in nodes:
                if "document_title" in node.metadata:
                    node.metadata["document_title"] = self._clean_title(
                        node.metadata["document_title"]
                    )

            logger.info("Title extraction complete")

            # Extract keywords for each node
            logger.info(f"Starting keyword extraction ({num_keywords} per chunk)...")
            keyword_extractor = self._get_keyword_extractor(num_keywords)
            nodes = keyword_extractor.process_nodes(nodes)

            # Clean up extracted keywords
            for idx, node in enumerate(nodes):
                if "excerpt_keywords" in node.metadata:
                    raw_val = node.metadata["excerpt_keywords"]
                    logger.info(f"Raw keywords for node {idx}: {repr(raw_val)}")

                    # Ensure it's a clean string first
                    if isinstance(raw_val, str):
                        # Remove potentially hallucinated labels
                        cleaned_str = (
                            raw_val.replace("Keywords:", "")
                            .replace("keywords:", "")
                            .strip()
                        )
                        # Use simple comma split
                        keywords_list = [
                            k.strip() for k in cleaned_str.split(",") if k.strip()
                        ]
                        # Store as comma-separated string for now (LlamaIndex/Chroma usually prefer strings or simple lists)
                        # But consistency with other metadata suggests passing the list or a clean string.
                        # TextNode metadata is usually Dict[str, Any].
                        node.metadata["excerpt_keywords"] = ", ".join(keywords_list)
            logger.info("Keyword extraction complete")

            # Extract questions for each node
            logger.info(f"Starting questions extraction ({num_questions} per chunk)...")
            questions_extractor = self._get_questions_extractor(num_questions)
            nodes = questions_extractor.process_nodes(nodes)
            logger.info("Questions extraction complete")
            # log the first 5 nodes
            logger.info(f"First 5 nodes: {nodes[:5]}")

            # Clean up the extracted questions
            for idx, node in enumerate(nodes):
                # DEBUG: Log full metadata keys and raw values
                logger.info(f"Node {idx} metadata keys: {list(node.metadata.keys())}")

                # Check ALL possible question keys
                q_key = None
                for k in node.metadata.keys():
                    if "question" in k.lower():
                        q_key = k
                        break

                if q_key:
                    raw_val = node.metadata[q_key]
                    logger.info(
                        f"Using question key '{q_key}' with raw value:\n{repr(raw_val)}"
                    )

                    # Normalize key to 'questions'
                    if q_key != "questions":
                        node.metadata["questions"] = raw_val
                        del node.metadata[q_key]

                    if "questions" in node.metadata:
                        raw_val = node.metadata["questions"]
                    # Clean it if it's there
                    if isinstance(raw_val, str):
                        # The prompt template should make this less necessary, but keep for robustness
                        node.metadata["questions"] = [
                            q.strip()
                            for q in raw_val.split("\n")
                            if q.strip()
                            and not q.strip().startswith(("1.", "2.", "3.", "4.", "5."))
                        ]
                        # If the LLM returns numbered list, convert to list of strings
                        if not node.metadata[
                            "questions"
                        ] and raw_val.strip().startswith("1."):
                            node.metadata["questions"] = [
                                re.sub(r"^\d+\.\s*", "", q).strip()
                                for q in raw_val.split("\n")
                                if q.strip()
                            ]
                        # Ensure it's a list of strings
                        if not isinstance(node.metadata["questions"], list):
                            node.metadata["questions"] = [
                                str(node.metadata["questions"])
                            ]
                else:
                    logger.warning(f"No questions key for node {idx}")
                    # Attempt fallback check
                    for k in node.metadata.keys():
                        if "question" in k.lower():
                            logger.info(
                                f"Found potential question key: {k} -> {str(node.metadata[k])[:50]}..."
                            )

            logger.info(f"Questions extraction complete")

        except Exception as e:
            import traceback

            logger.error(f"Metadata extraction failed: {type(e).__name__}: {e}")
            logger.error(traceback.format_exc())
            # Return original nodes without extracted metadata

        return nodes

    def _clean_title(self, title: str) -> str:
        """Clean up verbose LLM title output"""
        if not title:
            return ""

        import re

        # Strategy 1: Look for content inside quotes if multiple words
        # e.g. "The Art of Software Testing" -> The Art of Software Testing
        quotes_match = re.search(r'"([^"]{5,})"', title)
        if quotes_match:
            return quotes_match.group(1).strip()

        # Strategy 2: Look for "Title: <title>" pattern
        title_match = re.search(r"(?i)title:\s*(.+)$", title, re.MULTILINE)
        if title_match:
            return title_match.group(1).strip()

        # Strategy 3: Fallback - remove known prefixes manually (less reliable but safe)
        cleaned = title.strip()
        prefixes = [
            "Based on the provided context, a comprehensive title for this document could be:",
            "Based on the provided context, the title is:",
            "Based on the provided context,",
            "The title of this document is:",
            "Here is a title for the document:",
            "Title:",
        ]

        for prefix in prefixes:
            if cleaned.lower().startswith(prefix.lower()):
                cleaned = cleaned[len(prefix) :].strip()

        # If we have multiple lines (chatty output), try to find the shortest non-empty line
        # that looks like a title (or just the first line if cleaned)
        if "\n" in cleaned:
            lines = [l.strip() for l in cleaned.split("\n") if l.strip()]
            for line in lines:
                # If a line is short-ish and doesn't start with "I would suggest", it might be the title
                if len(line) < 100 and not line.lower().startswith("i would suggest"):
                    return line
            # Fallback to first line
            return lines[0] if lines else cleaned

        return cleaned

    def chunk_text(
        self,
        text: str,
        strategy: ChunkingStrategy = ChunkingStrategy.SENTENCE,
        chunk_size: int = 512,
        chunk_overlap: int = 50,
        metadata: Optional[Dict[str, Any]] = None,
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

        llama_doc = Document(text=text, metadata=metadata or {})

        splitter = self._get_splitter(strategy, chunk_size, chunk_overlap)
        nodes = splitter.get_nodes_from_documents([llama_doc])

        return self._nodes_to_chunks(nodes, doc_id)

    def _get_splitter(
        self, strategy: ChunkingStrategy, chunk_size: int, chunk_overlap: int
    ):
        """Get the appropriate splitter for the strategy"""

        if strategy == ChunkingStrategy.HIERARCHICAL:
            # Make hierarchy relative to user's chunk_size
            small = max(100, chunk_size // 4)
            medium = max(200, chunk_size // 2)
            large = chunk_size

            return HierarchicalNodeParser(
                chunk_sizes=[small, medium, large],
                chunk_overlap=chunk_overlap,
                node_parser_ids=["small", "medium", "large"],
            )
            # return HierarchicalNodeParser(
            #     chunk_sizes=[128, 512, 1024],  # small → medium → large parent chunks
            #     chunk_overlap=chunk_overlap,
            #     node_parser_ids=["small", "medium", "large"],
            # )

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
            # Initialize with your embedding model
            embed_model = OpenAIEmbedding()  # or your preferred model
            return SemanticSplitterNodeParser(
                buffer_size=1,
                breakpoint_percentile_threshold=95,
                embed_model=embed_model,
            )

        else:
            # Default to sentence splitter
            return SentenceSplitter(
                chunk_size=chunk_size,
                chunk_overlap=chunk_overlap,
            )

    def _nodes_to_chunks(
        self, nodes: List[TextNode], document_id: str
    ) -> List[TextChunk]:
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
                token_count=self._estimate_tokens(node.get_content()),
            )
            chunks.append(chunk)

        return chunks

    def _chunk_tables(
        self, tables: List[Dict[str, Any]], document_id: str, start_index: int
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
                token_count=self._estimate_tokens(table_text),
            )
            chunks.append(chunk)

        return chunks

    def _estimate_tokens(self, text: str) -> int:
        """Estimate token count (rough approximation: ~4 chars per token)"""
        tokenizer = get_tokenizer()
        tokens = tokenizer(text)
        # return len(text) // 4
        return len(tokens)

    def get_available_strategies(self) -> List[str]:
        """Get list of available chunking strategies"""
        return [s.value for s in ChunkingStrategy]
