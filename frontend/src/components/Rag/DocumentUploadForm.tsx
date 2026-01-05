/**
 * Document Upload Form
 * Uses TanStack Form for form state management and TanStack Query for submission
 */

import { useForm } from "@tanstack/react-form";
import { useUploadDocument } from "../../api/queries";
import type { DocumentRequest } from "../../models/Documents";

type FormValues = Omit<DocumentRequest, never> & {
  file: File | null;
};

const defaultValues: FormValues = {
  file: null,
  chunking_strategy: "sentence",
  chunk_size: 512,
  chunk_overlap: 50,
  store_in_vector_db: true,
  extract_tables: false,
  extract_figures: false,
  extract_metadata: false,
  num_questions: 3,
  max_pages: 10,
  collection_name: "default",
};

interface DocumentUploadFormProps {
  onSuccess?: () => void;
}

export function DocumentUploadForm({ onSuccess }: DocumentUploadFormProps) {
  const uploadMutation = useUploadDocument({
    onSuccess: (data) => {
      console.log("Upload successful:", data);
      form.reset();
      onSuccess?.();
    },
    onError: (error) => {
      console.error("Upload failed:", error);
    },
  });

  const form = useForm({
    defaultValues,
    onSubmit: async ({ value }) => {
      if (!value.file) {
        return;
      }

      const formData = new FormData();
      formData.append("file", value.file);
      formData.append("chunking_strategy", value.chunking_strategy);
      formData.append("chunk_size", value.chunk_size.toString());
      formData.append("chunk_overlap", value.chunk_overlap.toString());
      formData.append(
        "store_in_vector_db",
        value.store_in_vector_db.toString(),
      );
      formData.append("extract_tables", value.extract_tables.toString());
      formData.append("extract_figures", value.extract_figures.toString());
      formData.append("extract_metadata", value.extract_metadata.toString());
      formData.append("num_questions", value.num_questions.toString());
      formData.append("max_pages", value.max_pages.toString());
      formData.append("collection_name", value.collection_name);

      await uploadMutation.mutateAsync(formData);
    },
  });

  return (
    <div className="max-w-2xl mx-auto p-6">
      <h2 className="text-2xl font-bold mb-6">Upload Document</h2>

      {uploadMutation.isSuccess && (
        <div className="mb-4 p-4 bg-green-100 border border-green-400 text-green-700 rounded">
          Document uploaded successfully! ID: {uploadMutation.data?.document_id}
        </div>
      )}

      {uploadMutation.isError && (
        <div className="mb-4 p-4 bg-red-100 border border-red-400 text-red-700 rounded">
          Upload failed: {uploadMutation.error?.message}
        </div>
      )}

      <form
        onSubmit={(e) => {
          e.preventDefault();
          e.stopPropagation();
          form.handleSubmit();
        }}
        className="space-y-6"
      >
        {/* File Input */}
        <form.Field
          name="file"
          validators={{
            onChange: ({ value }) =>
              !value ? "Please select a file" : undefined,
          }}
        >
          {(field) => (
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                File *
              </label>
              <input
                type="file"
                accept=".pdf,.doc,.docx,.txt,.md"
                onChange={(e) =>
                  field.handleChange(e.target.files?.[0] ?? null)
                }
                className="block w-full text-sm text-gray-500 file:mr-4 file:py-2 file:px-4 file:rounded file:border-0 file:text-sm file:font-semibold file:bg-blue-50 file:text-blue-700 hover:file:bg-blue-100"
              />
              {field.state.meta.errors.length > 0 && (
                <p className="mt-1 text-sm text-red-600">
                  {field.state.meta.errors.join(", ")}
                </p>
              )}
            </div>
          )}
        </form.Field>

        {/* Chunking Strategy */}
        <form.Field name="chunking_strategy">
          {(field) => (
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Chunking Strategy
              </label>
              <select
                value={field.state.value}
                onChange={(e) => field.handleChange(e.target.value)}
                className="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 p-2 border"
              >
                <option value="semantic">Semantic</option>
                <option value="fixed">Fixed Size</option>
                <option value="sentence">Sentence</option>
                <option value="paragraph">Paragraph</option>
              </select>
            </div>
          )}
        </form.Field>

        {/* Chunk Size & Overlap */}
        <div className="grid grid-cols-2 gap-4">
          <form.Field
            name="chunk_size"
            validators={{
              onChange: ({ value }) =>
                value < 100 ? "Minimum chunk size is 100" : undefined,
            }}
          >
            {(field) => (
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Chunk Size
                </label>
                <input
                  type="number"
                  min={100}
                  max={2000}
                  value={field.state.value}
                  onChange={(e) => field.handleChange(Number(e.target.value))}
                  className="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 p-2 border"
                />
                {field.state.meta.errors.length > 0 && (
                  <p className="mt-1 text-sm text-red-600">
                    {field.state.meta.errors.join(", ")}
                  </p>
                )}
              </div>
            )}
          </form.Field>

          <form.Field
            name="chunk_overlap"
            validators={{
              onChange: ({ value }) =>
                value < 0 ? "Overlap cannot be negative" : undefined,
            }}
          >
            {(field) => (
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Chunk Overlap
                </label>
                <input
                  type="number"
                  min={0}
                  max={500}
                  value={field.state.value}
                  onChange={(e) => field.handleChange(Number(e.target.value))}
                  className="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 p-2 border"
                />
                {field.state.meta.errors.length > 0 && (
                  <p className="mt-1 text-sm text-red-600">
                    {field.state.meta.errors.join(", ")}
                  </p>
                )}
              </div>
            )}
          </form.Field>
        </div>

        {/* Max Pages & Num Questions */}
        <div className="grid grid-cols-2 gap-4">
          <form.Field
            name="max_pages"
            validators={{
              onChange: ({ value }) =>
                value < 1 ? "Must process at least 1 page" : undefined,
            }}
          >
            {(field) => (
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Max Pages
                </label>
                <input
                  type="number"
                  min={1}
                  max={100}
                  value={field.state.value}
                  onChange={(e) => field.handleChange(Number(e.target.value))}
                  className="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 p-2 border"
                />
                {field.state.meta.errors.length > 0 && (
                  <p className="mt-1 text-sm text-red-600">
                    {field.state.meta.errors.join(", ")}
                  </p>
                )}
              </div>
            )}
          </form.Field>

          <form.Field name="num_questions">
            {(field) => (
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Questions per Chunk
                </label>
                <input
                  type="number"
                  min={0}
                  max={10}
                  value={field.state.value}
                  onChange={(e) => field.handleChange(Number(e.target.value))}
                  className="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 p-2 border"
                />
              </div>
            )}
          </form.Field>
        </div>

        {/* Collection Name */}
        <form.Field
          name="collection_name"
          validators={{
            onChange: ({ value }) =>
              !value.trim() ? "Collection name is required" : undefined,
          }}
        >
          {(field) => (
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Collection Name
              </label>
              <input
                type="text"
                value={field.state.value}
                onChange={(e) => field.handleChange(e.target.value)}
                className="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 p-2 border"
                placeholder="default"
              />
              {field.state.meta.errors.length > 0 && (
                <p className="mt-1 text-sm text-red-600">
                  {field.state.meta.errors.join(", ")}
                </p>
              )}
            </div>
          )}
        </form.Field>

        {/* Boolean Options */}
        <div className="space-y-3">
          <form.Field name="store_in_vector_db">
            {(field) => (
              <label className="flex items-center space-x-3">
                <input
                  type="checkbox"
                  checked={field.state.value}
                  onChange={(e) => field.handleChange(e.target.checked)}
                  className="rounded border-gray-300 text-blue-600 focus:ring-blue-500 h-4 w-4"
                />
                <span className="text-sm text-gray-700">
                  Store in Vector DB
                </span>
              </label>
            )}
          </form.Field>

          <form.Field name="extract_tables">
            {(field) => (
              <label className="flex items-center space-x-3">
                <input
                  type="checkbox"
                  checked={field.state.value}
                  onChange={(e) => field.handleChange(e.target.checked)}
                  className="rounded border-gray-300 text-blue-600 focus:ring-blue-500 h-4 w-4"
                />
                <span className="text-sm text-gray-700">Extract Tables</span>
              </label>
            )}
          </form.Field>

          <form.Field name="extract_figures">
            {(field) => (
              <label className="flex items-center space-x-3">
                <input
                  type="checkbox"
                  checked={field.state.value}
                  onChange={(e) => field.handleChange(e.target.checked)}
                  className="rounded border-gray-300 text-blue-600 focus:ring-blue-500 h-4 w-4"
                />
                <span className="text-sm text-gray-700">Extract Figures</span>
              </label>
            )}
          </form.Field>

          <form.Field name="extract_metadata">
            {(field) => (
              <label className="flex items-center space-x-3">
                <input
                  type="checkbox"
                  checked={field.state.value}
                  onChange={(e) => field.handleChange(e.target.checked)}
                  className="rounded border-gray-300 text-blue-600 focus:ring-blue-500 h-4 w-4"
                />
                <span className="text-sm text-gray-700">
                  Extract Metadata (LLM-powered)
                </span>
              </label>
            )}
          </form.Field>
        </div>

        {/* Submit Button */}
        <form.Subscribe
          selector={(state) => [state.canSubmit, state.isSubmitting]}
        >
          {([canSubmit, isSubmitting]) => (
            <button
              type="submit"
              disabled={!canSubmit || uploadMutation.isPending}
              className="w-full py-2 px-4 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {uploadMutation.isPending || isSubmitting
                ? "Uploading..."
                : "Upload Document"}
            </button>
          )}
        </form.Subscribe>
      </form>
    </div>
  );
}
