"""
Application configuration and settings
"""

import os
from typing import Optional
from pydantic_settings import BaseSettings
from functools import lru_cache


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
    
    # Vector DB settings (for future integration)
    vector_db_type: str = "chroma"  # chroma, pinecone, qdrant, weaviate
    vector_db_host: Optional[str] = "localhost"
    vector_db_port: Optional[int] = 8001
    
    # ChromaDB specific
    chroma_persist_dir: str = "./chroma_db"
    chroma_collection_name: str = "documents"
    
    # Embedding settings
    embedding_model: str = "sentence-transformers/all-MiniLM-L6-v2"
    
    # OpenAI (optional, for better embeddings)
    openai_api_key: Optional[str] = None
    
    # Go backend integration
    go_backend_url: str = "http://localhost:8080"
    
    class Config:
        env_file = ".env"
        env_file_encoding = "utf-8"
        extra = "ignore"


@lru_cache()
def get_settings() -> Settings:
    """Get cached settings instance"""
    return Settings()


# Create upload directory if it doesn't exist
settings = get_settings()
os.makedirs(settings.upload_dir, exist_ok=True)
os.makedirs(settings.chroma_persist_dir, exist_ok=True)
