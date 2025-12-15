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
