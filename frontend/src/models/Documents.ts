//update this go type to typescript
export interface DocumentRequest {
  chunking_strategy: string;
  chunk_size: number;
  chunk_overlap: number;
  store_in_vector_db: boolean;
  extract_tables: boolean;
  extract_figures: boolean;
  extract_metadata: boolean;
  num_questions: number;
  max_pages: number;
  collection_name: string;
}

// Response from upload endpoint
export interface DocumentResponse {
  document_id: string;
  filename: string;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  metadata: Record<string, any>;
  processing_time_ms: number;
  status: string;
  total_chunks: number;
  vector_db_stored: boolean;
}

// Document from Redis registry (via /documents/list)
export interface Document {
  document_id: string;
  registered_at: string;
  filename: string;
  chunk_count: number;
  collection: string;
  file_size: number;
  chunking_strategy: string;
  chunk_size: number;
  chunk_overlap: number;
  extract_metadata: boolean;
  num_questions: number;
  max_pages: number;
  llm_provider: string;
  llm_model: string;
}

// Response from /api/ms/documents/list
export interface ListDocumentsResponse {
  documents: Document[];
  total: number;
}

// Vector document (from /api/ms/documents/vector)
export interface VectorDocument {
  document_id: string;
  filename: string;
  title?: string;
  chunk_count: number;
  collection: string;
}

// Response from /api/ms/documents/vector
export interface ListVectorDocumentsResponse {
  documents: VectorDocument[];
  total: number;
  collection: string;
}

// Single chunk from vector store
export interface DocumentChunk {
  id: string;
  text: string;
  metadata: Record<string, unknown>;
}

// Response from /api/ms/documents/chunks
export interface GetDocumentChunksResponse {
  chunks: DocumentChunk[];
  count: number;
  limit: number;
  offset: number;
}

// Response from DELETE /api/ms/documents/{document_id}
export interface DeleteDocumentResponse {
  success: boolean;
  document_id: string;
  deleted_chunks: number;
  deleted_from_registry: boolean;
}

// Response from DELETE /api/ms/documents/collection/{collection_name}
export interface DeleteCollectionResponse {
  success: boolean;
  collection_name: string;
  documents_removed_from_registry: number;
  total_documents: number;
}
