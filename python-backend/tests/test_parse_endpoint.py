"""
Tests for Parse Endpoint - Stateless Document Parsing
Tests the pure compute parse endpoints without any persistence.
"""

import io

import pytest
from fastapi.testclient import TestClient

from app.main import app

client = TestClient(app)


class TestParseDocument:
    """Test suite for POST /parse/document endpoint"""

    def test_parse_document_success(self, sample_pdf_bytes):
        """Test successful document parsing with full metadata"""
        files = {"file": ("test.pdf", sample_pdf_bytes, "application/pdf")}
        params = {"extract_metadata": True, "max_pages": 0}

        response = client.post("/parse/document", files=files, params=params)

        assert response.status_code == 200
        data = response.json()

        # Check response structure
        assert "text" in data
        assert "markdown" in data
        assert "metadata" in data
        assert "pages" in data
        assert "tables" in data
        assert "figures" in data
        assert "extraction_method" in data

        # Check text is not empty
        assert len(data["text"]) > 0
        assert data["extraction_method"] in ["docling", "pymupdf"]

    def test_parse_document_with_metadata(self, sample_pdf_bytes):
        """Test parsing with metadata extraction enabled"""
        files = {"file": ("test.pdf", sample_pdf_bytes, "application/pdf")}
        params = {"extract_metadata": True}

        response = client.post("/parse/document", files=files, params=params)

        assert response.status_code == 200
        data = response.json()

        # Metadata should be present
        assert data["metadata"] is not None
        assert isinstance(data["metadata"], dict)
        assert "filename" in data["metadata"]
        assert "file_type" in data["metadata"]
        assert "extraction_method" in data["metadata"]

    def test_parse_document_without_metadata(self, sample_pdf_bytes):
        """Test parsing without metadata extraction"""
        files = {"file": ("test.pdf", sample_pdf_bytes, "application/pdf")}
        params = {"extract_metadata": False}

        response = client.post("/parse/document", files=files, params=params)

        assert response.status_code == 200
        data = response.json()

        # Metadata should be empty or minimal
        assert data["metadata"] == {} or data["metadata"] is None
        assert data["pages"] == []
        assert data["tables"] == []
        assert data["figures"] == []

    def test_parse_document_with_page_limit(self, sample_pdf_bytes):
        """Test parsing with page limit"""
        files = {"file": ("test.pdf", sample_pdf_bytes, "application/pdf")}
        params = {"max_pages": 2}

        response = client.post("/parse/document", files=files, params=params)

        assert response.status_code == 200
        data = response.json()

        # Should only process first 2 pages
        if data["metadata"] and data["metadata"].get("page_count"):
            assert data["metadata"]["page_count"] <= 2

    def test_parse_document_empty_file(self):
        """Test parsing with empty file"""
        files = {"file": ("empty.pdf", b"", "application/pdf")}

        response = client.post("/parse/document", files=files)

        assert response.status_code == 400
        assert "empty file" in response.json()["detail"].lower()

    def test_parse_document_no_file(self):
        """Test endpoint without file upload"""
        response = client.post("/parse/document")

        assert response.status_code == 422  # Validation error

    def test_parse_document_different_formats(self, sample_text_file_bytes):
        """Test parsing different file formats"""
        # Test with text file
        files = {"file": ("test.txt", sample_text_file_bytes, "text/plain")}

        response = client.post("/parse/document", files=files)

        # Should handle gracefully (may succeed or fail depending on parser)
        assert response.status_code in [200, 500]


class TestParseTextOnly:
    """Test suite for POST /parse/text endpoint"""

    def test_parse_text_only_success(self, sample_pdf_bytes):
        """Test text-only parsing (faster variant)"""
        files = {"file": ("test.pdf", sample_pdf_bytes, "application/pdf")}

        response = client.post("/parse/text", files=files)

        assert response.status_code == 200
        data = response.json()

        # Should have text but minimal metadata
        assert "text" in data
        assert len(data["text"]) > 0
        assert data["tables"] == []
        assert data["figures"] == []
        assert data["pages"] == []

    def test_parse_text_only_with_page_limit(self, sample_pdf_bytes):
        """Test text-only parsing with page limit"""
        files = {"file": ("test.pdf", sample_pdf_bytes, "application/pdf")}
        params = {"max_pages": 1}

        response = client.post("/parse/text", files=files, params=params)

        assert response.status_code == 200
        data = response.json()

        assert "text" in data
        assert len(data["text"]) > 0

    def test_parse_text_only_empty_file(self):
        """Test text-only parsing with empty file"""
        files = {"file": ("empty.pdf", b"", "application/pdf")}

        response = client.post("/parse/text", files=files)

        assert response.status_code == 400


class TestParseHealth:
    """Test suite for GET /parse/health endpoint"""

    def test_parse_health(self):
        """Test parse service health check"""
        response = client.get("/parse/health")

        assert response.status_code == 200
        data = response.json()

        assert "status" in data
        assert data["status"] == "healthy"
        assert "service" in data
        assert data["service"] == "parse"
        assert "supported_formats" in data
        assert isinstance(data["supported_formats"], list)
        assert len(data["supported_formats"]) > 0


# Fixtures


@pytest.fixture
def sample_pdf_bytes():
    """Create a minimal valid PDF for testing"""
    # This is a minimal PDF that just contains text
    pdf_content = b"""%PDF-1.4
1 0 obj
<<
/Type /Catalog
/Pages 2 0 R
>>
endobj
2 0 obj
<<
/Type /Pages
/Kids [3 0 R]
/Count 1
>>
endobj
3 0 obj
<<
/Type /Page
/Parent 2 0 R
/MediaBox [0 0 612 792]
/Contents 4 0 R
/Resources <<
/Font <<
/F1 <<
/Type /Font
/Subtype /Type1
/BaseFont /Helvetica
>>
>>
>>
>>
endobj
4 0 obj
<<
/Length 44
>>
stream
BT
/F1 12 Tf
100 700 Td
(Test PDF Content) Tj
ET
endstream
endobj
xref
0 5
0000000000 65535 f
0000000009 00000 n
0000000058 00000 n
0000000115 00000 n
0000000317 00000 n
trailer
<<
/Size 5
/Root 1 0 R
>>
startxref
410
%%EOF"""
    return pdf_content


@pytest.fixture
def sample_text_file_bytes():
    """Create sample text file for testing"""
    return b"This is a sample text file for testing.\nIt has multiple lines.\n"


# Integration Tests


class TestParseIntegration:
    """Integration tests for parse endpoint workflow"""

    def test_parse_full_workflow(self, sample_pdf_bytes):
        """Test complete parse workflow"""
        # Step 1: Parse with full metadata
        files = {"file": ("test.pdf", sample_pdf_bytes, "application/pdf")}
        params = {"extract_metadata": True, "max_pages": 0}

        response = client.post("/parse/document", files=files, params=params)
        assert response.status_code == 200

        full_data = response.json()
        assert len(full_data["text"]) > 0

        # Step 2: Parse text only (should be faster)
        files = {"file": ("test.pdf", sample_pdf_bytes, "application/pdf")}

        response = client.post("/parse/text", files=files)
        assert response.status_code == 200

        text_only_data = response.json()

        # Text should be similar (may not be identical due to processing)
        assert len(text_only_data["text"]) > 0

    def test_parse_stateless_behavior(self, sample_pdf_bytes):
        """Test that parse is truly stateless (no side effects)"""
        files = {"file": ("test.pdf", sample_pdf_bytes, "application/pdf")}

        # Parse same file twice
        response1 = client.post("/parse/document", files=files)
        assert response1.status_code == 200

        # Create new file handle
        files = {"file": ("test.pdf", sample_pdf_bytes, "application/pdf")}
        response2 = client.post("/parse/document", files=files)
        assert response2.status_code == 200

        # Results should be consistent
        data1 = response1.json()
        data2 = response2.json()

        assert data1["text"] == data2["text"]
        assert data1["extraction_method"] == data2["extraction_method"]


# Error Handling Tests


class TestParseErrorHandling:
    """Test error handling for parse endpoints"""

    def test_parse_invalid_file_format(self):
        """Test parsing with invalid file format"""
        files = {
            "file": ("invalid.xyz", b"invalid content", "application/octet-stream")
        }

        response = client.post("/parse/document", files=files)

        # Should return error
        assert response.status_code in [400, 500]

    def test_parse_corrupted_pdf(self):
        """Test parsing with corrupted PDF"""
        files = {"file": ("corrupted.pdf", b"not a valid pdf", "application/pdf")}

        response = client.post("/parse/document", files=files)

        # Should handle gracefully
        assert response.status_code in [400, 500]

    def test_parse_very_large_page_limit(self, sample_pdf_bytes):
        """Test with unreasonably large page limit"""
        files = {"file": ("test.pdf", sample_pdf_bytes, "application/pdf")}
        params = {"max_pages": 999999}

        response = client.post("/parse/document", files=files, params=params)

        # Should handle gracefully (just process all pages)
        assert response.status_code == 200
