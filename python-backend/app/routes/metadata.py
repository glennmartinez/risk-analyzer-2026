"""
Pure Metadata Endpoint - Stateless Metadata Extraction
This endpoint extracts metadata from text using LLM.
No persistence, no side effects - pure computation.
"""

import logging
from typing import List, Optional

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel, Field

from ..services.chunker import DocumentChunker

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/metadata", tags=["compute"])


class MetadataRequest(BaseModel):
    """Request model for metadata extraction"""

    text: str = Field(..., description="Text to extract metadata from")
    extract_title: bool = Field(True, description="Extract document title")
    extract_keywords: bool = Field(True, description="Extract keywords")
    extract_questions: bool = Field(True, description="Extract questions answered")
    num_questions: int = Field(3, description="Number of questions to generate")
    num_keywords: int = Field(5, description="Number of keywords to extract")


class TitleRequest(BaseModel):
    """Request model for title extraction"""

    text: str = Field(..., description="Text to extract title from")


class KeywordsRequest(BaseModel):
    """Request model for keywords extraction"""

    text: str = Field(..., description="Text to extract keywords from")
    num_keywords: int = Field(5, description="Number of keywords to extract")


class QuestionsRequest(BaseModel):
    """Request model for questions extraction"""

    text: str = Field(..., description="Text to extract questions from")
    num_questions: int = Field(3, description="Number of questions to generate")


class MetadataResponse(BaseModel):
    """Response model for metadata extraction"""

    title: Optional[str] = Field(None, description="Extracted title")
    keywords: Optional[List[str]] = Field(None, description="Extracted keywords")
    questions: Optional[List[str]] = Field(
        None, description="Questions this text answers"
    )
    metadata: dict = Field(
        default_factory=dict, description="Additional extracted metadata"
    )


# Initialize chunker (has LLM for metadata extraction)
chunker = DocumentChunker()


@router.post("/extract", response_model=MetadataResponse)
async def extract_metadata(request: MetadataRequest) -> MetadataResponse:
    """
    Extract metadata from text using LLM.

    This is a pure computation endpoint - no persistence.

    Args:
        request: MetadataRequest with text and extraction options

    Returns:
        MetadataResponse with extracted metadata

    Raises:
        HTTPException: If metadata extraction fails
    """
    try:
        logger.info(
            f"Extracting metadata from text: {len(request.text)} chars, "
            f"title={request.extract_title}, "
            f"keywords={request.extract_keywords}, "
            f"questions={request.extract_questions}"
        )

        # Validate text
        if not request.text or not request.text.strip():
            raise HTTPException(status_code=400, detail="Empty text provided")

        # Prepare response
        response = MetadataResponse()

        # Extract title
        if request.extract_title:
            try:
                title_extractor = chunker._get_title_extractor()
                # Create a simple node-like object for extraction
                from llama_index.core.schema import TextNode

                node = TextNode(text=request.text[:5000])  # Limit to first 5000 chars
                result = title_extractor.extract([node])
                if result and len(result) > 0 and result[0].metadata.get("title"):
                    response.title = result[0].metadata["title"]
                    logger.info(f"Extracted title: {response.title}")
            except Exception as e:
                logger.warning(f"Failed to extract title: {e}")
                response.title = None

        # Extract keywords
        if request.extract_keywords:
            try:
                keyword_extractor = chunker._get_keyword_extractor(
                    num_keywords=request.num_keywords
                )
                from llama_index.core.schema import TextNode

                node = TextNode(text=request.text[:5000])  # Limit to first 5000 chars
                result = keyword_extractor.extract([node])
                if result and len(result) > 0 and result[0].metadata.get("keywords"):
                    response.keywords = result[0].metadata["keywords"]
                    logger.info(f"Extracted {len(response.keywords)} keywords")
            except Exception as e:
                logger.warning(f"Failed to extract keywords: {e}")
                response.keywords = None

        # Extract questions
        if request.extract_questions:
            try:
                questions_extractor = chunker._get_questions_extractor(
                    num_questions=request.num_questions
                )
                from llama_index.core.schema import TextNode

                node = TextNode(text=request.text[:5000])  # Limit to first 5000 chars
                result = questions_extractor.extract([node])
                if result and len(result) > 0 and result[0].metadata.get("questions"):
                    response.questions = result[0].metadata["questions"]
                    logger.info(f"Extracted {len(response.questions)} questions")
            except Exception as e:
                logger.warning(f"Failed to extract questions: {e}")
                response.questions = None

        # Add any additional metadata
        response.metadata = {
            "text_length": len(request.text),
            "extraction_successful": any(
                [response.title, response.keywords, response.questions]
            ),
        }

        logger.info(
            f"Successfully extracted metadata: "
            f"title={bool(response.title)}, "
            f"keywords={len(response.keywords) if response.keywords else 0}, "
            f"questions={len(response.questions) if response.questions else 0}"
        )

        return response

    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error extracting metadata: {e}", exc_info=True)
        raise HTTPException(
            status_code=500, detail=f"Failed to extract metadata: {str(e)}"
        )


@router.post("/title", response_model=dict)
async def extract_title_only(request: TitleRequest) -> dict:
    """
    Extract only the title from text.

    Faster endpoint when you only need the title.

    Args:
        request: TitleRequest with text

    Returns:
        Dictionary with title
    """
    try:
        logger.info(f"Extracting title from text: {len(request.text)} chars")

        # Validate text
        if not request.text or not request.text.strip():
            raise HTTPException(status_code=400, detail="Empty text provided")

        # Extract title using chunker's LLM
        title = None
        try:
            title_extractor = chunker._get_title_extractor()
            from llama_index.core.schema import TextNode

            node = TextNode(text=request.text[:5000])  # Limit to first 5000 chars
            result = title_extractor.extract([node])
            if result and len(result) > 0 and result[0].metadata.get("title"):
                title = result[0].metadata["title"]
        except Exception as e:
            logger.warning(f"Failed to extract title: {e}")

        logger.info(f"Extracted title: {title}")

        return {"title": title, "text_length": len(text)}

    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error extracting title: {e}", exc_info=True)
        raise HTTPException(
            status_code=500, detail=f"Failed to extract title: {str(e)}"
        )


@router.post("/keywords", response_model=dict)
async def extract_keywords_only(request: KeywordsRequest) -> dict:
    """
    Extract only keywords from text.

    Faster endpoint when you only need keywords.

    Args:
        request: KeywordsRequest with text and num_keywords

    Returns:
        Dictionary with keywords
    """
    try:
        logger.info(
            f"Extracting {request.num_keywords} keywords from text: {len(request.text)} chars"
        )

        # Validate text
        if not request.text or not request.text.strip():
            raise HTTPException(status_code=400, detail="Empty text provided")

        # Extract keywords using chunker's LLM
        keywords = None
        try:
            keyword_extractor = chunker._get_keyword_extractor()
            from llama_index.core.schema import TextNode

            node = TextNode(text=request.text[:5000])  # Limit to first 5000 chars
            result = keyword_extractor.extract([node])
            if (
                result
                and len(result) > 0
                and result[0].metadata.get("excerpt_keywords")
            ):
                keywords = result[0].metadata["excerpt_keywords"].split(", ")
                keywords = keywords[: request.num_keywords]  # Limit to requested number
        except Exception as e:
            logger.warning(f"Failed to extract keywords: {e}")

        logger.info(f"Extracted {len(keywords) if keywords else 0} keywords")

        return {
            "keywords": keywords or [],
            "count": len(keywords) if keywords else 0,
            "text_length": len(text),
        }

    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error extracting keywords: {e}", exc_info=True)
        raise HTTPException(
            status_code=500, detail=f"Failed to extract keywords: {str(e)}"
        )


@router.post("/questions", response_model=dict)
async def extract_questions_only(request: QuestionsRequest) -> dict:
    """
    Extract only questions from text.

    Faster endpoint when you only need questions.

    Args:
        request: QuestionsRequest with text and num_questions

    Returns:
        Dictionary with questions
    """
    try:
        logger.info(
            f"Extracting {request.num_questions} questions from text: {len(request.text)} chars"
        )

        # Validate text
        if not request.text or not request.text.strip():
            raise HTTPException(status_code=400, detail="Empty text provided")

        # Extract questions using chunker's LLM
        questions = None
        try:
            questions_extractor = chunker._get_questions_extractor()
            from llama_index.core.schema import TextNode

            node = TextNode(text=request.text[:5000])  # Limit to first 5000 chars
            result = questions_extractor.extract([node])
            if (
                result
                and len(result) > 0
                and result[0].metadata.get("questions_this_excerpt_can_answer")
            ):
                questions_str = result[0].metadata["questions_this_excerpt_can_answer"]
                # Parse questions (usually newline-separated)
                questions = [q.strip() for q in questions_str.split("\n") if q.strip()]
                questions = questions[
                    : request.num_questions
                ]  # Limit to requested number
        except Exception as e:
            logger.warning(f"Failed to extract questions: {e}")

        logger.info(f"Extracted {len(questions) if questions else 0} questions")

        return {
            "questions": questions or [],
            "count": len(questions) if questions else 0,
            "text_length": len(request.text),
        }

    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error extracting questions: {e}", exc_info=True)
        raise HTTPException(
            status_code=500, detail=f"Failed to extract questions: {str(e)}"
        )


@router.get("/health")
async def metadata_health():
    """Health check for metadata service"""
    return {
        "status": "healthy",
        "service": "metadata",
        "capabilities": ["title", "keywords", "questions"],
        "llm_configured": chunker._llm is not None
        or chunker.settings.llm_provider is not None,
    }
