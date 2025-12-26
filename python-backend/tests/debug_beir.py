# python-backend/tests/debug_beir.py
import os
import sys

print("Script started")

try:
    # Add the parent directory of tests/ (i.e., python-backend/) to sys.path
    script_dir = os.path.dirname(__file__)  # tests/
    parent_dir = os.path.dirname(script_dir)  # python-backend/
    sys.path.insert(0, parent_dir)

    from app.config import get_settings

    print("Config import successful")

    settings = get_settings()
    print(f"Chroma host: {settings.chroma_host}, port: {settings.chroma_port}")

    import chromadb

    client = chromadb.HttpClient(host=settings.chroma_host, port=settings.chroma_port)
    print("Chroma connection successful")

    # Test BEIR import
    from beir import util

    print("BEIR import successful")

except Exception as e:
    print(f"Error: {e}")
    import traceback

    traceback.print_exc()
