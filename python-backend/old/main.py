"""
FastAPI server for keyword extraction API
Provides REST endpoints compatible with the Go version
"""

from fastapi import FastAPI, HTTPException
from fastapi.responses import JSONResponse
from pydantic import BaseModel
from typing import List, Optional
import json
from simple_extractor import AdvancedKeywordExtractor, Issue, KeywordResult

app = FastAPI(title="Python Keyword Extraction API", version="1.0.0")

# Initialize extractor globally
extractor = None

@app.on_event("startup")
async def startup_event():
    global extractor
    extractor = AdvancedKeywordExtractor()

class IssueRequest(BaseModel):
    id: str
    title: str
    description: str
    issueType: str  # Match Go JSON field name
    components: List[str]

class KeywordResponse(BaseModel):
    word: str
    frequency: int
    score: float
    method: str
    pos_tag: str = ""
    confidence: float = 0.0

class IssueWithKeywords(BaseModel):
    id: str
    title: str
    description: str
    issueType: str  # Match Go format
    components: List[str]
    keywords: List[KeywordResponse]

@app.get("/")
async def root():
    return {"message": "Python Keyword Extraction API", "version": "1.0.0"}

@app.get("/health")
async def health_check():
    return {"status": "healthy", "extractor_loaded": extractor is not None}

@app.get("/issues/with-keywords")
async def get_issues_with_keywords(limit: Optional[int] = 10):
    """Get all issues with extracted keywords (same format as Go version)"""
    try:
        issues = load_issues_from_file()
        results = []
        
        for issue in issues:
            keywords = extractor.extract_keywords(issue, limit=limit)
            
            keyword_responses = [
                KeywordResponse(
                    word=kw.word,
                    frequency=kw.frequency,
                    score=kw.score,
                    method=kw.method,
                    pos_tag=kw.pos_tag,
                    confidence=kw.confidence
                ) for kw in keywords
            ]
            
            results.append(IssueWithKeywords(
                id=issue.id,
                title=issue.title,
                description=issue.description,
                issueType=issue.issue_type,
                components=issue.components,
                keywords=keyword_responses
            ))
        
        return results
        
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Extraction failed: {str(e)}")

@app.get("/issues/keywords-only")
async def get_keywords_only(limit: Optional[int] = 5):
    """Get only keywords for all issues (same format as Go version)"""
    try:
        issues = load_issues_from_file()
        result = {}
        
        for issue in issues:
            keywords = extractor.extract_keywords(issue, limit=limit)
            result[issue.id] = [kw.word for kw in keywords]
        
        return result
        
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Extraction failed: {str(e)}")

def load_issues_from_file():
    """Load issues from the same JSON file as Go version"""
    import json
    try:
        with open('../config/example_issues.json', 'r') as f:
            issues_data = json.load(f)
            return [
                Issue(
                    id=issue['id'],
                    title=issue['title'],
                    description=issue['description'],
                    issue_type=issue['issueType'],
                    components=issue['components']
                ) for issue in issues_data
            ]
    except FileNotFoundError:
        # Fallback to sample data
        return [
            Issue(
                id="ISSUE-123",
                title="Jenkins build fails with timeout error in production deployment",
                description="The Jenkins CI/CD pipeline is consistently failing during the production deployment phase. The error occurs after 30 minutes with a timeout exception. This issue affects the BigRedButton component and prevents automatic deployments to the China region. The build logs show memory allocation errors and network connectivity issues with the database connection pool.",
                issue_type="bug",
                components=["Jenkins", "BigRedButton", "China"]
            )
        ]

@app.get("/issues")
async def get_issues():
    """Get all issues (same as Go version)"""
    issues = load_issues_from_file()
    return [
        {
            "id": issue.id,
            "title": issue.title,
            "description": issue.description,
            "issueType": issue.issue_type,
            "components": issue.components
        } for issue in issues
    ]

@app.post("/extract-keywords")
async def extract_keywords_single(issue_request: IssueRequest, limit: Optional[int] = 10):
    """Extract keywords from a single issue"""
    try:
        issue = Issue(
            id=issue_request.id,
            title=issue_request.title,
            description=issue_request.description,
            issue_type=issue_request.issueType,  # Updated field name
            components=issue_request.components
        )
        
        keywords = extractor.extract_keywords(issue, limit=limit)
        
        keyword_responses = [
            KeywordResponse(
                word=kw.word,
                frequency=kw.frequency,
                score=kw.score,
                method=kw.method,
                pos_tag=kw.pos_tag,
                confidence=kw.confidence
            ) for kw in keywords
        ]
        
        return {
            "issue_id": issue.id,
            "keywords": keyword_responses,
            "total_keywords": len(keyword_responses)
        }
        
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Extraction failed: {str(e)}")

@app.get("/compare-with-go")
async def compare_with_go():
    """Compare results with Go implementation using the same sample data"""
    try:
        # Use the same sample issue as Go version
        issue = Issue(
            id="ISSUE-123",
            title="Jenkins build fails with timeout error in production deployment",
            description="The Jenkins CI/CD pipeline is consistently failing during the production deployment phase. The error occurs after 30 minutes with a timeout exception. This issue affects the BigRedButton component and prevents automatic deployments to the China region. The build logs show memory allocation errors and network connectivity issues with the database connection pool.",
            issue_type="bug",
            components=["Jenkins", "BigRedButton", "China"]
        )
        
        # Extract with different limits for comparison
        keywords_5 = extractor.extract_keywords(issue, limit=5)
        keywords_10 = extractor.extract_keywords(issue, limit=10)
        keywords_15 = extractor.extract_keywords(issue, limit=15)
        
        return {
            "comparison_note": "Use this endpoint to compare with Go implementation results",
            "issue": {
                "id": issue.id,
                "title": issue.title,
                "description_length": len(issue.description)
            },
            "results": {
                "top_5": [{"word": kw.word, "score": kw.score, "method": kw.method} for kw in keywords_5],
                "top_10": [{"word": kw.word, "score": kw.score, "method": kw.method} for kw in keywords_10],
                "top_15": [{"word": kw.word, "score": kw.score, "method": kw.method} for kw in keywords_15]
            },
            "python_advantages": [
                "Multiple extraction methods (spaCy, YAKE, KeyBERT, TF-IDF)",
                "Advanced BERT-based semantic understanding",
                "Better named entity recognition",
                "More sophisticated linguistic analysis",
                "Domain-specific pattern recognition"
            ]
        }
        
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Comparison failed: {str(e)}")

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8081, log_level="info")