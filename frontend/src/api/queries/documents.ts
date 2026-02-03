/**
 * TanStack Query hooks for document operations
 * Uses the existing ApiClient for data fetching
 */

import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationOptions,
} from "@tanstack/react-query";
import { apiClient } from "../client";
import type {
  DocumentResponse,
  ListDocumentsResponse,
  ListVectorDocumentsResponse,
  GetDocumentChunksResponse,
  DeleteDocumentResponse,
  DeleteCollectionResponse,
} from "../../models/Documents";
import { documentKeys, healthKeys } from "./keys";

/**
 * Hook to upload a document
 * Returns a mutation that can be triggered with formData
 */
export function useUploadDocument(
  options?: Omit<
    UseMutationOptions<DocumentResponse, Error, FormData>,
    "mutationFn"
  >,
) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (formData: FormData) => apiClient.uploadDocument(formData),
    onSuccess: (...args) => {
      // Invalidate document lists to refetch after upload
      queryClient.invalidateQueries({ queryKey: documentKeys.lists() });
      queryClient.invalidateQueries({ queryKey: documentKeys.vectors() });

      // Call any additional onSuccess handler
      options?.onSuccess?.(...args);
    },
    ...options,
  });
}

/**
 * Hook to list all documents
 */
export function useListDocuments(enabled = true) {
  return useQuery<ListDocumentsResponse>({
    queryKey: documentKeys.lists(),
    queryFn: () => apiClient.listDocuments(),
    enabled,
  });
}

/**
 * Hook to list vector documents, optionally filtered by collection
 */
export function useListVectorDocuments(
  collectionName?: string,
  enabled = true,
) {
  return useQuery<ListVectorDocumentsResponse>({
    queryKey: documentKeys.vector(collectionName),
    queryFn: () => apiClient.listVectorDocuments(collectionName),
    enabled,
  });
}

/**
 * Hook to check document service health
 */
export function useDocumentServiceHealth(enabled = true) {
  return useQuery({
    queryKey: healthKeys.documentService(),
    queryFn: () => apiClient.documentServiceHealth(),
    enabled,
    // Health checks can be stale quickly
    staleTime: 30 * 1000, // 30 seconds
    refetchInterval: 60 * 1000, // Refetch every minute
  });
}

/**
 * Hook to get chunks for a specific document
 */
export function useDocumentChunks(
  documentId: string | null,
  collectionName?: string,
  limit?: number,
  offset?: number,
  enabled = true,
) {
  return useQuery<GetDocumentChunksResponse>({
    queryKey: [
      ...documentKeys.detail(documentId ?? ""),
      "chunks",
      { collectionName, limit, offset },
    ],
    queryFn: () =>
      apiClient.getDocumentChunks(documentId!, collectionName, limit, offset),
    enabled: enabled && !!documentId,
  });
}

/**
 * Hook to delete a document from vector store and Redis
 */
export function useDeleteDocument(
  options?: Omit<
    UseMutationOptions<
      DeleteDocumentResponse,
      Error,
      { documentId: string; collectionName?: string }
    >,
    "mutationFn"
  >,
) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ documentId, collectionName }) =>
      apiClient.deleteDocument(documentId, collectionName),
    onSuccess: (...args) => {
      // Invalidate all document lists to refetch after delete
      queryClient.invalidateQueries({ queryKey: documentKeys.all });
      options?.onSuccess?.(...args);
    },
    ...options,
  });
}

/**
 * Hook to list all collections from vector store
 */
export function useListCollections(enabled = true) {
  return useQuery<{ collections: string[] }>({
    queryKey: ["collections"],
    queryFn: () => apiClient.listCollections(),
    enabled,
  });
}

/**
 * Hook to delete an entire collection from vector store and Redis
 */
export function useDeleteCollection(
  options?: Omit<
    UseMutationOptions<DeleteCollectionResponse, Error, string>,
    "mutationFn"
  >,
) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (collectionName: string) =>
      apiClient.deleteCollection(collectionName),
    onSuccess: (...args) => {
      // Invalidate all document lists to refetch after delete
      queryClient.invalidateQueries({ queryKey: documentKeys.all });
      options?.onSuccess?.(...args);
    },
    ...options,
  });
}
