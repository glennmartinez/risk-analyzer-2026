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
  useDeleteDocument,
  useDeleteCollection,
} from "../api/queries";
import type { Document, VectorDocument } from "../models/Documents";
import {
  Upload,
  FileText,
  Database,
  RefreshCw,
  ChevronDown,
  ChevronRight,
  Trash2,
  AlertTriangle,
} from "lucide-react";

type Tab = "documents" | "vectors";

interface DeleteConfirmation {
  type: "document" | "collection";
  id: string;
  name: string;
  collection?: string;
}

export function DocumentsView() {
  const [activeTab, setActiveTab] = useState<Tab>("documents");
  const [selectedDocument, setSelectedDocument] = useState<Document | null>(
    null,
  );
  const [selectedVectorDoc, setSelectedVectorDoc] =
    useState<VectorDocument | null>(null);
  const [selectedCollection, setSelectedCollection] = useState<string>("");
  const [isUploadModalOpen, setIsUploadModalOpen] = useState(false);
  const [expandedChunks, setExpandedChunks] = useState<Set<string>>(new Set());
  const [deleteConfirmation, setDeleteConfirmation] =
    useState<DeleteConfirmation | null>(null);

  const healthQuery = useDocumentServiceHealth();
  const documentsQuery = useListDocuments(activeTab === "documents");
  const vectorsQuery = useListVectorDocuments(
    selectedCollection || undefined,
    activeTab === "vectors",
  );

  // Fetch chunks for the selected document
  const chunksQuery = useDocumentChunks(
    selectedDocument?.document_id ?? selectedVectorDoc?.document_id ?? null,
    selectedDocument?.collection ?? selectedVectorDoc?.collection,
    100,
    0,
    !!selectedDocument || !!selectedVectorDoc,
  );

  const deleteDocumentMutation = useDeleteDocument({
    onSuccess: () => {
      setSelectedDocument(null);
      setSelectedVectorDoc(null);
      setDeleteConfirmation(null);
    },
  });

  const deleteCollectionMutation = useDeleteCollection({
    onSuccess: () => {
      setSelectedDocument(null);
      setSelectedVectorDoc(null);
      setDeleteConfirmation(null);
    },
  });

  const documents = documentsQuery.data?.documents ?? [];
  const vectorDocuments = vectorsQuery.data?.documents ?? [];
  const chunks = chunksQuery.data?.chunks ?? [];

  // Get unique collections from vector documents
  const collections = [...new Set(vectorDocuments.map((d) => d.collection))];

  const handleDocumentSelect = (doc: Document) => {
    setSelectedDocument(doc);
    setSelectedVectorDoc(null);
    setExpandedChunks(new Set());
  };

  const handleVectorDocSelect = (doc: VectorDocument) => {
    setSelectedVectorDoc(doc);
    setSelectedDocument(null);
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

  const handleDeleteDocument = (doc: Document | VectorDocument) => {
    setDeleteConfirmation({
      type: "document",
      id: doc.document_id,
      name: doc.filename,
      collection: doc.collection,
    });
  };

  const handleDeleteCollection = (collectionName: string) => {
    setDeleteConfirmation({
      type: "collection",
      id: collectionName,
      name: collectionName,
    });
  };

  const confirmDelete = () => {
    if (!deleteConfirmation) return;

    if (deleteConfirmation.type === "document") {
      deleteDocumentMutation.mutate({
        documentId: deleteConfirmation.id,
        collectionName: deleteConfirmation.collection,
      });
    } else {
      deleteCollectionMutation.mutate(deleteConfirmation.id);
    }
  };

  const isDeleting =
    deleteDocumentMutation.isPending || deleteCollectionMutation.isPending;

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
                setSelectedVectorDoc(null);
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
                setSelectedVectorDoc(null);
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
                  <button
                    key={doc.document_id}
                    onClick={() => handleVectorDocSelect(doc)}
                    className={`w-full text-left px-4 py-3 hover:bg-white transition-colors ${
                      selectedVectorDoc?.document_id === doc.document_id
                        ? "bg-white border-l-2 border-blue-600"
                        : ""
                    }`}
                  >
                    <p className="font-medium text-gray-900 truncate text-sm">
                      {doc.filename}
                    </p>
                    <p className="text-xs text-gray-500 mt-0.5">
                      {doc.chunk_count} chunks • {doc.collection}
                    </p>
                  </button>
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
              <div className="flex items-start justify-between mb-6">
                <div>
                  <h2 className="text-xl font-semibold text-gray-900">
                    {selectedDocument.filename}
                  </h2>
                  <p className="text-sm text-gray-500 mt-1">
                    ID: {selectedDocument.document_id}
                  </p>
                </div>
                <button
                  onClick={() => handleDeleteDocument(selectedDocument)}
                  className="flex items-center gap-2 px-3 py-2 text-red-600 hover:bg-red-50 rounded-lg transition-colors"
                >
                  <Trash2 className="w-4 h-4" />
                  Delete Document
                </button>
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

                {/* Delete Collection */}
                <div className="space-y-4">
                  <h3 className="text-sm font-semibold text-gray-700 uppercase tracking-wider">
                    Danger Zone
                  </h3>
                  <div className="p-4 border border-red-200 rounded-lg bg-red-50">
                    <p className="text-sm text-red-800 mb-3">
                      Delete the entire collection "
                      {selectedDocument.collection}" and all its documents.
                    </p>
                    <button
                      onClick={() =>
                        handleDeleteCollection(selectedDocument.collection)
                      }
                      className="flex items-center gap-2 px-3 py-2 bg-red-600 text-white rounded hover:bg-red-700 transition-colors text-sm"
                    >
                      <Trash2 className="w-4 h-4" />
                      Delete Collection
                    </button>
                  </div>
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
          ) : selectedVectorDoc ? (
            <div className="p-6">
              <div className="flex items-start justify-between mb-6">
                <div>
                  <h2 className="text-xl font-semibold text-gray-900">
                    {selectedVectorDoc.filename}
                  </h2>
                  <p className="text-sm text-gray-500 mt-1">
                    ID: {selectedVectorDoc.document_id}
                  </p>
                </div>
                <button
                  onClick={() => handleDeleteDocument(selectedVectorDoc)}
                  className="flex items-center gap-2 px-3 py-2 text-red-600 hover:bg-red-50 rounded-lg transition-colors"
                >
                  <Trash2 className="w-4 h-4" />
                  Delete Document
                </button>
              </div>

              <div className="grid grid-cols-2 gap-6">
                {/* Basic Info */}
                <div className="space-y-4">
                  <h3 className="text-sm font-semibold text-gray-700 uppercase tracking-wider">
                    Vector Store Info
                  </h3>
                  <dl className="space-y-3">
                    <div>
                      <dt className="text-xs text-gray-500">Collection</dt>
                      <dd className="text-sm text-gray-900">
                        {selectedVectorDoc.collection}
                      </dd>
                    </div>
                    <div>
                      <dt className="text-xs text-gray-500">Total Chunks</dt>
                      <dd className="text-sm text-gray-900">
                        {selectedVectorDoc.chunk_count}
                      </dd>
                    </div>
                    {selectedVectorDoc.title && (
                      <div>
                        <dt className="text-xs text-gray-500">Title</dt>
                        <dd className="text-sm text-gray-900">
                          {selectedVectorDoc.title}
                        </dd>
                      </div>
                    )}
                  </dl>
                </div>

                {/* Delete Collection */}
                <div className="space-y-4">
                  <h3 className="text-sm font-semibold text-gray-700 uppercase tracking-wider">
                    Danger Zone
                  </h3>
                  <div className="p-4 border border-red-200 rounded-lg bg-red-50">
                    <p className="text-sm text-red-800 mb-3">
                      Delete the entire collection "
                      {selectedVectorDoc.collection}" and all its documents.
                    </p>
                    <button
                      onClick={() =>
                        handleDeleteCollection(selectedVectorDoc.collection)
                      }
                      className="flex items-center gap-2 px-3 py-2 bg-red-600 text-white rounded hover:bg-red-700 transition-colors text-sm"
                    >
                      <Trash2 className="w-4 h-4" />
                      Delete Collection
                    </button>
                  </div>
                </div>
              </div>

              {/* Collections Overview */}
              {collections.length > 0 && (
                <div className="mt-8 pt-6 border-t border-gray-200">
                  <h3 className="text-sm font-semibold text-gray-700 uppercase tracking-wider mb-4">
                    All Collections
                  </h3>
                  <div className="grid grid-cols-3 gap-3">
                    {collections.map((coll) => {
                      const docsInColl = vectorDocuments.filter(
                        (d) => d.collection === coll,
                      );
                      const totalChunks = docsInColl.reduce(
                        (sum, d) => sum + d.chunk_count,
                        0,
                      );
                      return (
                        <div
                          key={coll}
                          className="p-3 border border-gray-200 rounded-lg"
                        >
                          <div className="flex items-center justify-between">
                            <span className="font-medium text-sm text-gray-900">
                              {coll}
                            </span>
                            <button
                              onClick={() => handleDeleteCollection(coll)}
                              className="p-1 text-red-500 hover:bg-red-50 rounded transition-colors"
                              title="Delete collection"
                            >
                              <Trash2 className="w-3.5 h-3.5" />
                            </button>
                          </div>
                          <p className="text-xs text-gray-500 mt-1">
                            {docsInColl.length} docs • {totalChunks} chunks
                          </p>
                        </div>
                      );
                    })}
                  </div>
                </div>
              )}

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
                      Showing {chunks.length} of {selectedVectorDoc.chunk_count}{" "}
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
            vectorsQuery.refetch();
          }}
        />
      </Modal>

      {/* Delete Confirmation Modal */}
      <Modal
        isOpen={!!deleteConfirmation}
        onClose={() => setDeleteConfirmation(null)}
        title={
          deleteConfirmation?.type === "collection"
            ? "Delete Collection"
            : "Delete Document"
        }
        size="sm"
      >
        <div className="p-6">
          <div className="flex items-center gap-3 mb-4">
            <div className="p-2 bg-red-100 rounded-full">
              <AlertTriangle className="w-6 h-6 text-red-600" />
            </div>
            <div>
              <h3 className="font-semibold text-gray-900">
                {deleteConfirmation?.type === "collection"
                  ? "Delete Collection"
                  : "Delete Document"}
              </h3>
              <p className="text-sm text-gray-500">
                This action cannot be undone.
              </p>
            </div>
          </div>

          <p className="text-sm text-gray-700 mb-6">
            {deleteConfirmation?.type === "collection" ? (
              <>
                Are you sure you want to delete the collection{" "}
                <strong>"{deleteConfirmation.name}"</strong>? This will remove
                all documents and chunks in this collection from both the vector
                store and Redis registry.
              </>
            ) : (
              <>
                Are you sure you want to delete{" "}
                <strong>"{deleteConfirmation?.name}"</strong>? This will remove
                all chunks from the vector store and the document from the Redis
                registry.
              </>
            )}
          </p>

          <div className="flex gap-3 justify-end">
            <button
              onClick={() => setDeleteConfirmation(null)}
              disabled={isDeleting}
              className="px-4 py-2 text-gray-700 bg-gray-100 hover:bg-gray-200 rounded-lg transition-colors disabled:opacity-50"
            >
              Cancel
            </button>
            <button
              onClick={confirmDelete}
              disabled={isDeleting}
              className="flex items-center gap-2 px-4 py-2 bg-red-600 text-white hover:bg-red-700 rounded-lg transition-colors disabled:opacity-50"
            >
              {isDeleting ? (
                <>
                  <RefreshCw className="w-4 h-4 animate-spin" />
                  Deleting...
                </>
              ) : (
                <>
                  <Trash2 className="w-4 h-4" />
                  Delete
                </>
              )}
            </button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
