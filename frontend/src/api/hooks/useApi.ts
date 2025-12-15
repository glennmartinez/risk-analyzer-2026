import { useState, useCallback } from "react";
import apiClient from "../client";
import type { ChatRequest, ChatResponse, ChatMessage } from "../types";

interface UseChatState {
  loading: boolean;
  error: Error | null;
}

export function useChat() {
  const [state, setState] = useState<UseChatState>({
    loading: false,
    error: null,
  });
  const [messages, setMessages] = useState<ChatMessage[]>([]);

  const sendMessage = useCallback(
    async (message: string): Promise<ChatResponse | null> => {
      setState({ loading: true, error: null });

      // Add user message to history
      const userMessage: ChatMessage = { role: "user", content: message };
      setMessages((prev) => [...prev, userMessage]);

      try {
        const request: ChatRequest = {
          message,
          history: messages,
        };

        const response = await apiClient.chat(request);

        // Add assistant response to history
        const assistantMessage: ChatMessage = {
          role: "assistant",
          content: response.message,
        };
        setMessages((prev) => [...prev, assistantMessage]);

        setState({ loading: false, error: null });
        return response;
      } catch (error) {
        const err = error instanceof Error ? error : new Error("Unknown error");
        setState({ loading: false, error: err });

        // Remove the user message if request failed
        setMessages((prev) => prev.slice(0, -1));
        return null;
      }
    },
    [messages]
  );

  const clearHistory = useCallback(() => {
    setMessages([]);
  }, []);

  return {
    messages,
    loading: state.loading,
    error: state.error,
    sendMessage,
    clearHistory,
  };
}

export function useHealthCheck() {
  const [loading, setLoading] = useState(false);
  const [healthy, setHealthy] = useState<boolean | null>(null);
  const [error, setError] = useState<Error | null>(null);

  const check = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      await apiClient.healthCheck();
      setHealthy(true);
    } catch (err) {
      setHealthy(false);
      setError(err instanceof Error ? err : new Error("Health check failed"));
    } finally {
      setLoading(false);
    }
  }, []);

  return { healthy, loading, error, check };
}
