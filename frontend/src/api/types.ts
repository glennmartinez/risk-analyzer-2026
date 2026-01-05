// Chat types - matches Go backend models/chat.go

export type MessageRole = "user" | "assistant" | "system";

export interface ChatMessage {
  role: MessageRole;
  content: string;
}

export interface ChatRequest {
  message: string;
  history?: ChatMessage[];
}

export interface ChatResponse {
  message: string;
  status: "success" | "error";
}

export interface RAGChatRequest {
  message: string;
  history?: ChatMessage[];
  use_rag?: boolean;
  max_chunks?: number;
  collection_name?: string;
}

export interface RAGChatResponse {
  message: string;
  status: string;
  context?: RAGContextChunk[];
  query?: string;
}

export interface RAGContextChunk {
  text: string;
  score?: number;
  metadata?: Record<string, any>;
}

// Health check response
export interface HealthResponse {
  message: string;
  status: "success" | "error";
}

// Search query types - matches Go backend models/search.go
export interface SearchQueryRequest {
  query: string;
  top_k: number;
  collection?: string;
}

export interface SearchQueryResponse {
  query: string;
  results: ChunkData[];
  total_results: number;
  search_time_seconds: number;
}

export interface ChunkData {
  chunk_id: string;
  text: string;
  score: number;
  metadata: ChunkMetadata;
}

export interface ChunkMetadata {
  token_count: number;
  file_name: string;
  chunk_index: number;
  file_type: string;
  document_id: string;
}
