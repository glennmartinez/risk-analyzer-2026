import { useState, useCallback } from "react";
import apiClient from "../client";
import type { RAGChatRequest, RAGChatResponse, ChatMessage } from "../types";

interface UseRagChatState {
  loading: boolean;
  error: Error | null;
  lastResponse?: RAGChatResponse;
}

interface UseRagChatConfig {
  collection_name?: string;
  max_chunks?: number;
}

export function useRagChat(config?: UseRagChatConfig) {
  const [state, setState] = useState<UseRagChatState>({
    loading: false,
    error: null,
  });
  const [messages, setMessages] = useState<ChatMessage[]>([]);

  const sendMessage = useCallback(
    async (message: string): Promise<RAGChatResponse | null> => {
      setState({ loading: true, error: null });

      // Add user message to history
      const userMessage: ChatMessage = { role: "user", content: message };
      setMessages((prev) => [...prev, userMessage]);

      try {
        const request: RAGChatRequest = {
          message,
          history: messages,
          max_chunks: config?.max_chunks || 5,
          collection_name: config?.collection_name,
        };

        const response = await apiClient.ragChat(request);

        // Add assistant response to history
        const assistantMessage: ChatMessage = {
          role: "assistant",
          content: response.message,
        };
        setMessages((prev) => [...prev, assistantMessage]);

        setState({ loading: false, error: null, lastResponse: response });
        return response;
      } catch (error) {
        const err = error instanceof Error ? error : new Error("Unknown error");
        setState({ loading: false, error: err });

        // Remove the user message if request failed
        setMessages((prev) => prev.slice(0, -1));
        return null;
      }
    },
    [messages, config?.collection_name, config?.max_chunks],
  );

  const clearHistory = useCallback(() => {
    setMessages([]);
  }, []);

  return {
    messages,
    loading: state.loading,
    error: state.error,
    lastResponse: state.lastResponse,
    sendMessage,
    clearHistory,
  };
}
