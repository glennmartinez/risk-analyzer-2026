import type { ChatRequest, ChatResponse, HealthResponse } from "./types";

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
    options?: RequestInit
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
}

export const apiClient = new ApiClient();
export default apiClient;
