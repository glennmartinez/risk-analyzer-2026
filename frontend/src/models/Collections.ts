export interface CollectionsResponse {
  collections: string[];
  total: number;
}

export interface Collection {
  name: string;
  document_count: number;
  chunk_count: number;
  document_ids: string[];
  metadata: Record<string, unknown>;
}
