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

// Health check response
export interface HealthResponse {
  message: string;
  status: "success" | "error";
}

// Search query types - matches Go backend models/search.go
export interface SearchQueryRequest {
  query: string;
  top_k: number;
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
