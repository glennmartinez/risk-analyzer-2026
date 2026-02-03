import logging
import os
import time
import uuid
from typing import Any, Dict, Optional

import requests
from fastapi import APIRouter, BackgroundTasks, HTTPException
from pydantic import BaseModel, Field

router = APIRouter()
logger = logging.getLogger("process_with_callback")
logger.setLevel(logging.INFO)


class DocumentCallbackPayload(BaseModel):
    document_id: str = Field(..., alias="document_id")
    callback_url: str = Field(..., alias="callback_url")
    status: Optional[str] = Field("processing", alias="status")
    message: Optional[str] = Field(None, alias="message")
    job: Optional[Dict[str, Any]] = Field(None, alias="job")


class ProcessAck(BaseModel):
    job_id: str
    status: str


@router.post("/process-with-callback", response_model=ProcessAck)
async def process_with_callback(
    payload: DocumentCallbackPayload, background_tasks: BackgroundTasks
):
    """
    Accept a job payload from the Go service, return an immediate acknowledgement
    with a python-side job id, and perform the (simulated) work in the background.

    The background task will POST the result to payload.callback_url after a delay.
    """
    # Basic validation for callback_url
    if not payload.callback_url or not isinstance(payload.callback_url, str):
        raise HTTPException(status_code=400, detail="callback_url is required")

    py_job_id = str(uuid.uuid4())
    ack = ProcessAck(job_id=py_job_id, status="accepted")

    # Kick off the background work that will call back when finished
    background_tasks.add_task(
        do_work_and_callback, py_job_id, payload.dict(by_alias=True)
    )

    logger.info(
        "Accepted job from Go: python_job_id=%s, document_id=%s",
        py_job_id,
        payload.document_id,
    )
    return ack


def do_work_and_callback(py_job_id: str, payload: Dict[str, Any]):
    """
    Simulated long-running work. Sleeps for 10 seconds then POSTs a completion payload
    to the callback_url supplied in the payload.

    This function uses requests synchronously because it's executed in FastAPI's
    BackgroundTasks worker thread.
    """
    try:
        logger.info("Background work starting for python_job_id=%s", py_job_id)

        # Simulate processing time
        time.sleep(10)

        callback_url = payload.get("callback_url")
        if not callback_url:
            logger.error(
                "No callback_url provided in payload for python_job_id=%s: %s",
                py_job_id,
                payload,
            )
            return

        # If callback_url is a relative path (starts with '/'), prepend base URL from env
        if isinstance(callback_url, str) and callback_url.startswith("/"):
            base = os.getenv("GO_SERVER_BASE_URL", "http://localhost:8080")
            callback_url = base.rstrip("/") + callback_url

        # Build callback payload — adjust fields to match what Go expects
        callback_payload = {
            "document_id": payload.get("document_id"),
            "python_job_id": py_job_id,
            "status": "completed",
            "message": "Processing finished (simulated)",
            "job": payload.get("job"),
            # Optionally include processed data (empty/dummy for now)
            "chunks": [],
            "metadata": {},
        }

        headers = {"Content-Type": "application/json"}
        try:
            logger.info(
                "Posting callback for python_job_id=%s to %s", py_job_id, callback_url
            )
            resp = requests.post(
                callback_url, json=callback_payload, headers=headers, timeout=15
            )
            resp.raise_for_status()
            logger.info(
                "Successfully posted result to callback URL %s (python_job_id=%s)",
                callback_url,
                py_job_id,
            )
        except requests.RequestException as e:
            # In production you'd want retry/backoff logic or enqueue this failure
            # Use %s for string formatting and include the exception text (avoid invalid format verbs)
            logger.exception(
                "Failed to POST callback to %s for python_job_id=%s: %s",
                callback_url,
                py_job_id,
                e,
            )

    except Exception as e:
        logger.exception(
            "Background processing failed for python_job_id=%s: %v", py_job_id, e
        )
