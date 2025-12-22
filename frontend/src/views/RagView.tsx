import { useState } from "react";
import { useSearchQuery } from "../api/hooks";
import type { SearchQueryResponse } from "../api/types";

export function RagView() {
  const { loading, error, sendSearchQuery } = useSearchQuery();
  const [searchQuery, setSearchQuery] = useState("");
  const [topK, setTopK] = useState(5);
  const [results, setResults] = useState<SearchQueryResponse | null>(null);

  const handleSearch = async () => {
    if (!searchQuery.trim()) return;
    const response = await sendSearchQuery({ query: searchQuery, top_k: topK });
    setResults(response);
  };

  return (
    <div className="p-4">
      <h1 className="text-2xl text-destructive mb-4">RAG View</h1>

      <div className="mb-4 flex gap-2">
        <input
          type="text"
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          placeholder="Enter your search query"
          className="flex-1 p-2 border rounded"
        />
        <select
          value={topK}
          onChange={(e) => setTopK(Number(e.target.value))}
          className="p-2 border rounded"
        >
          <option value={5}>5</option>
          <option value={10}>10</option>
          <option value={15}>15</option>
          <option value={20}>20</option>
        </select>
        <button
          onClick={handleSearch}
          disabled={loading}
          className="p-2 bg-blue-500 text-white rounded disabled:opacity-50"
        >
          {loading ? "Searching..." : "Search"}
        </button>
      </div>

      {error && <p className="text-red-500 mb-4">Error: {error.message}</p>}

      {results && (
        <div>
          <h2 className="text-xl mb-2">Results for: "{results.query}"</h2>
          <p className="mb-4">
            Total results: {results.total_results} | Search time:{" "}
            {results.search_time_seconds}s
          </p>
          <div className="space-y-4">
            {results.results.map((chunk) => (
              <div key={chunk.chunk_id} className="border p-4 rounded">
                <p className="font-semibold">Score: {chunk.score.toFixed(3)}</p>
                <p className="mb-2">{chunk.text}</p>
                <details>
                  <summary className="cursor-pointer text-sm text-gray-600">
                    Metadata
                  </summary>
                  <pre className="text-xs mt-2">
                    {JSON.stringify(chunk.metadata, null, 2)}
                  </pre>
                </details>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
