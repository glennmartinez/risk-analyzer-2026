"""
Pytest configuration and shared fixtures for Python backend tests.
"""

import os
import sys
from pathlib import Path

import pytest

# Add app to path for imports
sys.path.insert(0, str(Path(__file__).parent.parent))


# Test Configuration
@pytest.fixture(scope="session", autouse=True)
def setup_test_env():
    """Setup test environment variables"""
    os.environ["TESTING"] = "true"
    os.environ["UPLOAD_DIR"] = "/tmp/test_uploads"
    os.environ["CHROMA_HOST"] = "localhost"
    os.environ["CHROMA_PORT"] = "8001"
    os.environ["REDIS_HOST"] = "localhost"
    os.environ["REDIS_PORT"] = "6379"
    os.environ["LLM_PROVIDER"] = "lmstudio"
    os.environ["LMSTUDIO_BASE_URL"] = "http://localhost:1234/v1"

    # Mock API keys for testing (not real keys)
    os.environ["OPENAI_API_KEY"] = "test-key-not-real"

    yield

    # Cleanup
    if "TESTING" in os.environ:
        del os.environ["TESTING"]


# Sample Data Fixtures
@pytest.fixture
def sample_text():
    """Create sample text for testing"""
    return """
    This is a sample document for testing text processing functionality.
    It contains multiple sentences spread across several paragraphs.

    The first paragraph introduces the topic and provides context.
    This helps ensure we have enough text to create meaningful chunks.

    The second paragraph continues the discussion with additional details.
    Each sentence adds more information to the overall document content.
    We want to test various processing strategies on realistic text.

    The third paragraph concludes our sample text.
    It demonstrates how text processing works across different sections.
    This allows us to verify that processing preserves semantic meaning.
    """


@pytest.fixture
def sample_texts():
    """Create multiple sample texts for batch testing"""
    return [
        "First sample text for batch processing tests.",
        "Second sample text with different content and structure.",
        "Third text to verify batch operations work correctly.",
        "Fourth sample demonstrating variety in test data.",
        "Fifth and final text for comprehensive batch testing.",
    ]


@pytest.fixture
def sample_short_text():
    """Create short sample text"""
    return "This is a short sample text."


@pytest.fixture
def sample_long_text():
    """Create long sample text for stress testing"""
    return " ".join(
        [f"This is sentence number {i} in a very long document." for i in range(1000)]
    )


@pytest.fixture
def sample_markdown_text():
    """Create sample markdown text"""
    return """
# Main Title

This is the introduction paragraph with some context.

## Section 1: Overview

Content for the first section goes here.
It has multiple sentences explaining the topic.

### Subsection 1.1: Details

More detailed information in this subsection.
Each section builds on the previous one.

### Subsection 1.2: Examples

Here are some examples:
- Example 1
- Example 2
- Example 3

## Section 2: Advanced Topics

The second section covers more advanced material.
It references concepts from earlier sections.

### Subsection 2.1: Technical Details

Technical implementation details are provided here.

## Conclusion

Final thoughts and summary of the document.
"""


@pytest.fixture
def sample_pdf_bytes():
    """Create a minimal valid PDF for testing"""
    # This is a minimal PDF that contains text "Test PDF Content"
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
    """Create sample text file bytes"""
    return b"This is a sample text file for testing.\nIt has multiple lines.\nAnd various content.\n"


@pytest.fixture
def sample_json_metadata():
    """Create sample metadata dictionary"""
    return {
        "filename": "test_document.pdf",
        "file_type": "pdf",
        "page_count": 5,
        "title": "Test Document",
        "author": "Test Author",
        "file_size_bytes": 12345,
        "extraction_method": "docling",
    }


# Mock Fixtures
@pytest.fixture
def mock_embedding_response():
    """Create mock embedding response"""
    return [0.1, 0.2, 0.3, 0.4, 0.5] + [0.0] * 1531  # 1536 dimensions


@pytest.fixture
def mock_embeddings_batch():
    """Create mock batch embeddings response"""
    return [
        [0.1] * 1536,
        [0.2] * 1536,
        [0.3] * 1536,
    ]


# Test Markers
def pytest_configure(config):
    """Configure custom pytest markers"""
    config.addinivalue_line(
        "markers", "slow: marks tests as slow (deselect with '-m \"not slow\"')"
    )
    config.addinivalue_line("markers", "integration: marks tests as integration tests")
    config.addinivalue_line("markers", "unit: marks tests as unit tests")
    config.addinivalue_line("markers", "api: marks tests that require external APIs")
    config.addinivalue_line(
        "markers", "requires_llm: marks tests that require LLM access"
    )
    config.addinivalue_line(
        "markers", "requires_openai: marks tests that require OpenAI API"
    )


# Pytest collection hooks
def pytest_collection_modifyitems(config, items):
    """Auto-mark tests based on their path/name"""
    for item in items:
        # Auto-mark integration tests
        if "integration" in item.nodeid:
            item.add_marker(pytest.mark.integration)

        # Auto-mark unit tests
        if "test_" in item.nodeid and "integration" not in item.nodeid:
            item.add_marker(pytest.mark.unit)

        # Auto-mark slow tests
        if "slow" in item.nodeid or "large" in item.nodeid:
            item.add_marker(pytest.mark.slow)

        # Auto-mark API tests
        if any(keyword in item.nodeid for keyword in ["embed", "llm", "openai"]):
            item.add_marker(pytest.mark.api)


# Helper Functions
@pytest.fixture
def create_temp_file(tmp_path):
    """Factory fixture to create temporary files"""

    def _create_file(filename, content):
        file_path = tmp_path / filename
        file_path.write_bytes(
            content if isinstance(content, bytes) else content.encode()
        )
        return file_path

    return _create_file


@pytest.fixture
def cleanup_temp_files():
    """Cleanup fixture for temporary files"""
    temp_files = []

    def register(file_path):
        temp_files.append(file_path)

    yield register

    # Cleanup
    for file_path in temp_files:
        if os.path.exists(file_path):
            os.remove(file_path)
