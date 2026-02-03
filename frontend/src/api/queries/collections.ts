import { useQuery } from "@tanstack/react-query";
import type { CollectionsResponse, Collection } from "../../models/Collections";
import apiClient from "../client";

export function useCollectionList() {
  return useQuery<CollectionsResponse>({
    queryKey: ["collections"],
    queryFn: () => apiClient.listCollections(),
    staleTime: 5 * 60 * 1000, // 5 minutes
    refetchInterval: 10 * 60 * 1000, // 10 minutes
  });
}

export function useCollectionInfo(collectionName: string) {
  return useQuery<Collection>({
    queryKey: ["collection", collectionName],
    queryFn: () => apiClient.getCollectionInfo(collectionName),
    enabled: !!collectionName,
    staleTime: 5 * 60 * 1000, // 5 minutes
  });
}

export function useDocumentChunks(documentId: string) {
  return useQuery<any[]>({
    queryKey: ["document", "chunks", documentId],
    queryFn: () => apiClient.getDocumentChunks(documentId),
    enabled: !!documentId,
    staleTime: 5 * 60 * 1000, // 5 minutes
  });
}
