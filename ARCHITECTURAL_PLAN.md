# Hybrid Approach

#### Step-by-Step Flow in Detail

1. **User Input Processing**:
   - React sends query/Jira ID to Go API.
   - If Jira ID: Fetch full issue via API, extract description/summary as query text.
2. **Pre-RAG Tag Extraction (Initial LLM Pass)**:
   - Call Ollama with structured prompt on query text.
   - Output: Tags (components, categories, risks).
   - Example: Query "New login with username/password" → {"components": ["Authentication Service"], "specific_risks": ["sql_injection"]}.
3. **Initial Structured Linking (DB Queries)**:
   - Use tags to query Postgres: e.g., SELECT risks.\*, components.name, COUNT(issues.id) OVER (PARTITION BY risks.id) AS issue_count FROM risks JOIN components ON... WHERE tags @> '{"specific_risks": ["sql_injection"]}' AND issues.created_at > NOW() - INTERVAL '6 months'.
   - Builds partial output: Linked components (Output 2), issue lists/stats (Output 3), test scenarios/links (Output 4).
   - If few results: Flag for iteration.
4. **Targeted RAG Retrieval**:
   - Embed query + boost with tags (Chroma hybrid search).
   - Retrieves chunks from all sources (e.g., Notion mitigations, Jira comments, manual tests).
   - No associations yet— just raw content grounded in tags.
5. **Post-RAG LLM Refinement (Iterative Feedback)**:
   - Prompt Ollama with retrieved chunks + initial tags/output: "Review these chunks. Suggest refined tags/associations (e.g., new components, linked issues by ID, additional tests). Validate against provided data. Output structured JSON with updates."
   - LLM outputs: e.g., {"new_tags": {"specific_risks": ["mfa_bypass"]}, "suggested_links": {"issues": ["Jira-789"]}, "refined_analysis": "Text summary incorporating chunks"}.
   - Validate: Go checks if suggested IDs exist in DB (e.g., query issues WHERE jira_key = 'Jira-789').
   - If new valid tags/links: Re-query DB (iteration 1), optionally re-RAG if big changes.
   - Cap at 1-2 iterations (e.g., if no new tags, stop).
6. **Combine & Output**:
   - Merge: DB data (links, stats, lists) + RAG/LLM text (risk analysis from chunks, refined by iteration).
   - JSON to React: Matches your ideals (1: LLM-synthesized text, 2: components from DB/refined, 3: issues/stats from DB, 4: tests/links from DB/chunks).

#### Implementation Tips

- **Go Code Structure**: A loop in the API handler: extract tags → DB query → RAG → LLM refine → if changes, repeat (max 2).
- **Performance**: Pre-RAG keeps it fast (~1s total); iteration adds ~1-2s.
- **Edge Cases**: If LLM suggests invalid links, discard and log for prompt tuning.
- **Scaling**: Cache tag extractions; index DB tags for fast queries.

#### Step 1: User Input Processing

- **Description**: Parse the raw query (or fetch Jira if ID provided). No major transformation—just clean and prepare.
- **Input** (from React API payload):

  ```json
  {
    "query": "What risks should I consider when adding a web form UI for user registration?"
  }
  ```

- **Output** (cleaned query text for next steps):
  ```JSON
  {
    "processed_query": "What risks should I consider when adding a web form UI for user registration?"
  }
  ```

#### Step 2: Pre-RAG Tag Extraction (Initial LLM Pass)

- **Description**: Call Ollama with a structured prompt to extract initial tags from the query text. This gives a starting point for domains/components.
- **Input** (prompt payload to Ollama):

  ```JSON
  {
    "model": "llama3.1:8b",
    "prompt": "Extract components, risk categories, and specific risks from this query. Use platform context: [list components like Frontend UI, Authentication Service]. Output JSON only: {components: [...], risk_categories: [...], specific_risks: [...]}. Query: What risks should I consider when adding a web form UI for user registration?"
  }
  ```

- **Output** (Ollama response – initial tags):
  ```JSON
  {
    "components": ["Frontend UI", "User Profile Service"],
    "risk_categories": ["Security", "Usability"],
    "specific_risks": ["xss_injection", "form_validation_failure"]
  }
  ```

#### Step 3: Initial Structured Linking (DB Queries)

- **Description**: Use extracted tags to query Postgres for direct associations (components, issues, tests). This builds a partial structured output using FKs/tags.
- **Input** (SQL-like query params in Go – e.g., tags from Step 2):

  ```JSON
  {
    "tags": {
      "components": ["Frontend UI", "User Profile Service"],
      "specific_risks": ["xss_injection", "form_validation_failure"]
    },
    "time_filter": "last 6 months"  // For issue stats
  }
  ```

- **Output** (DB results – partial links/stats):

  ```JSON
  {
    "linked_components": [
      {"id": 5, "name": "Frontend UI", "description": "Handles web forms and inputs"}
    ],
    "associated_issues": [
      {"jira_key": "UI-123", "summary": "XSS vuln in registration form", "status": "Resolved"}
    ],
    "issue_stats": {"count_last_6_months": 3, "by_category": {"security": 2, "usability": 1}},
    "test_scenarios": [
      {"scenario": "Inject <script>alert('xss')</script> into form fields", "setup_links": ["https://testing-guide.com/xss"]}
    ]
  }
  ```

#### Step 4: Targeted RAG Retrieval

- **Description**: Embed the query + use tags for hybrid search in Chroma. Retrieves relevant chunks (text from all sources) based on semantics + tag filters.
- **Input** (Chroma query params):
  ```JSON
  {
    "query_text": "What risks should I consider when adding a web form UI for user registration?",
    "query_embedding": [0.123, -0.456, ...],  // From Ollama embed
    "filters": {
      "metadata.tags.specific_risks": {"$in": ["xss_injection", "form_validation_failure"]}
    },
    "top_k": 5
  }
  ```
- **Output** (Retrieved chunks – array of docs with metadata):
  ```JSON
  [
    {
      "chunk_id": "chunk_notn_websec_001",
      "content": "## Form Security Guidelines\n- Sanitize inputs to prevent XSS\n- Use CSRF tokens\nFrom Notion: Web UI Best Practices",
      "metadata": {"source": "notion", "tags": {"specific_risks": ["xss_injection"]}}
    },
    {
      "chunk_id": "chunk_jira_ui123",
      "content": "Jira UI-123: Fixed form validation bug causing data leaks. Tests: Input invalid emails.",
      "metadata": {"source": "jira", "tags": {"specific_risks": ["form_validation_failure"]}}
    }
  ]
  ```

#### Step 5: Post-RAG LLM Refinement (Iterative Feedback)

- **Description**: Feed chunks + initial DB output to Ollama for refinement. LLM suggests new tags/links based on chunks, validates them. If new, iterate (re-run Steps 3-4 with updates).
- **Input** (prompt payload to Ollama – first iteration):
  ```JSON
  {
    "model": "llama3.1:8b",
    "prompt": "Review these chunks and initial data. Suggest refined tags, new links (e.g., issue IDs from chunks), additional tests. Validate existence. Output JSON: {new_tags: {...}, suggested_links: {issues: [...], tests: [...]}, refined_analysis: 'text summary'}. Chunks: [array from Step 4]. Initial Data: [JSON from Step 3]."
  }
  ```
- **Output** (Ollama response – refinements):
  ```JSON
  {
    "new_tags": {
      "specific_risks": ["csrf_vulnerability"]
    },
    "suggested_links": {
      "issues": ["Jira-789"],  // Pulled from chunk text
      "tests": [{"scenario": "Test CSRF by submitting form without token"}]
    },
    "refined_analysis": "Adding web forms risks XSS and CSRF; validate inputs strictly."
  }
  ```
- **Iteration Check**: Go validates (e.g., query DB for "Jira-789" existence). If valid/new, add to tags → re-run Step 3 (DB) and optionally Step 4 (RAG). Example: New tags yield 1 more issue → updated DB output. Stop after 1-2 loops or no changes.

#### Step 6: Combine & Output

- **Description**: Merge all: Initial/refined DB data + RAG chunks + LLM analysis. Send as JSON to React for display.
- **Input** (merged data from prior steps – after iteration):
  ```JSON
  {
    "db_links": [JSON from Step 3 + refinements],
    "rag_chunks": [array from Step 4],
    "llm_refinements": [JSON from Step 5]
  }
  ```
- **Output** (Final holistic JSON – matches your ideals):
  ```JSON
  {
    "risk_analysis": "Adding web forms risks XSS, CSRF, and validation failures. Mitigate with sanitization and tokens. (Refined from chunks/DB).",  // Output 1: LLM text
    "linked_components": ["Frontend UI", "User Profile Service"],  // Output 2: From DB/refined
    "associated_issues": [
      {"jira_key": "UI-123", "summary": "XSS vuln"},
      {"jira_key": "Jira-789", "summary": "CSRF in forms"}  // Refined
    ],
    "issue_stats": {"count_last_6_months": 4},  // Output 3: From DB
    "test_scenarios": [
      "Inject <script>alert('xss')</script>",
      "Submit form without CSRF token",  // Refined
      "setup_links": ["https://testing-guide.com/xss", "https://csrf-howto.com"]
    ]  // Output 4: From DB/chunks
  }
  ```

This flow ensures a holistic view: DB for structured links/stats, RAG for content depth, LLM for smart refinement. Implement iteration as a simple loop in Go with a counter.

# Tasks:

### Shortened GitHub Issues List

1. **Setup Monorepo Structure**  
   Create root folders (backend, frontend, python, etc.) with .gitignore and README.

2. **Configure Docker Compose**  
   Write docker-compose.yml for all services (MySQL, Chroma, Ollama, backend, frontend, python).

3. **Set Up MySQL Schema**  
   Implement minimal tables (components, risks, issues, tests) with JSON tags in db/init.sql.

4. **Configure .env File**  
   Add template for secrets like DB creds, Ollama URL, and API keys.

5. **Implement Go DTOs/Models**  
   Define structs (Component, Risk, etc.) with tags in backend/db/models.go using GORM.

6. **DB Connection/Migrations**  
   Add GORM setup in backend/db to connect and auto-migrate schema.

7. **Build Ollama Wrapper**  
   Create service in backend/services/ollama.go for tag extraction and embeddings.

8. **Ingestion Services**  
   Add Jira/Notion fetchers in backend/services/ingestion to sanitize and insert data.

9. **RAG Pipeline in Go**  
   Handle chunking/embedding/upsert to Chroma in backend/services/rag.

10. **API Handlers for Analysis**  
    Add /analyze endpoint in backend/api for tag extraction, DB queries, RAG, and output.

11. **Python Chunker Service**  
    Set up FastAPI in python/app.py with /chunk endpoint using LlamaIndex.

12. **Integrate Python with Go**  
    Add HTTP calls from Go rag service to Python chunker.

13. **React App Basics**  
    Initialize frontend with routing and basic dashboard page.

14. **Query Input Form**  
    Create React form component to send queries to backend /analyze.

15. **Output Display**  
    Render holistic JSON as UI sections (risks, components, issues, tests).

16. **Review Queue UI**  
    Build page for approving/editing suggested tags/chunks.

17. **Unit/Integration Tests**  
    Add tests for backend services/API and frontend components.

18. **Project Documentation**  
    Update README with architecture, setup, and API details.

19. **CI/CD Pipeline**  
    Add GitHub Actions for build, test, and deploy.

20. **Logging/Monitoring**  
    Implement basic logging in backend and monitor Docker services.
