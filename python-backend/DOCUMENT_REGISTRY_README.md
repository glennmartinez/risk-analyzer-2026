# Document Registry & Filtering System

This functionality enables precise tracking of processed documents and allows for filtered RAG (Retrieval Augmented Generation) searches.

## Architecture
- **Storage**: We use **Redis** to maintain a lightweight registry of all processed documents.
- **Metadata**: Stores filename, chunk counts, processing timestamps, and vector DB collection status.
- **Integration**: 
  - **Uploads**: Automatically registers docs upon successful processing.
  - **Search**: Checks registry to allow filtering vector searches by specific `document_id`.

## Usage

### 1. Ingestion (Automatic)
No extra steps required. When you upload a document via `/documents/upload` or run `/documents/process-example`, it is automatically registered.

### 2. Implementation Details
- **Service**: `app/services/redis_service.py` handles the registry logic.
- **Key Pattern**: `doc:{document_id}` (Hash) and `docs:all` (Set).

### 3. API Endpoints

#### List Processed Documents
View all documents available for RAG.
```http
GET /documents/list
```
**Response:**
```json
{
  "documents": [
    {
      "document_id": "c4d2...",
      "filename": "annual_report.pdf",
      "chunk_count": "42",
      "registered_at": "2025-12-31T03:00:00"
    }
  ],
  "total": 1
}
```

#### Filtered RAG Search
Perform a semantic search *restricted to a single document* by passing its ID.
```http
POST /rag/search
Content-Type: application/json

{
  "query": "financial risk factors",
  "document_id": "c4d2..." 
}
```
*If `document_id` is omitted, the search runs across ALL documents in the collection.*

### 4. Docker Support
A `redis` service is included in the `docker-compose.yml`. 
- **Internal Host**: `redis`
- **Port**: `6379`
- **Persistence**: Data is saved to `redis_data` volume.

## 5. Internal Logic Details

### Parsing (Docling)
We use [Docling](https://github.com/DS4SD/docling) to parse documents. This allows us to:
- Recover table structures (rows/columns) rather than just raw text.
- Identify headers and page numbers.
- Fallback to `PyMuPDF` if Docling fails for specific files.

### Chunking (LlamaIndex)
We use `LlamaIndex` node parsers.
- **Default Strategy**: `SentenceSplitter` (splits by sentences to preserve meaning, respects `chunk_size` limit).
- **Metadata**: Each chunk retains the filename, page number, and other extracted metadata.

### Embedding (SentenceTransformers)
- **Model**: `all-MiniLM-L6-v2` (Configurable via `EMBEDDING_MODEL` env var).
- **Process**:
  1. Chunks are generated.
  2. Each chunk's text is passed to the local Transformer model.
  3. A 384-dimensional vector is generated.
  4. Vector + Text + Metadata is stored in ChromaDB.

## 6. API Reference

### Upload & Process Document
**POST** `/documents/upload`

**Payload (Multipart/Form-Data):**
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `file` | File | Required | The PDF or document file to upload. |
| `chunking_strategy` | String | `sentence` | Strategy: `sentence`, `token`, `markdown`, `semantic`. |
| `chunk_size` | Integer | `512` | Maximum tokens per chunk. |
| `chunk_overlap` | Integer | `50` | Overlap between chunks. |
| `extract_tables` | Boolean | `true` | Whether to extract tables using Docling. |
| `extract_figures` | Boolean | `true` | Whether to extract figures/images. |
| `extract_metadata` | Boolean | `false` | Enable LLM extraction of title, questions, keywords. |
| `num_questions` | Integer | `3` | Number of questions to generate per chunk (1-10). |
| `max_pages` | Integer | `30` | Maximum number of pages to process (1-500). |
| `collection_name` | String | `documents` | Optional ChromaDB collection name. |
| `store_in_vector_db` | Boolean | `false` | Whether to store chunks in ChromaDB. |

**Example Request:**
```bash
curl -X POST "http://localhost:8000/documents/upload" \
  -F "file=@/path/to/annual_report.pdf" \
  -F "chunking_strategy=sentence" \
  -F "chunk_size=512" \
  -F "chunk_overlap=50" \
  -F "extract_tables=true" \
  -F "extract_figures=true" \
  -F "extract_metadata=true" \
  -F "num_questions=3" \
  -F "max_pages=30" \
  -F "store_in_vector_db=true"
```

**Response (JSON):**
```json
{
  "success": true,
  "document_id": "c4d24660-323b-4860-9302-3c4ba7e8417c",
  "message": "Successfully processed annual_report.pdf",
  "metadata": {
    "filename": "annual_report.pdf",
    "file_type": "pdf",
    "page_count": 10,
    "file_size_bytes": 1048576,
    "title": "Annual Report 2024"
  },
  "chunk_count": 42,
  "vector_db_stored": true,
  "processing_time_seconds": 1.23
}
```

### Note on JSON Usage
The Upload endpoint requires `multipart/form-data` because it accepts a file. You **cannot** strictly send a `application/json` payload. 

However, if you want to understand the **parameters as a JSON object** (for example, if using a client that converts a JSON config into form fields), it would look like this:

**Conceptual JSON Configuration:**
```json
{
  "chunking_strategy": "sentence",
  "chunk_size": 512,
  "chunk_overlap": 50,
  "extract_tables": true,
  "extract_figures": true,
  "store_in_vector_db": true, 
  "collection_name": "documents"
}
```
*(Remember: In the actual API call, these must be sent as individual Form fields along with the `file` part.)*

### RAG Search (Filtered)
**POST** `/rag/search`

**Example Request:**
```bash
curl -X POST "http://localhost:8000/rag/search" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "financial risks",
    "document_id": "c4d24660-323b-4860-9302-3c4ba7e8417c",
    "top_k": 3
  }'
```

**Payload (JSON):**
```json
{
  "query": "What are the financial risks?",
  "document_id": "c4d24660-323b-4860-9302-3c4ba7e8417c", 
  "top_k": 5
}
```
*Note: `document_id` is optional. If provided, search is restricted to that document.*

**Response (JSON):**
```json
{
  "query": "What are the financial risks?",
  "results": [
    {
      "chunk_id": "chunk_0",
      "text": "The primary financial risk is...",
      "score": 0.89,
      "metadata": {
        "page_number": 3,
        "section": "Risk Factors"
      }
    }
  ],
  "total_results": 1,
  "search_time_seconds": 0.05
}
```

## 7. Testing with Postman
To test the **Upload** endpoint in Postman:

1.  **Method**: Select `POST`.
2.  **URL**: Enter `http://localhost:8000/documents/upload`.
3.  **Body Tab**: Select **Body** > **form-data**.
4.  **Fields**:
    *   **Key**: `file` (Change dropdown type from "Text" to **"File"**). Select your PDF.
    *   **Key**: `chunking_strategy` -> **Value**: `sentence`
    *   **Key**: `store_in_vector_db` -> **Value**: `true`
    *   *(Add other fields like `chunk_size` as needed)*.
5.  **Send**: Click Send.

To test the **Search** endpoint:
1.  **Method**: Select `POST`.
2.  **URL**: Enter `http://localhost:8000/rag/search`.
3.  **Body Tab**: Select **Body** > **raw** > **JSON**.
4.  **Payload**: Paste the JSON payload (e.g., `{"query": "...", "document_id": "..."}`).
5.  **Send**: Click Send.
