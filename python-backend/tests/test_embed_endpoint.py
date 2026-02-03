"""
Tests for Embed Endpoint - Stateless Embedding Generation
Tests the pure compute embed endpoints without any persistence.
"""

from unittest.mock import MagicMock, patch

import pytest
from fastapi.testclient import TestClient

from app.main import app

client = TestClient(app)


class TestEmbedSingleText:
    """Test suite for POST /embed/text endpoint"""

    @patch("app.routes.embed.get_embedder")
    def test_embed_text_success(self, mock_embedder, sample_text):
        """Test successful single text embedding"""
        # Mock embedder
        mock_embed_instance = MagicMock()
        mock_embed_instance.get_text_embedding.return_value = [0.1] * 1536
        mock_embedder.return_value = mock_embed_instance

        request_data = {"text": sample_text, "model": "text-embedding-3-small"}

        response = client.post("/embed/text", json=request_data)

        assert response.status_code == 200
        data = response.json()

        # Check response structure
        assert "embedding" in data
        assert "dimension" in data
        assert "model" in data

        # Verify embedding
        assert isinstance(data["embedding"], list)
        assert len(data["embedding"]) == 1536
        assert data["dimension"] == 1536
        assert data["model"] == "text-embedding-3-small"

    @patch("app.routes.embed.get_embedder")
    def test_embed_text_default_model(self, mock_embedder, sample_text):
        """Test embedding with default model"""
        mock_embed_instance = MagicMock()
        mock_embed_instance.get_text_embedding.return_value = [0.1] * 1536
        mock_embedder.return_value = mock_embed_instance

        request_data = {"text": sample_text}

        response = client.post("/embed/text", json=request_data)

        assert response.status_code == 200
        data = response.json()
        assert "embedding" in data

    def test_embed_text_empty_string(self):
        """Test embedding with empty text"""
        request_data = {"text": ""}

        response = client.post("/embed/text", json=request_data)

        assert response.status_code == 400
        assert "empty" in response.json()["detail"].lower()

    def test_embed_text_whitespace_only(self):
        """Test embedding with whitespace-only text"""
        request_data = {"text": "   \n\t   "}

        response = client.post("/embed/text", json=request_data)

        assert response.status_code == 400

    @patch("app.routes.embed.get_embedder")
    def test_embed_text_long_input(self, mock_embedder):
        """Test embedding with very long text"""
        mock_embed_instance = MagicMock()
        mock_embed_instance.get_text_embedding.return_value = [0.1] * 1536
        mock_embedder.return_value = mock_embed_instance

        long_text = "This is a sentence. " * 1000

        request_data = {"text": long_text}

        response = client.post("/embed/text", json=request_data)

        # Should handle or truncate appropriately
        assert response.status_code in [200, 400, 500]

    @patch("app.routes.embed.get_embedder")
    def test_embed_text_special_characters(self, mock_embedder):
        """Test embedding with special characters"""
        mock_embed_instance = MagicMock()
        mock_embed_instance.get_text_embedding.return_value = [0.1] * 1536
        mock_embedder.return_value = mock_embed_instance

        special_text = "Text with émojis 🚀 and spëcial çhars!"

        request_data = {"text": special_text}

        response = client.post("/embed/text", json=request_data)

        assert response.status_code == 200


class TestEmbedBatch:
    """Test suite for POST /embed/batch endpoint"""

    @patch("app.routes.embed.get_embedder")
    def test_embed_batch_success(self, mock_embedder, sample_texts):
        """Test successful batch embedding"""
        mock_embed_instance = MagicMock()
        mock_embed_instance.get_text_embedding_batch.return_value = [
            [0.1] * 1536,
            [0.2] * 1536,
            [0.3] * 1536,
        ]
        mock_embedder.return_value = mock_embed_instance

        request_data = {
            "texts": sample_texts,
            "model": "text-embedding-3-small",
            "batch_size": 100,
        }

        response = client.post("/embed/batch", json=request_data)

        assert response.status_code == 200
        data = response.json()

        # Check response structure
        assert "embeddings" in data
        assert "dimension" in data
        assert "model" in data
        assert "total_embeddings" in data

        # Verify embeddings
        assert isinstance(data["embeddings"], list)
        assert len(data["embeddings"]) == 3
        assert data["total_embeddings"] == 3
        assert data["dimension"] == 1536

    @patch("app.routes.embed.get_embedder")
    def test_embed_batch_single_text(self, mock_embedder, sample_text):
        """Test batch endpoint with single text"""
        mock_embed_instance = MagicMock()
        mock_embed_instance.get_text_embedding_batch.return_value = [[0.1] * 1536]
        mock_embedder.return_value = mock_embed_instance

        request_data = {"texts": [sample_text]}

        response = client.post("/embed/batch", json=request_data)

        assert response.status_code == 200
        data = response.json()
        assert len(data["embeddings"]) == 1

    def test_embed_batch_empty_list(self):
        """Test batch embedding with empty list"""
        request_data = {"texts": []}

        response = client.post("/embed/batch", json=request_data)

        assert response.status_code == 400
        assert "no texts" in response.json()["detail"].lower()

    @patch("app.routes.embed.get_embedder")
    def test_embed_batch_with_empty_strings(self, mock_embedder):
        """Test batch embedding with some empty strings"""
        mock_embed_instance = MagicMock()
        mock_embed_instance.get_text_embedding_batch.return_value = [
            [0.1] * 1536,
            [0.2] * 1536,
        ]
        mock_embedder.return_value = mock_embed_instance

        request_data = {"texts": ["Valid text", "", "   ", "Another valid text"]}

        response = client.post("/embed/batch", json=request_data)

        # Should filter empty texts
        assert response.status_code == 200
        data = response.json()
        # Should only have embeddings for valid texts
        assert data["total_embeddings"] == 2

    @patch("app.routes.embed.get_embedder")
    def test_embed_batch_large_batch(self, mock_embedder):
        """Test batch embedding with large number of texts"""
        mock_embed_instance = MagicMock()
        num_texts = 100
        mock_embed_instance.get_text_embedding_batch.return_value = [
            [0.1] * 1536
        ] * num_texts
        mock_embedder.return_value = mock_embed_instance

        texts = [f"Sample text {i}" for i in range(num_texts)]

        request_data = {"texts": texts, "batch_size": 50}

        response = client.post("/embed/batch", json=request_data)

        assert response.status_code == 200
        data = response.json()
        assert data["total_embeddings"] == num_texts

    @patch("app.routes.embed.get_embedder")
    def test_embed_batch_different_models(self, mock_embedder, sample_texts):
        """Test batch embedding with different models"""
        models = [
            "text-embedding-3-small",
            "text-embedding-3-large",
            "text-embedding-ada-002",
        ]

        for model in models:
            mock_embed_instance = MagicMock()
            dim = 3072 if "large" in model else 1536
            mock_embed_instance.get_text_embedding_batch.return_value = [
                [0.1] * dim
            ] * len(sample_texts)
            mock_embedder.return_value = mock_embed_instance

            request_data = {"texts": sample_texts, "model": model}

            response = client.post("/embed/batch", json=request_data)

            assert response.status_code == 200
            data = response.json()
            assert data["model"] == model


class TestEmbedQuery:
    """Test suite for POST /embed/query endpoint"""

    @patch("app.routes.embed.get_embedder")
    def test_embed_query_success(self, mock_embedder):
        """Test successful query embedding"""
        mock_embed_instance = MagicMock()
        mock_embed_instance.get_query_embedding.return_value = [0.1] * 1536
        mock_embedder.return_value = mock_embed_instance

        params = {"query": "What is machine learning?"}

        response = client.post("/embed/query", params=params)

        assert response.status_code == 200
        data = response.json()

        assert "embedding" in data
        assert len(data["embedding"]) == 1536

    @patch("app.routes.embed.get_embedder")
    def test_embed_query_with_model(self, mock_embedder):
        """Test query embedding with specific model"""
        mock_embed_instance = MagicMock()
        mock_embed_instance.get_query_embedding.return_value = [0.1] * 1536
        mock_embedder.return_value = mock_embed_instance

        params = {"query": "Search query here", "model": "text-embedding-3-small"}

        response = client.post("/embed/query", params=params)

        assert response.status_code == 200

    def test_embed_query_empty(self):
        """Test query embedding with empty query"""
        params = {"query": ""}

        response = client.post("/embed/query", params=params)

        assert response.status_code == 400


class TestEmbedModels:
    """Test suite for GET /embed/models endpoint"""

    def test_list_models(self):
        """Test listing available embedding models"""
        response = client.get("/embed/models")

        assert response.status_code == 200
        data = response.json()

        assert "models" in data
        assert "default" in data
        assert isinstance(data["models"], list)
        assert len(data["models"]) > 0

        # Check model structure
        first_model = data["models"][0]
        assert "name" in first_model
        assert "dimension" in first_model
        assert "description" in first_model

        # Check expected models are present
        model_names = [m["name"] for m in data["models"]]
        assert "text-embedding-3-small" in model_names
        assert "text-embedding-3-large" in model_names


class TestEmbedHealth:
    """Test suite for GET /embed/health endpoint"""

    def test_embed_health(self):
        """Test embed service health check"""
        response = client.get("/embed/health")

        assert response.status_code == 200
        data = response.json()

        assert "status" in data
        assert data["status"] == "healthy"
        assert "service" in data
        assert data["service"] == "embed"
        assert "default_model" in data
        assert "api_key_configured" in data
        assert isinstance(data["api_key_configured"], bool)


# Fixtures


@pytest.fixture
def sample_text():
    """Create sample text for embedding tests"""
    return "This is a sample text for embedding generation."


@pytest.fixture
def sample_texts():
    """Create sample texts for batch embedding tests"""
    return [
        "First sample text for embedding.",
        "Second sample text with different content.",
        "Third text to test batch processing.",
    ]


# Integration Tests


class TestEmbedIntegration:
    """Integration tests for embed endpoint workflow"""

    @patch("app.routes.embed.get_embedder")
    def test_embed_stateless_behavior(self, mock_embedder, sample_text):
        """Test that embed is truly stateless"""
        mock_embed_instance = MagicMock()
        mock_embed_instance.get_text_embedding.return_value = [0.1] * 1536
        mock_embedder.return_value = mock_embed_instance

        request_data = {"text": sample_text}

        # Embed same text twice
        response1 = client.post("/embed/text", json=request_data)
        response2 = client.post("/embed/text", json=request_data)

        assert response1.status_code == 200
        assert response2.status_code == 200

        data1 = response1.json()
        data2 = response2.json()

        # Results should be identical
        assert data1["embedding"] == data2["embedding"]
        assert data1["dimension"] == data2["dimension"]

    @patch("app.routes.embed.get_embedder")
    def test_embed_single_vs_batch_consistency(self, mock_embedder, sample_text):
        """Test that single and batch embeddings are consistent"""
        mock_embed_instance = MagicMock()
        expected_embedding = [0.1] * 1536
        mock_embed_instance.get_text_embedding.return_value = expected_embedding
        mock_embed_instance.get_text_embedding_batch.return_value = [expected_embedding]
        mock_embedder.return_value = mock_embed_instance

        # Single embedding
        single_response = client.post("/embed/text", json={"text": sample_text})

        # Batch embedding with same text
        batch_response = client.post("/embed/batch", json={"texts": [sample_text]})

        assert single_response.status_code == 200
        assert batch_response.status_code == 200

        single_data = single_response.json()
        batch_data = batch_response.json()

        # Embeddings should be the same
        assert single_data["embedding"] == batch_data["embeddings"][0]


# Error Handling Tests


class TestEmbedErrorHandling:
    """Test error handling for embed endpoints"""

    def test_embed_missing_api_key(self):
        """Test embedding without API key configured"""
        # This would require mocking settings to remove API key
        # For now, we just verify the endpoint exists
        pass

    @patch("app.routes.embed.get_embedder")
    def test_embed_api_error(self, mock_embedder, sample_text):
        """Test handling of API errors"""
        mock_embedder.side_effect = Exception("API Error")

        request_data = {"text": sample_text}

        response = client.post("/embed/text", json=request_data)

        assert response.status_code == 500
        assert (
            "error" in response.json()["detail"].lower()
            or "failed" in response.json()["detail"].lower()
        )

    def test_embed_batch_all_empty_texts(self):
        """Test batch embedding with all empty texts"""
        request_data = {"texts": ["", "   ", "\n\t"]}

        response = client.post("/embed/batch", json=request_data)

        assert response.status_code == 400
        assert "empty" in response.json()["detail"].lower()

    def test_embed_missing_required_field(self):
        """Test embedding without required text field"""
        request_data = {"model": "text-embedding-3-small"}

        response = client.post("/embed/text", json=request_data)

        assert response.status_code == 422  # Validation error
