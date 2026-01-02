"""
Application configuration and settings
"""

import os
from functools import lru_cache
from typing import Optional

from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    """Application settings loaded from environment variables"""

    # Server settings
    app_name: str = "Document Processing Service"
    app_version: str = "0.1.0"
    debug: bool = False
    host: str = "0.0.0.0"
    port: int = 8000

    # CORS settings
    cors_origins: list[str] = ["http://localhost:8080", "http://localhost:5173"]

    # File storage
    upload_dir: str = "./uploads"
    max_file_size_mb: int = 50

    # Chunking settings
    chunk_size: int = 512
    chunk_overlap: int = 50

    # PDF processing settings
    max_pdf_pages: int = 30  # Limit pages to process (0 = no limit)

    # Vector DB settings
    vector_db_type: str = "chroma"  # chroma, pinecone, qdrant, weaviate

    # ChromaDB specific (client-server mode)
    chroma_host: str = "localhost"
    chroma_port: int = 8001
    chroma_collection_name: str = "documents"

    # Embedding settings
    embedding_model: str = "sentence-transformers/all-MiniLM-L6-v2"

    # LLM settings for metadata extraction
    llm_provider: str = "lmstudio"  # "lmstudio" (local) or "openai"
    # NOTE: For LM Studio, use "gpt-3.5-turbo" (LM Studio ignores this and uses loaded model)
    # LlamaIndex validates model names, so we must use a known OpenAI model name
    llm_model: str = "gpt-3.5-turbo"  
    lmstudio_base_url: str = "http://localhost:1234/v1"  # LM Studio default
    
    # OpenAI (required only if llm_provider="openai")
    openai_api_key: Optional[str] = None

    # Go backend integration
    go_backend_url: str = "http://localhost:8080"

    class Config:
        env_file = ".env"
        env_file_encoding = "utf-8"
        extra = "ignore"

    # Redis settings
    redis_host: str = "localhost"
    redis_port: int = 6379
    redis_db: int = 0
    redis_password: Optional[str] = None


@lru_cache()
def get_settings() -> Settings:
    """Get cached settings instance"""
    return Settings()


# Create upload directory if it doesn't exist
settings = get_settings()
os.makedirs(settings.upload_dir, exist_ok=True)
