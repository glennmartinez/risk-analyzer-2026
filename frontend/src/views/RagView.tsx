import { useState } from "react";
import { useSearchQuery } from "../api/hooks";
import { useListCollections } from "../api/queries";
import type { SearchQueryResponse } from "../api/types";

export function RagView() {
  const { loading, error, sendSearchQuery } = useSearchQuery();
  const collectionsQuery = useListCollections();
  const [searchQuery, setSearchQuery] = useState("");
  const [selectedCollection, setSelectedCollection] = useState<string>("");
  const [topK, setTopK] = useState(5);
  const [results, setResults] = useState<SearchQueryResponse | null>(null);

  const collections = collectionsQuery.data?.collections ?? [];

  const handleSearch = async () => {
    if (!searchQuery.trim()) return;
    const response = await sendSearchQuery({
      query: searchQuery,
      top_k: topK,
      collection: selectedCollection || undefined,
    });
    setResults(response);
  };

  return (
    <div className="p-4">
      <h1 className="text-2xl text-destructive mb-4">RAG View</h1>

      <div className="mb-4 space-y-2">
        <div className="flex gap-2">
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && handleSearch()}
            placeholder="Enter your search query"
            className="flex-1 p-2 border rounded"
          />
          <select
            value={topK}
            onChange={(e) => setTopK(Number(e.target.value))}
            className="p-2 border rounded"
          >
            <option value={5}>Top 5</option>
            <option value={10}>Top 10</option>
            <option value={15}>Top 15</option>
            <option value={20}>Top 20</option>
          </select>
          <button
            onClick={handleSearch}
            disabled={loading}
            className="px-4 py-2 bg-blue-500 text-white rounded disabled:opacity-50 hover:bg-blue-600 transition-colors"
          >
            {loading ? "Searching..." : "Search"}
          </button>
        </div>

        <div className="flex gap-2 items-center">
          <label
            htmlFor="collection-select"
            className="text-sm font-medium text-gray-700"
          >
            Collection:
          </label>
          <select
            id="collection-select"
            value={selectedCollection}
            onChange={(e) => setSelectedCollection(e.target.value)}
            className="p-2 border rounded flex-1"
            disabled={collectionsQuery.isLoading}
          >
            <option value="">All Collections</option>
            {collections.map((collection) => (
              <option key={collection} value={collection}>
                {collection}
              </option>
            ))}
          </select>
          {collectionsQuery.isLoading && (
            <span className="text-sm text-gray-500">
              Loading collections...
            </span>
          )}
          {selectedCollection && (
            <button
              onClick={() => setSelectedCollection("")}
              className="text-sm text-blue-600 hover:text-blue-800"
            >
              Clear
            </button>
          )}
        </div>
      </div>

      {error && <p className="text-red-500 mb-4">Error: {error.message}</p>}

      {results && (
        <div>
          <div className="mb-4">
            <h2 className="text-xl font-semibold mb-2">
              Results for: "{results.query}"
            </h2>
            <div className="flex gap-4 text-sm text-gray-600">
              <span>
                Total results: <strong>{results.total_results}</strong>
              </span>
              <span>
                Search time: <strong>{results.search_time_seconds}s</strong>
              </span>
              {selectedCollection && (
                <span>
                  Collection: <strong>{selectedCollection}</strong>
                </span>
              )}
            </div>
          </div>
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
