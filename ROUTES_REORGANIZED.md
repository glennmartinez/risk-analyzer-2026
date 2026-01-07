# Routes Reorganized - Deprecated Folder

**Date**: 2024-01-06  
**Status**: ✅ Complete

---

## What Changed

Moved deprecated persistence-heavy routes to `app/routes/deprecated/` folder.

### Structure

```
app/routes/
├── __init__.py           # Imports from both folders
├── chunk.py              # ✅ New stateless endpoint
├── embed.py              # ✅ New stateless endpoint
├── metadata.py           # ✅ New stateless endpoint
├── parse.py              # ✅ New stateless endpoint
├── health.py             # Health checks
└── deprecated/           # 📦 Old routes (kept for compatibility)
    ├── __init__.py
    ├── documents.py      # Old document upload/management
    ├── search.py         # Old vector search
    └── rag.py            # Old RAG endpoints
```

---

## Routes Summary

**New Compute Endpoints** (19 routes):
- `/parse/*` - Document parsing
- `/chunk/*` - Text chunking
- `/embed/*` - Local embeddings
- `/metadata/*` - Metadata extraction

**Deprecated Endpoints** (22 routes):
- `/documents/*` - Old document management
- `/search/*` - Old vector search
- `/rag/*` - Old RAG operations

**Total**: 49 routes

---

## Import Changes

### Before
```python
from .documents import router as documents_router
from .search import router as search_router
from .rag import router as rag_router
```

### After
```python
from .deprecated import documents_router, rag_router, search_router
```

---

## Files Modified

1. Created `app/routes/deprecated/__init__.py`
2. Moved `documents.py` → `deprecated/documents.py` (fixed imports)
3. Moved `search.py` → `deprecated/search.py`
4. Moved `rag.py` → `deprecated/rag.py`
5. Updated `app/routes/__init__.py`

---

## Verification

```bash
✅ App loads successfully
✅ 19 new compute endpoints
✅ 22 deprecated endpoints
✅ Total: 49 routes
```

---

## Next Steps

Ready for **Phase 3 - Task 3.1**: Create Python client in Go

---

**Status**: ✅ Routes organized, app working