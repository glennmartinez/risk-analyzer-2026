/**
 * Documents View
 * Displays document list in sidebar, details in main area, upload via modal
 */

import { useState } from "react";
import { DocumentUploadForm } from "../components/Rag";
import { Modal } from "../components/Modal";
import {
  useListDocuments,
  useListVectorDocuments,
  useDocumentServiceHealth,
  useDocumentChunks,
} from "../api/queries";
import type { Document } from "../models/Documents";
import {
  Upload,
  FileText,
  Database,
  RefreshCw,
  ChevronDown,
  ChevronRight,
} from "lucide-react";

type Tab = "documents" | "vectors";

export function DocumentsView() {
  const [activeTab, setActiveTab] = useState<Tab>("documents");
  const [selectedDocument, setSelectedDocument] = useState<Document | null>(
    null,
  );
  const [selectedCollection, setSelectedCollection] = useState<string>("");
  const [isUploadModalOpen, setIsUploadModalOpen] = useState(false);
  const [expandedChunks, setExpandedChunks] = useState<Set<string>>(new Set());

  const healthQuery = useDocumentServiceHealth();
  const documentsQuery = useListDocuments(activeTab === "documents");
  const vectorsQuery = useListVectorDocuments(
    selectedCollection || undefined,
    activeTab === "vectors",
  );

  // Fetch chunks for the selected document
  const chunksQuery = useDocumentChunks(
    selectedDocument?.document_id ?? null,
    selectedDocument?.collection,
    100,
    0,
    !!selectedDocument,
  );

  const documents = documentsQuery.data?.documents ?? [];
  const vectorDocuments = vectorsQuery.data?.documents ?? [];
  const chunks = chunksQuery.data?.chunks ?? [];

  const handleDocumentSelect = (doc: Document) => {
    setSelectedDocument(doc);
    setExpandedChunks(new Set());
  };

  const toggleChunkExpanded = (chunkId: string) => {
    setExpandedChunks((prev) => {
      const next = new Set(prev);
      if (next.has(chunkId)) {
        next.delete(chunkId);
      } else {
        next.add(chunkId);
      }
      return next;
    });
  };

  const formatDate = (dateString: string) => {
    try {
      return new Date(dateString).toLocaleString();
    } catch {
      return dateString;
    }
  };

  const formatFileSize = (bytes: number) => {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  };

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="px-6 py-4 border-b border-gray-200 bg-white">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-gray-900">
              Document Management
            </h1>
            <div className="flex items-center gap-4 mt-1">
              <p className="text-gray-600">
                Manage documents for RAG processing
              </p>
              {/* Service Health */}
              <div className="flex items-center gap-1.5">
                {healthQuery.isLoading ? (
                  <span className="text-xs text-gray-400">Checking...</span>
                ) : healthQuery.isError ? (
                  <span className="flex items-center gap-1 text-xs text-red-600">
                    <span className="w-1.5 h-1.5 bg-red-500 rounded-full"></span>
                    Service Offline
                  </span>
                ) : (
                  <span className="flex items-center gap-1 text-xs text-green-600">
                    <span className="w-1.5 h-1.5 bg-green-500 rounded-full"></span>
                    Service Online
                  </span>
                )}
              </div>
            </div>
          </div>
          <button
            onClick={() => setIsUploadModalOpen(true)}
            className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
          >
            <Upload className="w-4 h-4" />
            Upload Document
          </button>
        </div>
      </div>

      {/* Main Content */}
      <div className="flex-1 flex overflow-hidden">
        {/* Sidebar - Document List */}
        <div className="w-80 border-r border-gray-200 bg-gray-50 flex flex-col">
          {/* Tabs */}
          <div className="flex border-b border-gray-200 bg-white">
            <button
              onClick={() => {
                setActiveTab("documents");
                setSelectedDocument(null);
              }}
              className={`flex-1 flex items-center justify-center gap-2 px-4 py-3 text-sm font-medium transition-colors ${
                activeTab === "documents"
                  ? "text-blue-600 border-b-2 border-blue-600 bg-blue-50/50"
                  : "text-gray-600 hover:text-gray-900 hover:bg-gray-50"
              }`}
            >
              <FileText className="w-4 h-4" />
              Registry
            </button>
            <button
              onClick={() => {
                setActiveTab("vectors");
                setSelectedDocument(null);
              }}
              className={`flex-1 flex items-center justify-center gap-2 px-4 py-3 text-sm font-medium transition-colors ${
                activeTab === "vectors"
                  ? "text-blue-600 border-b-2 border-blue-600 bg-blue-50/50"
                  : "text-gray-600 hover:text-gray-900 hover:bg-gray-50"
              }`}
            >
              <Database className="w-4 h-4" />
              Vectors
            </button>
          </div>

          {/* Refresh & Filter */}
          <div className="p-3 border-b border-gray-200 bg-white">
            {activeTab === "documents" ? (
              <button
                onClick={() => documentsQuery.refetch()}
                disabled={documentsQuery.isFetching}
                className="flex items-center gap-2 px-3 py-1.5 text-sm text-gray-600 hover:text-gray-900 hover:bg-gray-100 rounded transition-colors disabled:opacity-50 w-full"
              >
                <RefreshCw
                  className={`w-4 h-4 ${documentsQuery.isFetching ? "animate-spin" : ""}`}
                />
                {documentsQuery.isFetching ? "Refreshing..." : "Refresh"}
              </button>
            ) : (
              <div className="space-y-2">
                <input
                  type="text"
                  placeholder="Filter by collection..."
                  value={selectedCollection}
                  onChange={(e) => setSelectedCollection(e.target.value)}
                  className="w-full px-3 py-1.5 text-sm border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
                <button
                  onClick={() => vectorsQuery.refetch()}
                  disabled={vectorsQuery.isFetching}
                  className="flex items-center gap-2 px-3 py-1.5 text-sm text-gray-600 hover:text-gray-900 hover:bg-gray-100 rounded transition-colors disabled:opacity-50 w-full"
                >
                  <RefreshCw
                    className={`w-4 h-4 ${vectorsQuery.isFetching ? "animate-spin" : ""}`}
                  />
                  {vectorsQuery.isFetching ? "Refreshing..." : "Refresh"}
                </button>
              </div>
            )}
          </div>

          {/* Document List */}
          <div className="flex-1 overflow-y-auto">
            {activeTab === "documents" ? (
              documentsQuery.isLoading ? (
                <div className="p-4 text-center text-gray-500">
                  Loading documents...
                </div>
              ) : documentsQuery.isError ? (
                <div className="p-4 text-center text-red-500 text-sm">
                  Error: {documentsQuery.error?.message}
                </div>
              ) : documents.length === 0 ? (
                <div className="p-4 text-center text-gray-500 text-sm">
                  No documents found.
                  <br />
                  Upload one to get started!
                </div>
              ) : (
                <div className="divide-y divide-gray-200">
                  {documents.map((doc) => (
                    <button
                      key={doc.document_id}
                      onClick={() => handleDocumentSelect(doc)}
                      className={`w-full text-left px-4 py-3 hover:bg-white transition-colors ${
                        selectedDocument?.document_id === doc.document_id
                          ? "bg-white border-l-2 border-blue-600"
                          : ""
                      }`}
                    >
                      <div className="flex items-start justify-between">
                        <div className="flex-1 min-w-0">
                          <p className="font-medium text-gray-900 truncate text-sm">
                            {doc.filename}
                          </p>
                          <p className="text-xs text-gray-500 mt-0.5">
                            {doc.chunk_count} chunks • {doc.collection}
                          </p>
                        </div>
                      </div>
                    </button>
                  ))}
                </div>
              )
            ) : vectorsQuery.isLoading ? (
              <div className="p-4 text-center text-gray-500">
                Loading vector documents...
              </div>
            ) : vectorsQuery.isError ? (
              <div className="p-4 text-center text-red-500 text-sm">
                Error: {vectorsQuery.error?.message}
              </div>
            ) : vectorDocuments.length === 0 ? (
              <div className="p-4 text-center text-gray-500 text-sm">
                No documents in vector store.
              </div>
            ) : (
              <div className="divide-y divide-gray-200">
                {vectorDocuments.map((doc) => (
                  <div
                    key={doc.document_id}
                    className="px-4 py-3 hover:bg-white transition-colors"
                  >
                    <p className="font-medium text-gray-900 truncate text-sm">
                      {doc.filename}
                    </p>
                    <p className="text-xs text-gray-500 mt-0.5">
                      {doc.chunk_count} chunks
                      {doc.title && ` • ${doc.title}`}
                    </p>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Document Count */}
          <div className="px-4 py-2 border-t border-gray-200 bg-white text-xs text-gray-500">
            {activeTab === "documents"
              ? `${documents.length} document${documents.length !== 1 ? "s" : ""} registered`
              : `${vectorDocuments.length} document${vectorDocuments.length !== 1 ? "s" : ""} in vector store`}
          </div>
        </div>

        {/* Main Content - Document Details */}
        <div className="flex-1 overflow-y-auto bg-white">
          {selectedDocument ? (
            <div className="p-6">
              <div className="mb-6">
                <h2 className="text-xl font-semibold text-gray-900">
                  {selectedDocument.filename}
                </h2>
                <p className="text-sm text-gray-500 mt-1">
                  ID: {selectedDocument.document_id}
                </p>
              </div>

              <div className="grid grid-cols-2 gap-6">
                {/* Basic Info */}
                <div className="space-y-4">
                  <h3 className="text-sm font-semibold text-gray-700 uppercase tracking-wider">
                    Document Info
                  </h3>
                  <dl className="space-y-3">
                    <div>
                      <dt className="text-xs text-gray-500">Registered At</dt>
                      <dd className="text-sm text-gray-900">
                        {formatDate(selectedDocument.registered_at)}
                      </dd>
                    </div>
                    <div>
                      <dt className="text-xs text-gray-500">File Size</dt>
                      <dd className="text-sm text-gray-900">
                        {formatFileSize(selectedDocument.file_size)}
                      </dd>
                    </div>
                    <div>
                      <dt className="text-xs text-gray-500">Collection</dt>
                      <dd className="text-sm text-gray-900">
                        {selectedDocument.collection}
                      </dd>
                    </div>
                    <div>
                      <dt className="text-xs text-gray-500">Total Chunks</dt>
                      <dd className="text-sm text-gray-900">
                        {selectedDocument.chunk_count}
                      </dd>
                    </div>
                  </dl>
                </div>

                {/* Processing Settings */}
                <div className="space-y-4">
                  <h3 className="text-sm font-semibold text-gray-700 uppercase tracking-wider">
                    Processing Settings
                  </h3>
                  <dl className="space-y-3">
                    <div>
                      <dt className="text-xs text-gray-500">
                        Chunking Strategy
                      </dt>
                      <dd className="text-sm text-gray-900">
                        {selectedDocument.chunking_strategy}
                      </dd>
                    </div>
                    <div>
                      <dt className="text-xs text-gray-500">Chunk Size</dt>
                      <dd className="text-sm text-gray-900">
                        {selectedDocument.chunk_size} tokens
                      </dd>
                    </div>
                    <div>
                      <dt className="text-xs text-gray-500">Chunk Overlap</dt>
                      <dd className="text-sm text-gray-900">
                        {selectedDocument.chunk_overlap} tokens
                      </dd>
                    </div>
                    <div>
                      <dt className="text-xs text-gray-500">Max Pages</dt>
                      <dd className="text-sm text-gray-900">
                        {selectedDocument.max_pages}
                      </dd>
                    </div>
                  </dl>
                </div>

                {/* LLM Settings */}
                <div className="space-y-4">
                  <h3 className="text-sm font-semibold text-gray-700 uppercase tracking-wider">
                    LLM Settings
                  </h3>
                  <dl className="space-y-3">
                    <div>
                      <dt className="text-xs text-gray-500">
                        Metadata Extraction
                      </dt>
                      <dd className="text-sm text-gray-900">
                        {selectedDocument.extract_metadata
                          ? "Enabled"
                          : "Disabled"}
                      </dd>
                    </div>
                    <div>
                      <dt className="text-xs text-gray-500">
                        Questions per Chunk
                      </dt>
                      <dd className="text-sm text-gray-900">
                        {selectedDocument.num_questions}
                      </dd>
                    </div>
                    <div>
                      <dt className="text-xs text-gray-500">LLM Provider</dt>
                      <dd className="text-sm text-gray-900">
                        {selectedDocument.llm_provider || "N/A"}
                      </dd>
                    </div>
                    <div>
                      <dt className="text-xs text-gray-500">LLM Model</dt>
                      <dd className="text-sm text-gray-900">
                        {selectedDocument.llm_model || "N/A"}
                      </dd>
                    </div>
                  </dl>
                </div>
              </div>

              {/* Document Chunks Section */}
              <div className="mt-8 pt-6 border-t border-gray-200">
                <div className="flex items-center justify-between mb-4">
                  <h3 className="text-sm font-semibold text-gray-700 uppercase tracking-wider">
                    Document Chunks
                  </h3>
                  <button
                    onClick={() => chunksQuery.refetch()}
                    disabled={chunksQuery.isFetching}
                    className="flex items-center gap-1.5 px-2 py-1 text-xs text-gray-600 hover:text-gray-900 hover:bg-gray-100 rounded transition-colors disabled:opacity-50"
                  >
                    <RefreshCw
                      className={`w-3 h-3 ${chunksQuery.isFetching ? "animate-spin" : ""}`}
                    />
                    Refresh
                  </button>
                </div>

                {chunksQuery.isLoading ? (
                  <div className="text-center py-8 text-gray-500">
                    Loading chunks...
                  </div>
                ) : chunksQuery.isError ? (
                  <div className="text-center py-8 text-red-500 text-sm">
                    Error loading chunks: {chunksQuery.error?.message}
                  </div>
                ) : chunks.length === 0 ? (
                  <div className="text-center py-8 text-gray-500 text-sm">
                    No chunks found in vector store for this document.
                  </div>
                ) : (
                  <div className="space-y-2">
                    <p className="text-xs text-gray-500 mb-3">
                      Showing {chunks.length} of {selectedDocument.chunk_count}{" "}
                      chunks
                    </p>
                    {chunks.map((chunk, index) => (
                      <div
                        key={chunk.id}
                        className="border border-gray-200 rounded-lg overflow-hidden"
                      >
                        <button
                          onClick={() => toggleChunkExpanded(chunk.id)}
                          className="w-full flex items-center justify-between px-4 py-3 bg-gray-50 hover:bg-gray-100 transition-colors text-left"
                        >
                          <div className="flex items-center gap-3">
                            {expandedChunks.has(chunk.id) ? (
                              <ChevronDown className="w-4 h-4 text-gray-500" />
                            ) : (
                              <ChevronRight className="w-4 h-4 text-gray-500" />
                            )}
                            <span className="text-sm font-medium text-gray-700">
                              Chunk {index + 1}
                            </span>
                            <span className="text-xs text-gray-500">
                              {chunk.text.length} chars
                            </span>
                          </div>
                          <span className="text-xs text-gray-400 font-mono">
                            {chunk.id.substring(0, 8)}...
                          </span>
                        </button>

                        {expandedChunks.has(chunk.id) && (
                          <div className="px-4 py-3 border-t border-gray-200">
                            <div className="mb-3">
                              <h4 className="text-xs font-semibold text-gray-500 uppercase mb-1">
                                Content
                              </h4>
                              <p className="text-sm text-gray-700 whitespace-pre-wrap bg-gray-50 p-3 rounded border max-h-60 overflow-y-auto">
                                {chunk.text}
                              </p>
                            </div>

                            {Object.keys(chunk.metadata).length > 0 && (
                              <div>
                                <h4 className="text-xs font-semibold text-gray-500 uppercase mb-1">
                                  Metadata
                                </h4>
                                <div className="bg-gray-50 p-3 rounded border">
                                  <dl className="grid grid-cols-2 gap-2 text-xs">
                                    {Object.entries(chunk.metadata).map(
                                      ([key, value]) => (
                                        <div key={key}>
                                          <dt className="text-gray-500">
                                            {key}
                                          </dt>
                                          <dd className="text-gray-900 font-mono">
                                            {typeof value === "object"
                                              ? JSON.stringify(value)
                                              : String(value)}
                                          </dd>
                                        </div>
                                      ),
                                    )}
                                  </dl>
                                </div>
                              </div>
                            )}
                          </div>
                        )}
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
          ) : (
            <div className="h-full flex items-center justify-center text-gray-500">
              <div className="text-center">
                <FileText className="w-12 h-12 mx-auto mb-3 text-gray-300" />
                <p>Select a document to view details</p>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Upload Modal */}
      <Modal
        isOpen={isUploadModalOpen}
        onClose={() => setIsUploadModalOpen(false)}
        title="Upload Document"
        size="lg"
      >
        <DocumentUploadForm
          onSuccess={() => {
            setIsUploadModalOpen(false);
            documentsQuery.refetch();
          }}
        />
      </Modal>
    </div>
  );
}
