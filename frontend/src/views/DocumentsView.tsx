/**
 * Documents View
 * Displays collection summary and document IDs in sidebar
 */

import { useState } from "react";
import { DocumentUploadForm } from "../components/Rag";
import { Modal } from "../components/Modal";
import { useDocumentServiceHealth } from "../api/queries";
import {
  Upload,
  FileText,
  RefreshCw,
  Trash2,
  AlertTriangle,
} from "lucide-react";

import {
  useCollectionList,
  useCollectionInfo,
  useDocumentChunks,
} from "../api/queries/collections";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../components/ui/select";

interface DeleteConfirmation {
  type: "collection";
  id: string;
  name: string;
}

export function DocumentsView() {
  const [selectedCollection, setSelectedCollection] = useState<string>("");
  const [selectedDocumentId, setSelectedDocumentId] = useState<string>("");
  const [isUploadModalOpen, setIsUploadModalOpen] = useState(false);
  const [deleteConfirmation, setDeleteConfirmation] =
    useState<DeleteConfirmation | null>(null);

  const healthQuery = useDocumentServiceHealth();
  const {
    data: collections,
    isLoading: collectionsLoading,
    error: collectionsError,
    refetch: refetchCollections,
  } = useCollectionList();
  const {
    data: collectionInfo,
    isLoading: collectionLoading,
    error: collectionError,
    refetch: refetchCollection,
  } = useCollectionInfo(selectedCollection);

  const {
    data: documentChunksResponse,
    isLoading: chunksLoading,
    error: chunksError,
  } = useDocumentChunks(selectedDocumentId);

  const handleDeleteCollection = (collectionName: string) => {
    setDeleteConfirmation({
      type: "collection",
      id: collectionName,
      name: collectionName,
    });
  };

  const confirmDelete = () => {
    if (!deleteConfirmation) return;
    // Only collection deletion now
    // deleteCollectionMutation.mutate(deleteConfirmation.id);
  };

  const isDeleting = false; // Placeholder

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="px-6 py-4 border-b border-gray-200 bg-white">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-gray-900">
              Document Management
            </h1>
            {/* Collection Select */}
            <div className="mt-2 flex items-center gap-2">
              <label className="text-sm font-medium text-gray-700">
                Current Collection
              </label>
              <Select
                value={selectedCollection}
                onValueChange={setSelectedCollection}
              >
                <SelectTrigger className="w-[180px]">
                  <SelectValue placeholder="Select a collection" />
                </SelectTrigger>
                <SelectContent>
                  {collections?.collections.map((collection) => (
                    <SelectItem key={collection} value={collection}>
                      {collection}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
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
        {/* Sidebar - Collection Summary & Document IDs */}
        <div className="w-96 border-r border-gray-200 bg-gray-50 flex flex-col">
          {/* Collection Summary */}
          {collectionInfo && (
            <div className="p-4 border-b border-gray-200 bg-white">
              <div className="flex items-start justify-between mb-4">
                <div>
                  <h3 className="text-lg font-semibold text-gray-900">
                    Collection: {collectionInfo.name}
                  </h3>
                  <p className="text-sm text-gray-500">Summary of documents</p>
                </div>
                <button
                  onClick={() => handleDeleteCollection(collectionInfo.name)}
                  className="flex items-center gap-1 px-2 py-1 text-red-600 hover:bg-red-50 rounded text-sm"
                >
                  <Trash2 className="w-3 h-3" />
                  Delete
                </button>
              </div>

              <div className="grid grid-cols-1 gap-4">
                <div>
                  <h4 className="text-xs font-semibold text-gray-700 uppercase tracking-wider mb-2">
                    Stats
                  </h4>
                  <dl className="space-y-1">
                    <div className="flex justify-between">
                      <dt className="text-xs text-gray-500">Documents</dt>
                      <dd className="text-xs text-gray-900">
                        {collectionInfo.document_count}
                      </dd>
                    </div>
                    <div className="flex justify-between">
                      <dt className="text-xs text-gray-500">Chunks</dt>
                      <dd className="text-xs text-gray-900">
                        {collectionInfo.chunk_count}
                      </dd>
                    </div>
                  </dl>
                </div>

                {Object.keys(collectionInfo.metadata).length > 0 && (
                  <div>
                    <h4 className="text-xs font-semibold text-gray-700 uppercase tracking-wider mb-2">
                      Metadata
                    </h4>
                    <dl className="space-y-1">
                      {Object.entries(collectionInfo.metadata).map(
                        ([key, value]) => (
                          <div key={key} className="flex justify-between">
                            <dt className="text-xs text-gray-500">{key}</dt>
                            <dd className="text-xs text-gray-900 font-mono">
                              {typeof value === "object"
                                ? JSON.stringify(value)
                                : String(value)}
                            </dd>
                          </div>
                        ),
                      )}
                    </dl>
                  </div>
                )}
              </div>
            </div>
          )}

          {/* Document IDs Header */}
          <div className="p-3 border-b border-gray-200 bg-white">
            <h3 className="text-sm font-semibold text-gray-700 uppercase tracking-wider">
              Document IDs
            </h3>
          </div>

          <div className="flex-1 overflow-y-auto">
            {collectionLoading ? (
              <div className="p-4 text-center text-gray-500">
                Loading documents...
              </div>
            ) : collectionError ? (
              <div className="p-4 text-center text-red-500 text-sm">
                Error: {collectionError.message}
              </div>
            ) : !collectionInfo ? (
              <div className="p-4 text-center text-gray-500 text-sm">
                Select a collection to view documents.
              </div>
            ) : (collectionInfo.document_ids?.length || 0) === 0 ? (
              <div className="p-4 text-center text-gray-500 text-sm">
                No documents in this collection.
              </div>
            ) : (
              <div className="divide-y divide-gray-200">
                {collectionInfo.document_ids?.map((docId) => (
                  <button
                    key={docId}
                    onClick={() => setSelectedDocumentId(docId)}
                    className={`w-full text-left px-4 py-3 hover:bg-white transition-colors font-mono text-sm ${
                      selectedDocumentId === docId
                        ? "bg-white border-l-2 border-blue-600"
                        : ""
                    }`}
                  >
                    {docId}
                  </button>
                ))}
              </div>
            )}
          </div>

          <div className="px-4 py-2 border-t border-gray-200 bg-white text-xs text-gray-500">
            {collectionInfo
              ? `${collectionInfo.document_ids?.length || 0} document${(collectionInfo.document_ids?.length || 0) !== 1 ? "s" : ""}`
              : "No collection selected"}
          </div>
        </div>

        {/* Main Content - Document Chunks */}
        <div className="flex-1 overflow-y-auto bg-white">
          {chunksLoading ? (
            <div className="h-full flex items-center justify-center text-gray-500">
              <div className="text-center">
                <RefreshCw className="w-12 h-12 mx-auto mb-3 text-gray-300 animate-spin" />
                <p>Loading chunks...</p>
              </div>
            </div>
          ) : chunksError ? (
            <div className="h-full flex items-center justify-center text-red-500">
              <div className="text-center">
                <AlertTriangle className="w-12 h-12 mx-auto mb-3 text-red-300" />
                <p>Error loading chunks: {chunksError.message}</p>
              </div>
            </div>
          ) : !selectedDocumentId ? (
            <div className="h-full flex items-center justify-center text-gray-500">
              <div className="text-center">
                <FileText className="w-12 h-12 mx-auto mb-3 text-gray-300" />
                <p>Select a document ID to view chunks</p>
              </div>
            </div>
          ) : documentChunksResponse?.chunks &&
            documentChunksResponse.chunks.length > 0 ? (
            <div className="p-6">
              <div className="mb-6">
                <h2 className="text-xl font-semibold text-gray-900">
                  Document Chunks: {selectedDocumentId}
                </h2>
                <p className="text-sm text-gray-500 mt-1">
                  {documentChunksResponse.chunks.length} chunk
                  {documentChunksResponse.chunks.length !== 1 ? "s" : ""}
                </p>
              </div>

              <div className="space-y-4">
                {documentChunksResponse.chunks.map((chunk, index) => (
                  <div
                    key={chunk.id}
                    className="border border-gray-200 rounded-lg overflow-hidden"
                  >
                    <div className="px-4 py-3 bg-gray-50 border-b border-gray-200">
                      <div className="flex items-center justify-between">
                        <span className="text-sm font-medium text-gray-700">
                          Chunk {index + 1}
                        </span>
                        <span className="text-xs text-gray-400 font-mono">
                          {chunk.id.substring(0, 8)}...
                        </span>
                      </div>
                    </div>

                    <div className="p-4">
                      <div className="mb-4">
                        <h4 className="text-xs font-semibold text-gray-500 uppercase mb-2">
                          Content
                        </h4>
                        <p className="text-sm text-gray-700 whitespace-pre-wrap">
                          {chunk.text}
                        </p>
                      </div>

                      <div className="grid grid-cols-2 gap-4">
                        <div>
                          <h4 className="text-xs font-semibold text-gray-500 uppercase mb-2">
                            Metadata
                          </h4>
                          <dl className="space-y-1 text-xs">
                            <div className="flex justify-between">
                              <dt className="text-gray-500">Chunk Index</dt>
                              <dd className="text-gray-900">
                                {chunk.chunk_index}
                              </dd>
                            </div>
                            <div className="flex justify-between">
                              <dt className="text-gray-500">Token Count</dt>
                              <dd className="text-gray-900">
                                {chunk.token_count}
                              </dd>
                            </div>
                            <div className="flex justify-between">
                              <dt className="text-gray-500">Filename</dt>
                              <dd className="text-gray-900">
                                {chunk.metadata?.filename}
                              </dd>
                            </div>
                            <div className="flex justify-between">
                              <dt className="text-gray-500">Title</dt>
                              <dd className="text-gray-900">
                                {chunk.metadata?.title}
                              </dd>
                            </div>
                          </dl>
                        </div>

                        <div>
                          <h4 className="text-xs font-semibold text-gray-500 uppercase mb-2">
                            Keywords & Questions
                          </h4>
                          <div className="space-y-2">
                            {chunk.metadata?.keywords && (
                              <div>
                                <dt className="text-xs text-gray-500">
                                  Keywords
                                </dt>
                                <dd className="text-xs text-gray-900">
                                  {Array.isArray(chunk.metadata.keywords)
                                    ? chunk.metadata.keywords.join(", ")
                                    : chunk.metadata.keywords}
                                </dd>
                              </div>
                            )}
                            {chunk.metadata?.questions && (
                              <div>
                                <dt className="text-xs text-gray-500">
                                  Questions
                                </dt>
                                <dd className="text-xs text-gray-900">
                                  <ul className="list-disc list-inside">
                                    {Array.isArray(chunk.metadata.questions)
                                      ? chunk.metadata.questions.map((q, i) => (
                                          <li key={i}>{q}</li>
                                        ))
                                      : chunk.metadata.questions}
                                  </ul>
                                </dd>
                              </div>
                            )}
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          ) : (
            <div className="h-full flex items-center justify-center text-gray-500">
              <div className="text-center">
                <FileText className="w-12 h-12 mx-auto mb-3 text-gray-300" />
                <p>No chunks found for this document</p>
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
            refetchCollections();
            if (selectedCollection) refetchCollection();
          }}
        />
      </Modal>

      {/* Delete Confirmation Modal */}
      <Modal
        isOpen={!!deleteConfirmation}
        onClose={() => setDeleteConfirmation(null)}
        title="Delete Collection"
        size="sm"
      >
        <div className="p-6">
          <div className="flex items-center gap-3 mb-4">
            <div className="p-2 bg-red-100 rounded-full">
              <AlertTriangle className="w-6 h-6 text-red-600" />
            </div>
            <div>
              <h3 className="font-semibold text-gray-900">Delete Collection</h3>
              <p className="text-sm text-gray-500">
                This action cannot be undone.
              </p>
            </div>
          </div>

          <p className="text-sm text-gray-700 mb-6">
            Are you sure you want to delete the collection{" "}
            <strong>"{deleteConfirmation?.name}"</strong>? This will remove all
            documents and chunks in this collection from the vector store.
          </p>

          <div className="flex gap-3 justify-end">
            <button
              onClick={() => setDeleteConfirmation(null)}
              className="px-4 py-2 text-gray-700 bg-gray-100 hover:bg-gray-200 rounded-lg transition-colors"
            >
              Cancel
            </button>
            <button
              onClick={confirmDelete}
              className="flex items-center gap-2 px-4 py-2 bg-red-600 text-white hover:bg-red-700 rounded-lg transition-colors"
            >
              Delete
            </button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
