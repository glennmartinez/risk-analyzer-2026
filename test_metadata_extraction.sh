#!/bin/bash

# Integration test for metadata extraction in chunking flow
# This script tests that extract_metadata and num_questions params work correctly

set -e

PYTHON_BACKEND_URL="${PYTHON_BACKEND_URL:-http://localhost:8001}"
GO_BACKEND_URL="${GO_BACKEND_URL:-http://localhost:8080}"

echo "=============================================="
echo "Metadata Extraction Integration Test"
echo "=============================================="
echo "Python Backend: $PYTHON_BACKEND_URL"
echo "Go Backend: $GO_BACKEND_URL"
echo ""

# Test 1: Direct Python chunking endpoint WITHOUT metadata extraction
echo "Test 1: Chunk text WITHOUT metadata extraction"
echo "----------------------------------------------"
RESPONSE=$(curl -s -X POST "$PYTHON_BACKEND_URL/chunk/text" \
  -H "Content-Type: application/json" \
  -d '{
    "text": "Machine learning is a subset of artificial intelligence that enables systems to learn from data. Deep learning uses neural networks with multiple layers. Natural language processing helps computers understand human language.",
    "strategy": "sentence",
    "chunk_size": 100,
    "chunk_overlap": 10,
    "extract_metadata": false,
    "num_questions": 0
  }')

echo "Response (truncated):"
echo "$RESPONSE" | jq '{total_chunks, strategy_used, first_chunk_has_metadata: (.chunks[0].metadata != null)}'
echo ""

# Test 2: Direct Python chunking endpoint WITH metadata extraction
echo "Test 2: Chunk text WITH metadata extraction"
echo "--------------------------------------------"
RESPONSE=$(curl -s -X POST "$PYTHON_BACKEND_URL/chunk/text" \
  -H "Content-Type: application/json" \
  -d '{
    "text": "Machine learning is a subset of artificial intelligence that enables systems to learn from data. It involves algorithms that improve through experience. Deep learning uses neural networks with multiple layers to process complex patterns. Natural language processing helps computers understand human language. These technologies are transforming industries worldwide.",
    "strategy": "sentence",
    "chunk_size": 150,
    "chunk_overlap": 20,
    "extract_metadata": true,
    "num_questions": 3
  }')

echo "Response:"
echo "$RESPONSE" | jq '{
  total_chunks,
  strategy_used,
  chunks: [.chunks[] | {
    index,
    text_preview: (.text | .[0:50] + "..."),
    metadata: .metadata
  }]
}'
echo ""

# Test 3: Check if metadata fields are populated
echo "Test 3: Verify metadata fields"
echo "------------------------------"
HAS_TITLE=$(echo "$RESPONSE" | jq '[.chunks[].metadata.title | select(. != null)] | length > 0')
HAS_KEYWORDS=$(echo "$RESPONSE" | jq '[.chunks[].metadata.keywords | select(. != null and length > 0)] | length > 0')
HAS_QUESTIONS=$(echo "$RESPONSE" | jq '[.chunks[].metadata.questions | select(. != null and length > 0)] | length > 0')

echo "Has title in any chunk: $HAS_TITLE"
echo "Has keywords in any chunk: $HAS_KEYWORDS"
echo "Has questions in any chunk: $HAS_QUESTIONS"
echo ""

# Test 4: Go backend health check
echo "Test 4: Go backend health check"
echo "--------------------------------"
GO_HEALTH=$(curl -s "$GO_BACKEND_URL/health" 2>/dev/null || echo '{"error": "Go backend not reachable"}')
echo "$GO_HEALTH" | jq '.' 2>/dev/null || echo "$GO_HEALTH"
echo ""

# Test 5: Check available collections in ChromaDB
echo "Test 5: List collections (via Go backend)"
echo "------------------------------------------"
COLLECTIONS=$(curl -s "$GO_BACKEND_URL/api/v1/collections" 2>/dev/null || echo '{"error": "Could not fetch collections"}')
echo "$COLLECTIONS" | jq '.' 2>/dev/null || echo "$COLLECTIONS"
echo ""

# Test 6: Document upload test with metadata extraction (if a test file exists)
TEST_FILE="./Test_Data/example.pdf"
if [ -f "$TEST_FILE" ]; then
    echo "Test 6: Upload document WITH metadata extraction"
    echo "-------------------------------------------------"
    UPLOAD_RESPONSE=$(curl -s -X POST "$GO_BACKEND_URL/api/v1/documents/upload" \
      -F "file=@$TEST_FILE" \
      -F "collection=test_metadata" \
      -F "chunking_strategy=sentence" \
      -F "chunk_size=512" \
      -F "chunk_overlap=50" \
      -F "extract_metadata=true" \
      -F "num_questions=3" \
      -F "async=false")

    echo "Upload response:"
    echo "$UPLOAD_RESPONSE" | jq '.'

    DOC_ID=$(echo "$UPLOAD_RESPONSE" | jq -r '.document_id // empty')
    if [ -n "$DOC_ID" ] && [ "$DOC_ID" != "null" ]; then
        echo ""
        echo "Test 7: Fetch chunks for uploaded document"
        echo "-------------------------------------------"
        sleep 2  # Wait for processing

        CHUNKS_RESPONSE=$(curl -s "$GO_BACKEND_URL/api/v1/documents/$DOC_ID/chunks?limit=5")
        echo "Chunks response (first 5):"
        echo "$CHUNKS_RESPONSE" | jq '{
          total_count,
          chunks: [.chunks[:5][] | {
            id,
            chunk_index,
            text_preview: (.text | .[0:80] + "..."),
            has_title: (.metadata.title != null),
            has_keywords: (.metadata.keywords != null),
            has_questions: (.metadata.questions != null),
            metadata_keys: (.metadata | keys)
          }]
        }'
    fi
else
    echo "Test 6: Skipped (no test file at $TEST_FILE)"
fi

echo ""
echo "=============================================="
echo "Integration Tests Complete"
echo "=============================================="

# Summary
echo ""
echo "Summary:"
echo "--------"
if [ "$HAS_TITLE" = "true" ] || [ "$HAS_KEYWORDS" = "true" ] || [ "$HAS_QUESTIONS" = "true" ]; then
    echo "✅ Metadata extraction is WORKING"
    [ "$HAS_TITLE" = "true" ] && echo "   - Titles: ✅" || echo "   - Titles: ❌"
    [ "$HAS_KEYWORDS" = "true" ] && echo "   - Keywords: ✅" || echo "   - Keywords: ❌"
    [ "$HAS_QUESTIONS" = "true" ] && echo "   - Questions: ✅" || echo "   - Questions: ❌"
else
    echo "❌ Metadata extraction may NOT be working"
    echo "   Check that:"
    echo "   1. LLM is configured (OPENAI_API_KEY or local model)"
    echo "   2. extract_metadata=true is being passed"
    echo "   3. Python backend logs for errors"
fi
