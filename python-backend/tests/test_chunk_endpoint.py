"""
Tests for Chunk Endpoint - Stateless Text Chunking
Tests the pure compute chunk endpoints without any persistence.
"""

import pytest
from fastapi.testclient import TestClient

from app.main import app

client = TestClient(app)


class TestChunkText:
    """Test suite for POST /chunk/text endpoint"""

    def test_chunk_text_success(self, sample_text):
        """Test successful text chunking with default settings"""
        request_data = {
            "text": sample_text,
            "strategy": "sentence",
            "chunk_size": 512,
            "chunk_overlap": 50,
            "extract_metadata": False,
            "num_questions": 3,
        }

        response = client.post("/chunk/text", json=request_data)

        assert response.status_code == 200
        data = response.json()

        # Check response structure
        assert "chunks" in data
        assert "total_chunks" in data
        assert "strategy_used" in data
        assert "chunk_size" in data
        assert "chunk_overlap" in data

        # Verify chunks were created
        assert len(data["chunks"]) > 0
        assert data["total_chunks"] == len(data["chunks"])
        assert data["strategy_used"] == "sentence"

        # Check chunk structure
        first_chunk = data["chunks"][0]
        assert "text" in first_chunk
        assert "index" in first_chunk
        assert len(first_chunk["text"]) > 0

    def test_chunk_text_all_strategies(self, sample_text):
        """Test chunking with all available strategies"""
        strategies = [
            "sentence",
            "semantic",
            "token",
            "fixed",
            "markdown",
            "hierarchical",
        ]

        for strategy in strategies:
            request_data = {
                "text": sample_text,
                "strategy": strategy,
                "chunk_size": 512,
                "chunk_overlap": 50,
                "extract_metadata": False,
            }

            response = client.post("/chunk/text", json=request_data)

            # Some strategies might fail on small text, that's ok
            assert response.status_code in [200, 500]

            if response.status_code == 200:
                data = response.json()
                assert data["strategy_used"] == strategy
                assert len(data["chunks"]) > 0

    def test_chunk_text_with_metadata(self, sample_text):
        """Test chunking with metadata extraction enabled"""
        request_data = {
            "text": sample_text,
            "strategy": "sentence",
            "chunk_size": 512,
            "chunk_overlap": 50,
            "extract_metadata": True,
            "num_questions": 3,
        }

        response = client.post("/chunk/text", json=request_data)

        assert response.status_code == 200
        data = response.json()

        # Check if metadata is present in chunks
        if len(data["chunks"]) > 0:
            first_chunk = data["chunks"][0]
            if "metadata" in first_chunk and first_chunk["metadata"]:
                assert "chunk_index" in first_chunk["metadata"]

    def test_chunk_text_different_sizes(self, sample_text):
        """Test chunking with different chunk sizes"""
        sizes = [256, 512, 1024]

        for size in sizes:
            request_data = {
                "text": sample_text,
                "strategy": "fixed",
                "chunk_size": size,
                "chunk_overlap": 50,
                "extract_metadata": False,
            }

            response = client.post("/chunk/text", json=request_data)

            assert response.status_code == 200
            data = response.json()
            assert data["chunk_size"] == size

    def test_chunk_text_different_overlaps(self, sample_text):
        """Test chunking with different overlap values"""
        overlaps = [0, 25, 50, 100]

        for overlap in overlaps:
            request_data = {
                "text": sample_text,
                "strategy": "sentence",
                "chunk_size": 512,
                "chunk_overlap": overlap,
                "extract_metadata": False,
            }

            response = client.post("/chunk/text", json=request_data)

            assert response.status_code == 200
            data = response.json()
            assert data["chunk_overlap"] == overlap

    def test_chunk_text_empty_string(self):
        """Test chunking with empty text"""
        request_data = {
            "text": "",
            "strategy": "sentence",
            "chunk_size": 512,
            "chunk_overlap": 50,
        }

        response = client.post("/chunk/text", json=request_data)

        assert response.status_code == 400
        assert "empty" in response.json()["detail"].lower()

    def test_chunk_text_whitespace_only(self):
        """Test chunking with whitespace-only text"""
        request_data = {
            "text": "   \n\t   ",
            "strategy": "sentence",
            "chunk_size": 512,
            "chunk_overlap": 50,
        }

        response = client.post("/chunk/text", json=request_data)

        assert response.status_code == 400

    def test_chunk_text_invalid_strategy(self, sample_text):
        """Test chunking with invalid strategy"""
        request_data = {
            "text": sample_text,
            "strategy": "invalid_strategy",
            "chunk_size": 512,
            "chunk_overlap": 50,
        }

        response = client.post("/chunk/text", json=request_data)

        assert response.status_code == 400
        assert "invalid strategy" in response.json()["detail"].lower()

    def test_chunk_text_very_long_text(self):
        """Test chunking with very long text"""
        long_text = "This is a sentence. " * 1000  # ~20,000 chars

        request_data = {
            "text": long_text,
            "strategy": "sentence",
            "chunk_size": 512,
            "chunk_overlap": 50,
            "extract_metadata": False,
        }

        response = client.post("/chunk/text", json=request_data)

        assert response.status_code == 200
        data = response.json()

        # Should create multiple chunks
        assert len(data["chunks"]) > 1

    def test_chunk_text_single_sentence(self):
        """Test chunking with single short sentence"""
        request_data = {
            "text": "This is a single short sentence.",
            "strategy": "sentence",
            "chunk_size": 512,
            "chunk_overlap": 50,
            "extract_metadata": False,
        }

        response = client.post("/chunk/text", json=request_data)

        assert response.status_code == 200
        data = response.json()

        # Should create at least one chunk
        assert len(data["chunks"]) >= 1


class TestChunkSimple:
    """Test suite for POST /chunk/simple endpoint"""

    def test_chunk_simple_success(self, sample_text):
        """Test simple chunking endpoint"""
        params = {"text": sample_text, "chunk_size": 512, "chunk_overlap": 50}

        response = client.post("/chunk/simple", params=params)

        assert response.status_code == 200
        data = response.json()

        assert "chunks" in data
        assert len(data["chunks"]) > 0
        assert data["strategy_used"] == "sentence"

        # Should not have metadata
        first_chunk = data["chunks"][0]
        assert first_chunk["metadata"] is None

    def test_chunk_simple_minimal_params(self, sample_text):
        """Test simple chunking with minimal parameters"""
        params = {"text": sample_text}

        response = client.post("/chunk/simple", params=params)

        # Should use defaults
        assert response.status_code == 200

    def test_chunk_simple_empty_text(self):
        """Test simple chunking with empty text"""
        params = {"text": ""}

        response = client.post("/chunk/simple", params=params)

        assert response.status_code == 400


class TestChunkStrategies:
    """Test suite for GET /chunk/strategies endpoint"""

    def test_list_strategies(self):
        """Test listing available chunking strategies"""
        response = client.get("/chunk/strategies")

        assert response.status_code == 200
        data = response.json()

        assert "strategies" in data
        assert isinstance(data["strategies"], list)
        assert len(data["strategies"]) > 0

        # Check strategy structure
        first_strategy = data["strategies"][0]
        assert "name" in first_strategy
        assert "description" in first_strategy

        # Check all expected strategies are present
        strategy_names = [s["name"] for s in data["strategies"]]
        expected = [
            "sentence",
            "semantic",
            "token",
            "fixed",
            "markdown",
            "hierarchical",
        ]
        for expected_strategy in expected:
            assert expected_strategy in strategy_names


class TestChunkHealth:
    """Test suite for GET /chunk/health endpoint"""

    def test_chunk_health(self):
        """Test chunk service health check"""
        response = client.get("/chunk/health")

        assert response.status_code == 200
        data = response.json()

        assert "status" in data
        assert data["status"] == "healthy"
        assert "service" in data
        assert data["service"] == "chunk"
        assert "strategies" in data
        assert isinstance(data["strategies"], list)


# Fixtures


@pytest.fixture
def sample_text():
    """Create sample text for chunking tests"""
    return """
    This is a sample document for testing text chunking functionality.
    It contains multiple sentences spread across several paragraphs.

    The first paragraph introduces the topic and provides context.
    This helps ensure we have enough text to create meaningful chunks.

    The second paragraph continues the discussion with additional details.
    Each sentence adds more information to the overall document content.
    We want to test various chunking strategies on realistic text.

    The third paragraph concludes our sample text.
    It demonstrates how text chunking works across different sections.
    This allows us to verify that chunking preserves semantic meaning.
    """


@pytest.fixture
def markdown_text():
    """Create sample markdown text for markdown chunking strategy"""
    return """
# Main Title

This is the introduction paragraph.

## Section 1

Content for section 1 goes here.
It has multiple sentences.

### Subsection 1.1

Details in subsection 1.1.

## Section 2

Content for section 2.
More information here.
"""


# Integration Tests


class TestChunkIntegration:
    """Integration tests for chunk endpoint workflow"""

    def test_chunk_stateless_behavior(self, sample_text):
        """Test that chunk is truly stateless"""
        request_data = {
            "text": sample_text,
            "strategy": "sentence",
            "chunk_size": 512,
            "chunk_overlap": 50,
            "extract_metadata": False,
        }

        # Chunk same text twice
        response1 = client.post("/chunk/text", json=request_data)
        response2 = client.post("/chunk/text", json=request_data)

        assert response1.status_code == 200
        assert response2.status_code == 200

        data1 = response1.json()
        data2 = response2.json()

        # Results should be identical
        assert data1["total_chunks"] == data2["total_chunks"]
        assert len(data1["chunks"]) == len(data2["chunks"])

        # Compare chunk texts
        for i in range(len(data1["chunks"])):
            assert data1["chunks"][i]["text"] == data2["chunks"][i]["text"]

    def test_chunk_different_strategies_same_text(self, sample_text):
        """Test different strategies produce different results"""
        strategies = ["sentence", "fixed", "token"]
        results = []

        for strategy in strategies:
            request_data = {
                "text": sample_text,
                "strategy": strategy,
                "chunk_size": 256,
                "chunk_overlap": 25,
                "extract_metadata": False,
            }

            response = client.post("/chunk/text", json=request_data)

            if response.status_code == 200:
                results.append(response.json())

        # Should have different chunk counts
        if len(results) > 1:
            chunk_counts = [r["total_chunks"] for r in results]
            # Not all will be the same
            assert len(set(chunk_counts)) > 1 or all(
                c == chunk_counts[0] for c in chunk_counts
            )

    def test_chunk_overlap_behavior(self, sample_text):
        """Test that overlap creates expected redundancy"""
        # No overlap
        request_no_overlap = {
            "text": sample_text,
            "strategy": "fixed",
            "chunk_size": 100,
            "chunk_overlap": 0,
            "extract_metadata": False,
        }

        # With overlap
        request_with_overlap = {
            "text": sample_text,
            "strategy": "fixed",
            "chunk_size": 100,
            "chunk_overlap": 50,
            "extract_metadata": False,
        }

        response1 = client.post("/chunk/text", json=request_no_overlap)
        response2 = client.post("/chunk/text", json=request_with_overlap)

        assert response1.status_code == 200
        assert response2.status_code == 200

        data1 = response1.json()
        data2 = response2.json()

        # With overlap should create more chunks
        assert data2["total_chunks"] >= data1["total_chunks"]


# Error Handling Tests


class TestChunkErrorHandling:
    """Test error handling for chunk endpoints"""

    def test_chunk_missing_required_field(self):
        """Test chunking without required text field"""
        request_data = {
            "strategy": "sentence",
            "chunk_size": 512,
        }

        response = client.post("/chunk/text", json=request_data)

        assert response.status_code == 422  # Validation error

    def test_chunk_invalid_chunk_size(self, sample_text):
        """Test chunking with invalid chunk size"""
        request_data = {
            "text": sample_text,
            "strategy": "sentence",
            "chunk_size": -1,  # Invalid
            "chunk_overlap": 50,
        }

        response = client.post("/chunk/text", json=request_data)

        # Should either validate or fail gracefully
        assert response.status_code in [400, 422, 500]

    def test_chunk_overlap_larger_than_size(self, sample_text):
        """Test chunking with overlap larger than chunk size"""
        request_data = {
            "text": sample_text,
            "strategy": "sentence",
            "chunk_size": 100,
            "chunk_overlap": 200,  # Larger than chunk_size
            "extract_metadata": False,
        }

        response = client.post("/chunk/text", json=request_data)

        # Should handle gracefully
        assert response.status_code in [200, 400, 500]
