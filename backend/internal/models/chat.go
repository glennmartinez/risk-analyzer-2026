package models

// ChatMessage represents a single message in a conversation
type ChatMessage struct {
	Role    string `json:"role"`    // "user", "assistant", or "system"
	Content string `json:"content"` // The message content
}

// ChatRequest represents the incoming chat request from the frontend
type ChatRequest struct {
	Message string        `json:"message"`           // The current user message
	History []ChatMessage `json:"history,omitempty"` // Previous conversation history
}

// ChatResponse represents the response sent back to the frontend
type ChatResponse struct {
	Message string `json:"message"` // The assistant's response
	Status  string `json:"status"`  // "success" or "error"
}

// RAGChatRequest represents a chat request with automatic document retrieval
type RAGChatRequest struct {
	Message        string        `json:"message"`           // The current user message
	History        []ChatMessage `json:"history,omitempty"` // Previous conversation history
	UseRAG         bool          `json:"use_rag,omitempty"` // Whether to use RAG (default: true)
	MaxChunks      int           `json:"max_chunks,omitempty"`      // Maximum chunks to retrieve (default: 3)
	CollectionName string        `json:"collection_name,omitempty"` // Vector DB collection to search (default: "documents")
}

// RAGChatResponse represents a response with retrieved context information
type RAGChatResponse struct {
	Message string            `json:"message"`          // The assistant's response
	Status  string            `json:"status"`           // "success" or "error"
	Context []RAGContextChunk `json:"context,omitempty"` // Retrieved document chunks used for context
	Query   string            `json:"query,omitempty"`   // The search query used
}

// RAGContextChunk represents a document chunk used as context
type RAGContextChunk struct {
	Text     string                 `json:"text"`               // The chunk text content
	Score    float64                `json:"score,omitempty"`    // Similarity score
	Metadata map[string]interface{} `json:"metadata,omitempty"` // Chunk metadata (document_id, page, etc.)
}
