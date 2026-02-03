/**
 * Query key factory for TanStack Query
 * Provides type-safe, consistent query keys for cache management
 */

export const documentKeys = {
  all: ["documents"] as const,
  lists: () => [...documentKeys.all, "list"] as const,
  list: (filters?: { collectionName?: string }) =>
    [...documentKeys.lists(), filters] as const,
  vectors: () => [...documentKeys.all, "vector"] as const,
  vector: (collectionName?: string) =>
    [...documentKeys.vectors(), { collectionName }] as const,
  details: () => [...documentKeys.all, "detail"] as const,
  detail: (id: string) => [...documentKeys.details(), id] as const,
};

export const healthKeys = {
  all: ["health"] as const,
  documentService: () => [...healthKeys.all, "documentService"] as const,
};
