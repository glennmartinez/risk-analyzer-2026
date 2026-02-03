import { useState } from "react";
import { useRagChat } from "../api/hooks";
import { useListCollections } from "../api/queries";

export function LLMTestsView() {
  const [selectedCollection, setSelectedCollection] = useState<string>("");
  const [maxChunks, setMaxChunks] = useState(5);
  const collectionsQuery = useListCollections();
  const { messages, loading, error, sendMessage, clearHistory, lastResponse } =
    useRagChat({
      collection_name: selectedCollection || undefined,
      max_chunks: maxChunks,
    });
  const [input, setInput] = useState("");

  const collections = collectionsQuery.data?.collections ?? [];

  const handleSend = async () => {
    if (!input.trim()) return;
    await sendMessage(input);
    setInput("");
  };

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !loading) {
      handleSend();
    }
  };

  return (
    <div className="p-4 max-w-4xl mx-auto">
      <h1 className="text-2xl font-bold mb-4">LLM Tests (RAG Chat)</h1>

      {/* Configuration Panel */}
      <div className="border rounded-lg p-4 mb-4 ">
        <h3 className="text-lg font-semibold mb-3">RAG Configuration</h3>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label
              htmlFor="collection-select"
              className="block text-sm font-medium text-gray-700 mb-1"
            >
              Collection:
            </label>
            <select
              id="collection-select"
              value={selectedCollection}
              onChange={(e) => setSelectedCollection(e.target.value)}
              className="w-full p-2 border rounded"
              disabled={collectionsQuery.isLoading}
            >
              <option value="">-- Select a collection --</option>
              {collections.map((collection) => (
                <option key={collection} value={collection}>
                  {collection}
                </option>
              ))}
            </select>
            {collectionsQuery.isLoading && (
              <span className="text-xs text-gray-500 mt-1">
                Loading collections...
              </span>
            )}
            {!selectedCollection && !collectionsQuery.isLoading && (
              <span className="text-xs text-yellow-600 mt-1">
                ⚠️ Collection required for RAG chat
              </span>
            )}
          </div>
          <div>
            <label
              htmlFor="max-chunks"
              className="block text-sm font-medium text-gray-700 mb-1"
            >
              Max Context Chunks:
            </label>
            <select
              id="max-chunks"
              value={maxChunks}
              onChange={(e) => setMaxChunks(Number(e.target.value))}
              className="w-full p-2 border rounded"
            >
              <option value={3}>3 chunks</option>
              <option value={5}>5 chunks</option>
              <option value={7}>7 chunks</option>
              <option value={10}>10 chunks</option>
            </select>
          </div>
        </div>
      </div>

      {/* Chat History */}
      <div className="border rounded-lg p-4 mb-4 h-96 overflow-y-auto bg-gray-50">
        {messages.length === 0 ? (
          <p className="text-gray-500">
            Start a conversation with the RAG model.
          </p>
        ) : (
          messages.map((msg, index) => (
            <div
              key={index}
              className={`mb-2 p-2 rounded ${
                msg.role === "user"
                  ? "bg-blue-100 text-right"
                  : "bg-green-100 text-left"
              }`}
            >
              <strong>{msg.role === "user" ? "You" : "Assistant"}:</strong>{" "}
              {msg.content}
            </div>
          ))
        )}
        {loading && <p className="text-gray-500">Assistant is thinking...</p>}
      </div>

      {/* Retrieved Context */}
      {lastResponse?.context && lastResponse.context.length > 0 && (
        <div className="border rounded-lg p-4 mb-4 bg-blue-50">
          <h3 className="text-lg font-semibold mb-2">
            Retrieved Context ({lastResponse.context.length} chunks
            {selectedCollection && ` from "${selectedCollection}"`}):
          </h3>
          {lastResponse.context.map((chunk, idx) => (
            <div key={idx} className="border p-2 mb-2 rounded bg-white">
              <p className="font-medium">
                Score: {chunk.score?.toFixed(3) || "N/A"}
              </p>
              <p className="mb-2">{chunk.text}</p>
              {chunk.metadata && (
                <details>
                  <summary className="cursor-pointer text-sm text-gray-600">
                    Metadata
                  </summary>
                  <pre className="text-xs mt-2 bg-gray-100 p-1 rounded">
                    {JSON.stringify(chunk.metadata, null, 2)}
                  </pre>
                </details>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Error Display */}
      {error && (
        <div className="border border-red-300 rounded-lg p-4 mb-4 bg-red-50">
          <p className="text-red-700 font-semibold">Error: {error.message}</p>
          <p className="text-sm text-red-600 mt-1">
            Tip: Make sure the selected collection exists and contains documents
          </p>
        </div>
      )}

      {/* Input and Controls */}
      <div className="flex gap-2">
        <input
          type="text"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={handleKeyPress}
          placeholder="Type your message..."
          className="flex-1 p-2 border rounded"
          disabled={loading}
        />
        <button
          onClick={handleSend}
          disabled={loading || !input.trim() || !selectedCollection}
          className="px-4 py-2 bg-blue-500 text-white rounded disabled:opacity-50 hover:bg-blue-600 transition-colors"
          title={!selectedCollection ? "Please select a collection first" : ""}
        >
          {loading ? "Sending..." : "Send"}
        </button>
        <button
          onClick={clearHistory}
          className="px-4 py-2 bg-gray-500 text-white rounded hover:bg-gray-600 transition-colors"
          disabled={loading}
        >
          Clear
        </button>
      </div>
    </div>
  );
}
