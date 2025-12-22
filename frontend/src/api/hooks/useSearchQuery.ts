import type { SearchQueryRequest, SearchQueryResponse } from "../types";
import { useCallback, useState } from "react";
import apiClient from "../client";

interface SearchQueryState {
  loading: boolean;
  error: Error | null;
}

export function useSearchQuery() {
  const [state, setState] = useState<SearchQueryState>({
    loading: false,
    error: null,
  });

  const [query, setQuery] = useState<SearchQueryRequest | null>(null);

  const sendSearchQuery = useCallback(
    async (
      request: SearchQueryRequest,
    ): Promise<SearchQueryResponse | null> => {
      setState({ loading: true, error: null });

      const queryParams = new URLSearchParams({
        q: request.query,
        top_k: request.top_k.toString(),
      });

      try {
        const response = await apiClient.searchQuery(queryParams);
        const searchQueryResponse = {
          query: response.query,
          results: response.results,
          total_results: response.total_results,
          search_time_seconds: response.search_time_seconds,
        };
        setState({ loading: false, error: null });
        setQuery(request);
        return searchQueryResponse;
      } catch (error) {
        const err = error instanceof Error ? error : new Error("Unknown error");
        setState({ loading: false, error: err });
        return null;
      }
    },
    [],
  );

  return {
    loading: state.loading,
    error: state.error,
    sendSearchQuery,
    query,
  };
}
