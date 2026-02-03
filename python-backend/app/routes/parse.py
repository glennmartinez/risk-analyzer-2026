"""
Parse Endpoint - supports both synchronous responses and async callback-based processing.

Behavior:
- If `callback_url` is provided (form field), the endpoint will:
  1) Read the uploaded file into memory
  2) Generate or accept a `python_job_id` and immediately return {"python_job_id": "...", "status": "accepted"}
  3) In the background, POST a 'processing' callback to `callback_url` and then run the parse.
  4) When parse completes or fails, POST a final callback with the result envelope.

- If `callback_url` is not provided, the endpoint will behave synchronously and return the parsed result directly
  (same behavior as before).

Notes:
- The background tasks use `httpx` to POST JSON to the callback_url.
- The callback envelope includes optional `job`/`go_job_id` fields if provided by the caller, and always includes
  `python_job_id`, `job_type`, `status`, `message`, `result` (when available) and `timestamp`.
"""

import asyncio
import json
import logging
import uuid
from datetime import datetime
from typing import Any, Optional

import httpx
from fastapi import (
    APIRouter,
    BackgroundTasks,
    File,
    Form,
    HTTPException,
    Request,
    UploadFile,
)
from pydantic import BaseModel, Field

from ..config import get_settings
from ..services.parser import DocumentParser

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/parse", tags=["compute"])


class ParseRequest(BaseModel):
    """Request model for parsing (when using URL or existing file)"""

    file_path: Optional[str] = Field(
        None, description="Path to file on server (for testing)"
    )
    extract_metadata: bool = Field(
        True, description="Whether to extract document metadata"
    )
    max_pages: int = Field(0, description="Maximum pages to process (0 = all pages)")


class ParseResponse(BaseModel):
    """Response model for document parsing"""

    text: str = Field(..., description="Extracted text content")
    markdown: Optional[str] = Field(None, description="Content as markdown")
    metadata: dict = Field(default_factory=dict, description="Document metadata")
    pages: list = Field(default_factory=list, description="Page-level information")
    tables: list = Field(default_factory=list, description="Extracted tables")
    figures: list = Field(default_factory=list, description="Extracted figures")
    extraction_method: str = Field(
        ..., description="Extraction method used (docling/pymupdf)"
    )


# Initialize parser
parser = DocumentParser()


def _build_parse_response(parsed_doc, extract_metadata: bool) -> ParseResponse:
    return ParseResponse(
        text=parsed_doc.raw_text,
        markdown=parsed_doc.markdown_text,
        metadata={
            "filename": parsed_doc.metadata.filename,
            "file_type": parsed_doc.metadata.file_type,
            "page_count": parsed_doc.metadata.page_count,
            "title": parsed_doc.metadata.title,
            "author": parsed_doc.metadata.author,
            "file_size_bytes": parsed_doc.metadata.file_size_bytes,
            "extraction_method": parsed_doc.metadata.extraction_method,
        }
        if extract_metadata
        else {},
        pages=parsed_doc.pages if extract_metadata else [],
        tables=parsed_doc.tables if extract_metadata else [],
        figures=parsed_doc.figures if extract_metadata else [],
        extraction_method=parsed_doc.metadata.extraction_method,
    )


async def _post_json(url: str, payload: dict, timeout: float = 15.0):
    """Helper to POST JSON to a callback URL with simple error handling."""
    try:
        async with httpx.AsyncClient(timeout=timeout) as client:
            r = await client.post(url, json=payload)
            r.raise_for_status()
            logger.info("Posted callback to %s (status=%s)", url, r.status_code)
            return True
    except Exception as e:
        logger.exception("Failed to POST callback to %s: %s", url, e)
        return False


async def _run_parse_and_post_callbacks(
    *,
    file_bytes: bytes,
    filename: str,
    callback_url: str,
    python_job_id: str,
    go_job_id: Optional[str],
    extract_metadata: bool,
    max_pages: int,
):
    """Background worker that sends 'processing' and final callbacks to callback_url."""
    settings = get_settings()

    # Build base envelope data
    base_envelope: dict[str, Any] = {
        "python_job_id": python_job_id,
        "job_type": "document_parse",
        "timestamp": datetime.utcnow().isoformat() + "Z",
    }
    if go_job_id:
        # include a small job object for convenience
        base_envelope["job"] = {"id": go_job_id, "type": "document_parse"}
        base_envelope["go_job_id"] = go_job_id

    # 1) Send immediate 'processing' callback
    logger.info(
        "Background parse starting: python_job_id=%s go_job_id=%s filename=%s callback_url=%s",
        python_job_id,
        go_job_id,
        filename,
        callback_url,
    )
    processing_payload = {
        **base_envelope,
        "status": "processing",
        "message": "Parsing started",
    }
    await _post_json(callback_url, processing_payload)

    # 2) Run the parse
    original_max_pages = parser.settings.max_pdf_pages
    try:
        if max_pages and max_pages > 0:
            parser.settings.max_pdf_pages = max_pages

        # Run parsing in a threadpool (blocking parse implementation)
        loop = asyncio.get_running_loop()
        parsed_doc = await loop.run_in_executor(
            None,
            lambda: parser.parse_bytes(
                file_bytes=file_bytes, filename=filename, upload_dir=settings.upload_dir
            ),
        )

        # Build result
        parse_response = _build_parse_response(parsed_doc, extract_metadata)
        result_payload = {
            **base_envelope,
            "status": "completed",
            "message": "Parsing completed successfully",
            "result": {"parse_response": json.loads(parse_response.json())},
            "timestamp": datetime.utcnow().isoformat() + "Z",
        }

        # Optionally include chunk manifest/other info if parser produced them
        # (left as a placeholder for future improvements)
        # if parsed_doc.chunk_manifest_url:
        #     result_payload["result"]["chunk_manifest_url"] = parsed_doc.chunk_manifest_url

        await _post_json(callback_url, result_payload)
        logger.info(
            "Background parse completed: python_job_id=%s go_job_id=%s filename=%s status=completed",
            python_job_id,
            go_job_id,
            filename,
        )

    except Exception as e:
        logger.exception("Parsing failed for %s: %s", filename, e)
        failure_payload = {
            **base_envelope,
            "status": "failed",
            "message": f"Parsing failed: {str(e)}",
            "timestamp": datetime.utcnow().isoformat() + "Z",
        }
        await _post_json(callback_url, failure_payload)
        logger.info(
            "Background parse failed: python_job_id=%s go_job_id=%s filename=%s",
            python_job_id,
            go_job_id,
            filename,
        )
    finally:
        parser.settings.max_pdf_pages = original_max_pages


@router.post("/document")
async def parse_document(
    request: Request,
    background_tasks: BackgroundTasks,
    file: UploadFile = File(...),
    extract_metadata: bool = Form(default=True),
    max_pages: int = Form(default=0),
    callback_url: Optional[str] = Form(default=None),
    python_job_id: Optional[str] = Form(default=None),
    go_job_id: Optional[str] = Form(default=None),
):
    """
    Parse a document and extract text, tables, and figures.

    If `callback_url` is provided the endpoint will accept the job and return immediately
    with a `python_job_id`. The actual parsing will happen in background and callbacks
    will be posted to `callback_url` (a 'processing' callback followed by a final
    'completed' or 'failed' callback).

    If no `callback_url` is provided, this behaves as a synchronous parse and returns
    the ParseResponse.
    """
    try:
        # Log incoming request information (remote address, headers and form fields)
        client_host = request.client.host if request.client else None
        headers = dict(request.headers)
        logger.info(
            f"Received parse request from {client_host}: filename={file.filename} "
            f"(extract_metadata={extract_metadata}, max_pages={max_pages}, callback_url={callback_url}, python_job_id={python_job_id}, go_job_id={go_job_id})"
        )
        # Log a small subset of headers to avoid leaking secrets in logs; include common useful headers
        header_snapshot = {
            "host": headers.get("host"),
            "user-agent": headers.get("user-agent"),
            "content-type": headers.get("content-type"),
            "authorization_present": ("authorization" in headers),
        }
        logger.info(f"Parse request headers: {header_snapshot}")

        # Read file content up-front (we need bytes for background tasks)
        content = await file.read()
        if not content:
            raise HTTPException(status_code=400, detail="Empty file uploaded")

        # If caller requested callback-based processing, spin up background task and return quickly
        if callback_url:
            # Ensure we have a python_job_id
            if not python_job_id or python_job_id.strip() == "":
                python_job_id = str(uuid.uuid4())

            # Start background work
            # Use FastAPI BackgroundTasks to schedule; it will run after the response is sent.
            background_tasks.add_task(
                _run_parse_and_post_callbacks,
                file_bytes=content,
                filename=file.filename,
                callback_url=callback_url,
                python_job_id=python_job_id,
                go_job_id=go_job_id,
                extract_metadata=extract_metadata,
                max_pages=max_pages,
            )

            # Immediate acknowledgement so caller (Go server) can mark the job as 'processing'
            logger.info(
                "Accepted async parse request: python_job_id=%s go_job_id=%s file=%s callback_url=%s",
                python_job_id,
                go_job_id,
                file.filename,
                callback_url,
            )
            return {"python_job_id": python_job_id, "status": "accepted"}

        # Otherwise synchronous parse and return the ParseResponse
        settings = get_settings()

        original_max_pages = parser.settings.max_pdf_pages
        if max_pages > 0:
            parser.settings.max_pdf_pages = max_pages

        try:
            parsed_doc = parser.parse_bytes(
                file_bytes=content,
                filename=file.filename,
                upload_dir=settings.upload_dir,
            )

            response = _build_parse_response(parsed_doc, extract_metadata)

            logger.info(
                f"Successfully parsed {file.filename}: "
                f"{len(parsed_doc.raw_text)} chars, "
                f"{len(parsed_doc.tables)} tables, "
                f"{len(parsed_doc.figures)} figures"
            )

            # Return the pydantic model as dict
            return json.loads(response.json())
        finally:
            parser.settings.max_pdf_pages = original_max_pages

    except HTTPException:
        raise
    except Exception as e:
        logger.exception("Error parsing document %s: %s", file.filename, e)
        raise HTTPException(
            status_code=500, detail=f"Failed to parse document: {str(e)}"
        )


@router.post("/text")
async def parse_text_only(
    request: Request,
    background_tasks: BackgroundTasks,
    file: UploadFile = File(...),
    max_pages: int = Form(default=0),
    callback_url: Optional[str] = Form(default=None),
    python_job_id: Optional[str] = Form(default=None),
    go_job_id: Optional[str] = Form(default=None),
):
    """
    Parse a document and extract only text (no tables/figures).

    Supports the same callback-based flow as `/document`, but returns a minimal response
    in both synchronous and async callback envelopes.
    """
    try:
        # Log incoming request information for the text-only endpoint
        client_host = request.client.host if request.client else None
        headers = dict(request.headers)
        logger.info(
            f"Received text-only parse request from {client_host}: filename={file.filename} "
            f"(max_pages={max_pages}, callback_url={callback_url}, python_job_id={python_job_id}, go_job_id={go_job_id})"
        )
        header_snapshot = {
            "host": headers.get("host"),
            "user-agent": headers.get("user-agent"),
            "content-type": headers.get("content-type"),
            "authorization_present": ("authorization" in headers),
        }
        logger.info(f"Text-only parse request headers: {header_snapshot}")

        content = await file.read()
        if not content:
            raise HTTPException(status_code=400, detail="Empty file uploaded")

        # If callback_url provided, run in background and return accepted immediately
        if callback_url:
            if not python_job_id or python_job_id.strip() == "":
                python_job_id = str(uuid.uuid4())

            # We will reuse the same background runner but then send a smaller result
            async def _run_text_parse_and_post_callbacks(
                *,
                file_bytes,
                filename,
                callback_url,
                python_job_id,
                go_job_id,
                max_pages,
            ):
                settings = get_settings()
                base_envelope: dict[str, Any] = {
                    "python_job_id": python_job_id,
                    "job_type": "document_parse_text_only",
                    "timestamp": datetime.utcnow().isoformat() + "Z",
                }
                if go_job_id:
                    base_envelope["job"] = {
                        "id": go_job_id,
                        "type": "document_parse_text_only",
                    }
                    base_envelope["go_job_id"] = go_job_id

                logger.info(
                    "Background text-parse starting: python_job_id=%s go_job_id=%s filename=%s callback_url=%s",
                    python_job_id,
                    go_job_id,
                    filename,
                    callback_url,
                )
                processing_payload = {
                    **base_envelope,
                    "status": "processing",
                    "message": "Text parsing started",
                }
                await _post_json(callback_url, processing_payload)

                original_max_pages = parser.settings.max_pdf_pages
                try:
                    if max_pages and max_pages > 0:
                        parser.settings.max_pdf_pages = max_pages

                    loop = asyncio.get_running_loop()
                    parsed_doc = await loop.run_in_executor(
                        None,
                        lambda: parser.parse_bytes(
                            file_bytes=file_bytes,
                            filename=filename,
                            upload_dir=settings.upload_dir,
                        ),
                    )

                    parse_response = {
                        "text": parsed_doc.raw_text,
                        "markdown": None,
                        "metadata": {
                            "filename": parsed_doc.metadata.filename,
                            "extraction_method": parsed_doc.metadata.extraction_method,
                        },
                        "pages": [],
                        "tables": [],
                        "figures": [],
                        "extraction_method": parsed_doc.metadata.extraction_method,
                    }

                    result_payload = {
                        **base_envelope,
                        "status": "completed",
                        "message": "Text parsing completed",
                        "result": {"parse_response": parse_response},
                        "timestamp": datetime.utcnow().isoformat() + "Z",
                    }
                    await _post_json(callback_url, result_payload)
                    logger.info(
                        "Background text-parse completed: python_job_id=%s go_job_id=%s filename=%s status=completed",
                        python_job_id,
                        go_job_id,
                        filename,
                    )
                except Exception as e:
                    logger.exception("Text parsing failed for %s: %s", filename, e)
                    failure_payload = {
                        **base_envelope,
                        "status": "failed",
                        "message": f"Text parsing failed: {str(e)}",
                        "timestamp": datetime.utcnow().isoformat() + "Z",
                    }
                    await _post_json(callback_url, failure_payload)
                    logger.info(
                        "Background text-parse failed: python_job_id=%s go_job_id=%s filename=%s",
                        python_job_id,
                        go_job_id,
                        filename,
                    )
                finally:
                    parser.settings.max_pdf_pages = original_max_pages

            background_tasks.add_task(
                _run_text_parse_and_post_callbacks,
                file_bytes=content,
                filename=file.filename,
                callback_url=callback_url,
                python_job_id=python_job_id,
                go_job_id=go_job_id,
                max_pages=max_pages,
            )

            return {"python_job_id": python_job_id, "status": "accepted"}

        # Synchronous text-only parse
        settings = get_settings()
        original_max_pages = parser.settings.max_pdf_pages
        if max_pages > 0:
            parser.settings.max_pdf_pages = max_pages

        try:
            parsed_doc = parser.parse_bytes(
                file_bytes=content,
                filename=file.filename,
                upload_dir=settings.upload_dir,
            )

            response = {
                "text": parsed_doc.raw_text,
                "markdown": None,
                "metadata": {
                    "filename": parsed_doc.metadata.filename,
                    "extraction_method": parsed_doc.metadata.extraction_method,
                },
                "pages": [],
                "tables": [],
                "figures": [],
                "extraction_method": parsed_doc.metadata.extraction_method,
            }

            logger.info(
                f"Successfully parsed text from {file.filename}: "
                f"{len(parsed_doc.raw_text)} chars"
            )

            return response
        finally:
            parser.settings.max_pdf_pages = original_max_pages

    except HTTPException:
        raise
    except Exception as e:
        logger.exception("Error parsing text from %s: %s", file.filename, e)
        raise HTTPException(status_code=500, detail=f"Failed to parse text: {str(e)}")


@router.get("/health")
async def parse_health():
    """Health check for parse service"""
    return {
        "status": "healthy",
        "service": "parse",
        "supported_formats": parser.get_supported_formats(),
    }
