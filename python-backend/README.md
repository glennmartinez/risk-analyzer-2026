# Document Processing Microservice

A Python microservice for PDF parsing with **Docling** and document chunking with **LlamaIndex**, designed to support the Go backend for the Risk Analyzer application.

## Features

- **PDF Parsing** with Docling

  - Text extraction
  - Table extraction with structure preservation
  - Figure/image detection
  - OCR support for scanned documents

- **Document Chunking** with LlamaIndex

  - Multiple chunking strategies (sentence, token, markdown, semantic)
  - Configurable chunk size and overlap
  - Metadata preservation

- **Vector Storage** with ChromaDB
  - Persistent storage
  - Semantic search with sentence-transformers
  - Collection management

## Quick Start

### 1. Install Dependencies

```bash
cd python-backend

# Create virtual environment
python -m venv venv
source venv/bin/activate  # On Windows: venv\Scripts\activate

# Install dependencies
pip install -r requirements.txt
```

### 2. Configure Environment

```bash
cp .env.example .env
# Edit .env as needed
```

### 3. Run the Server

```bash
# Development mode with auto-reload
uvicorn app.main:app --reload --host 0.0.0.0 --port 8000

# Or production mode
uvicorn app.main:app --host 0.0.0.0 --port 8000 --workers 4
```

### 4. Access API Documentation

- Swagger UI: http://localhost:8000/docs
- ReDoc: http://localhost:8000/redoc

## API Endpoints

### Document Processing

| Method | Endpoint                | Description                                                      |
| ------ | ----------------------- | ---------------------------------------------------------------- |
| POST   | `/documents/upload`     | Upload and process a document (parse + chunk + optionally store) |
| POST   | `/documents/parse`      | Parse document only (no chunking)                                |
| POST   | `/documents/chunk`      | Parse and chunk (no storage)                                     |
| DELETE | `/documents/{id}`       | Delete document from vector store                                |
| GET    | `/documents/strategies` | List chunking strategies                                         |
| GET    | `/documents/formats`    | List supported file formats                                      |

### Vector Search

| Method | Endpoint                           | Description                  |
| ------ | ---------------------------------- | ---------------------------- |
| POST   | `/search/`                         | Semantic search with filters |
| GET    | `/search/query?q=...`              | Quick search                 |
| GET    | `/search/collections`              | List collections             |
| GET    | `/search/collections/{name}/stats` | Collection statistics        |
| DELETE | `/search/collections/{name}`       | Reset collection             |

### Health

| Method | Endpoint  | Description          |
| ------ | --------- | -------------------- |
| GET    | `/health` | Service health check |
| GET    | `/ready`  | Readiness probe      |
| GET    | `/live`   | Liveness probe       |

## Usage Examples

### Upload and Process a PDF

```bash
curl -X POST "http://localhost:8000/documents/upload" \
  -F "file=@document.pdf" \
  -F "chunking_strategy=sentence" \
  -F "chunk_size=512" \
  -F "store_in_vector_db=true"
```

### Search Documents

```bash
# Simple search
curl "http://localhost:8000/search/query?q=risk%20assessment&top_k=5"

# Advanced search with filters
curl -X POST "http://localhost:8000/search/" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "security vulnerabilities",
    "top_k": 10,
    "filter_metadata": {"document_id": "abc123"}
  }'
```

### Parse Only (No Chunking)

```bash
curl -X POST "http://localhost:8000/documents/parse" \
  -F "file=@document.pdf"
```

## Docker

### Build and Run

```bash
docker build -t doc-processor .
docker run -p 8000:8000 -v $(pwd)/uploads:/app/uploads -v $(pwd)/chroma_db:/app/chroma_db doc-processor
```

### With Docker Compose

The service is configured in the main `docker-compose.yml`:

```bash
docker-compose up python-backend
```

## Integration with Go Backend

The Python microservice is designed to be called from the Go backend:

```go
// Example: Call from Go backend
resp, err := http.Post(
    "http://localhost:8000/documents/upload",
    "multipart/form-data",
    fileData,
)
```

## Chunking Strategies

| Strategy    | Description                     | Best For               |
| ----------- | ------------------------------- | ---------------------- |
| `sentence`  | Split on sentence boundaries    | General text, articles |
| `token`     | Split by token count            | Consistent chunk sizes |
| `markdown`  | Preserve markdown structure     | Structured documents   |
| `semantic`  | Group semantically similar text | Complex documents      |
| `recursive` | Hierarchical splitting          | Mixed content          |

## Project Structure

```
python-backend/
├── app/
│   ├── __init__.py
│   ├── main.py              # FastAPI app entry point
│   ├── config.py            # Configuration settings
│   ├── models.py            # Pydantic models
│   ├── routes/
│   │   ├── __init__.py
│   │   ├── documents.py     # Document processing routes
│   │   ├── search.py        # Vector search routes
│   │   └── health.py        # Health check routes
│   └── services/
│       ├── __init__.py
│       ├── parser.py        # Docling document parser
│       ├── chunker.py       # LlamaIndex chunker
│       └── vector_store.py  # ChromaDB integration
├── uploads/                 # Temporary file storage
├── chroma_db/              # ChromaDB persistence
├── requirements.txt
├── Dockerfile
├── .env.example
└── README.md
```

## Environment Variables

| Variable             | Default               | Description                |
| -------------------- | --------------------- | -------------------------- |
| `PORT`               | 8000                  | Server port                |
| `UPLOAD_DIR`         | ./uploads             | File upload directory      |
| `CHUNK_SIZE`         | 512                   | Default chunk size         |
| `CHUNK_OVERLAP`      | 50                    | Default chunk overlap      |
| `CHROMA_PERSIST_DIR` | ./chroma_db           | ChromaDB storage           |
| `EMBEDDING_MODEL`    | all-MiniLM-L6-v2      | Sentence transformer model |
| `GO_BACKEND_URL`     | http://localhost:8080 | Go backend URL             |
