import { useState } from "react";
import { useRagChat } from "../api/hooks";

export function LLMTestsView() {
  const { messages, loading, error, sendMessage, clearHistory, lastResponse } =
    useRagChat();
  const [input, setInput] = useState("");

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
      <h1 className="text-2xl font-bold mb-4">LLM Tests</h1>

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
          <h3 className="text-lg font-semibold mb-2">Retrieved Context:</h3>
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
      {error && <p className="text-red-500 mb-4">Error: {error.message}</p>}

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
          disabled={loading || !input.trim()}
          className="p-2 bg-blue-500 text-white rounded disabled:opacity-50"
        >
          {loading ? "Sending..." : "Send"}
        </button>
        <button
          onClick={clearHistory}
          className="p-2 bg-gray-500 text-white rounded"
        >
          Clear
        </button>
      </div>
    </div>
  );
}
