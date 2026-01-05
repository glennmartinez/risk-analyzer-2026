import type {
  ChatRequest,
  ChatResponse,
  HealthResponse,
  RAGChatRequest,
  RAGChatResponse,
  SearchQueryResponse,
} from "./types";
import type {
  DocumentResponse,
  ListDocumentsResponse,
  ListVectorDocumentsResponse,
  GetDocumentChunksResponse,
  DeleteDocumentResponse,
  DeleteCollectionResponse,
} from "../models/Documents";

const API_BASE_URL = "http://localhost:8080";

// Re-export types for convenience
export type {
  ChatMessage,
  ChatRequest,
  ChatResponse,
  HealthResponse,
} from "./types";

class ApiClient {
  private baseUrl: string;

  constructor(baseUrl: string = API_BASE_URL) {
    this.baseUrl = baseUrl;
  }

  private async request<T>(
    endpoint: string,
    options?: RequestInit,
  ): Promise<T> {
    const url = `${this.baseUrl}${endpoint}`;

    const response = await fetch(url, {
      ...options,
      headers: {
        "Content-Type": "application/json",
        ...options?.headers,
      },
    });

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(`API Error: ${response.status} - ${errorText}`);
    }

    return response.json();
  }

  // Server health check
  async healthCheck(): Promise<HealthResponse> {
    return this.request<HealthResponse>("/health");
  }

  // LLM Studio health check
  async llmHealthCheck(): Promise<HealthResponse> {
    return this.request<HealthResponse>("/llm/health");
  }

  // Send a chat message
  async chat(request: ChatRequest): Promise<ChatResponse> {
    return this.request<ChatResponse>("/chat", {
      method: "POST",
      body: JSON.stringify(request),
    });
  }

  // Send a RAG chat message
  async ragChat(request: RAGChatRequest): Promise<RAGChatResponse> {
    return this.request<RAGChatResponse>("/chat/rag", {
      method: "POST",
      body: JSON.stringify(request),
    });
  }

  // Send a search query
  async searchQuery(request: URLSearchParams): Promise<SearchQueryResponse> {
    return this.request<SearchQueryResponse>(`/search/query?${request}`, {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
      },
    });
  }

  // Upload a document
  async uploadDocument(formData: FormData): Promise<DocumentResponse> {
    const url = `${this.baseUrl}/api/ms/documents/upload`;

    const response = await fetch(url, {
      method: "POST",
      body: formData, // No Content-Type header - browser sets it with boundary
    });

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(`Upload failed: ${response.status} - ${errorText}`);
    }

    return response.json();
  }

  // List documents
  async listDocuments(): Promise<ListDocumentsResponse> {
    return this.request<ListDocumentsResponse>("/api/ms/documents/list");
  }

  // List vector documents
  async listVectorDocuments(
    collectionName?: string,
  ): Promise<ListVectorDocumentsResponse> {
    const params = collectionName
      ? `?collection_name=${encodeURIComponent(collectionName)}`
      : "";
    return this.request<ListVectorDocumentsResponse>(
      `/api/ms/documents/vector${params}`,
    );
  }

  // Document service health check
  async documentServiceHealth(): Promise<HealthResponse> {
    return this.request<HealthResponse>("/api/ms/documents/health");
  }

  // Get chunks for a specific document
  async getDocumentChunks(
    documentId: string,
    collectionName?: string,
    limit?: number,
    offset?: number,
  ): Promise<GetDocumentChunksResponse> {
    const params = new URLSearchParams({ document_id: documentId });
    if (collectionName) params.append("collection_name", collectionName);
    if (limit) params.append("limit", limit.toString());
    if (offset) params.append("offset", offset.toString());
    return this.request<GetDocumentChunksResponse>(
      `/api/ms/documents/chunks?${params}`,
    );
  }

  // Delete a document from vector store and Redis
  async deleteDocument(
    documentId: string,
    collectionName?: string,
  ): Promise<DeleteDocumentResponse> {
    const params = collectionName
      ? `?collection_name=${encodeURIComponent(collectionName)}`
      : "";
    const url = `${this.baseUrl}/api/ms/documents/${documentId}${params}`;

    const response = await fetch(url, {
      method: "DELETE",
    });

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(`Delete failed: ${response.status} - ${errorText}`);
    }

    return response.json();
  }

  // Delete an entire collection from vector store and Redis
  async deleteCollection(
    collectionName: string,
  ): Promise<DeleteCollectionResponse> {
    const url = `${this.baseUrl}/api/ms/documents/collection/${encodeURIComponent(collectionName)}`;

    const response = await fetch(url, {
      method: "DELETE",
    });

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(
        `Delete collection failed: ${response.status} - ${errorText}`,
      );
    }

    return response.json();
  }

  // List all collections from vector store
  async listCollections(): Promise<{ collections: string[] }> {
    return this.request<{ collections: string[] }>("/search/collections");
  }
}
export const apiClient = new ApiClient();
export default apiClient;
