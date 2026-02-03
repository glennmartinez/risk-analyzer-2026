package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"risk-analyzer/internal/models"
	"risk-analyzer/internal/repositories"
	"risk-analyzer/internal/services"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type ContentHandler struct {
	docRepo         repositories.DocumentRepository
	jobRepo         repositories.JobRepository
	jobStateMachine *services.JobStateMachine
	logger          *log.Logger
}

func NewContentHandler(docRepo repositories.DocumentRepository, jobRepo repositories.JobRepository, logger *log.Logger, jobStateMachine *services.JobStateMachine) *ContentHandler {
	return &ContentHandler{
		docRepo:         docRepo,
		jobRepo:         jobRepo,
		jobStateMachine: jobStateMachine,
		logger:          logger,
	}
}

func (h *ContentHandler) IngestContent(w http.ResponseWriter, r *http.Request) {
	h.logger.Printf("Content ingest request from %s", r.RemoteAddr)

	var req models.ContentIngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Printf("Failed to decode request: %v", err)
		h.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := req.Validate(); err != nil {
		h.logger.Printf("Validation failed: %v", err)
		h.sendError(w, http.StatusBadRequest, fmt.Sprintf("Validation error: %v", err))
		return
	}

	documentID := uuid.New().String()
	jobID := uuid.New().String()

	chunkingStrategy := req.ChunkingStrategy
	if chunkingStrategy == "" {
		chunkingStrategy = "semantic"
	}

	chunkSize := req.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 512
	}

	chunkOverlap := req.ChunkOverlap
	if chunkOverlap < 0 {
		chunkOverlap = 50
	}

	numQuestions := req.NumQuestions
	if numQuestions <= 0 {
		numQuestions = 3
	}

	payload := map[string]interface{}{
		"document_id":       documentID,
		"collection":        req.Collection,
		"source_type":       req.SourceType,
		"chunking_strategy": chunkingStrategy,
		"chunk_size":        chunkSize,
		"chunk_overlap":     chunkOverlap,
		"extract_metadata":  req.ExtractMetadata,
		"num_questions":     numQuestions,
	}

	var jobType repositories.JobType
	var sourceTitle string
	var sourceMetadata map[string]interface{}

	switch models.ContentSourceType(req.SourceType) {
	case models.SourceTypeJira:
		jobType = repositories.JobTypeJiraIngest
		payload["content"] = req.JiraPayload
		sourceTitle = req.JiraPayload.Summary
		sourceMetadata = map[string]interface{}{
			"jira_key":        req.JiraPayload.Key,
			"jira_project":    req.JiraPayload.Project,
			"jira_status":     req.JiraPayload.Status,
			"jira_priority":   req.JiraPayload.Priority,
			"jira_issue_type": req.JiraPayload.IssueType,
			"jira_labels":     req.JiraPayload.Labels,
		}

	case models.SourceTypeNotion:
		jobType = repositories.JobTypeNotionIngest
		payload["content"] = req.NotionPayload
		sourceTitle = req.NotionPayload.Title
		sourceMetadata = map[string]interface{}{
			"notion_id":         req.NotionPayload.ID,
			"notion_url":        req.NotionPayload.URL,
			"notion_created_by": req.NotionPayload.CreatedBy,
		}

	case models.SourceTypeMarkdown:
		jobType = repositories.JobTypeMarkdownIngest
		payload["content"] = req.MarkdownContent
		sourceTitle = "Markdown Content"

	case models.SourceTypeJSON:
		jobType = repositories.JobTypeJSONIngest
		payload["content"] = req.JSONContent
		sourceTitle = "JSON Content"
	}

	job := &repositories.Job{
		ID:         jobID,
		Type:       jobType,
		Status:     repositories.JobStatusQueued,
		Priority:   1,
		Progress:   0,
		Message:    "Content ingestion queued",
		MaxRetries: 3,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		Payload:    payload,
	}

	h.logger.Printf("Creating content ingestion job: job_id=%s, document_id=%s, type=%s", jobID, documentID, jobType)

	if h.docRepo != nil {
		doc := &repositories.Document{
			ID:               documentID,
			Filename:         sourceTitle,
			Collection:       req.Collection,
			Status:           repositories.DocumentStatusPending,
			ChunkingStrategy: chunkingStrategy,
			ChunkSize:        chunkSize,
			ChunkOverlap:     chunkOverlap,
			ExtractMetadata:  req.ExtractMetadata,
			NumQuestions:     numQuestions,
			Metadata:         sourceMetadata,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}

		if err := h.docRepo.Register(r.Context(), doc); err != nil {
			h.logger.Printf("Failed to register document: %v", err)
		}
	}

	if err := h.jobRepo.CreateJob(r.Context(), job); err != nil {
		h.logger.Printf("Failed to create job: %v", err)
		h.sendError(w, http.StatusInternalServerError, "Failed to create job")
		return
	}

	if err := h.jobRepo.EnqueueJob(r.Context(), job); err != nil {
		h.logger.Printf("Failed to enqueue job: %v", err)
		h.sendError(w, http.StatusInternalServerError, "Failed to enqueue job")
		return
	}

	resp := models.ContentIngestResponse{
		DocumentID: documentID,
		JobID:      jobID,
		SourceType: req.SourceType,
		Collection: req.Collection,
		Status:     "queued",
		Message:    "Content ingestion job created successfully",
		Metadata:   sourceMetadata,
	}

	h.sendJSON(w, http.StatusAccepted, resp)
}

func (h *ContentHandler) GetContentStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	documentID := vars["id"]

	h.logger.Printf("Get content status: %s", documentID)

	if h.docRepo == nil {
		h.sendError(w, http.StatusServiceUnavailable, "Document repository not available")
		return
	}

	doc, err := h.docRepo.Get(r.Context(), documentID)
	if err != nil {
		h.logger.Printf("Failed to get document: %v", err)
		h.sendError(w, http.StatusNotFound, "Document not found")
		return
	}

	h.sendJSON(w, http.StatusOK, map[string]interface{}{
		"document_id": documentID,
		"status":      string(doc.Status),
		"filename":    doc.Filename,
		"collection":  doc.Collection,
	})
}

func (h *ContentHandler) sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Printf("Failed to encode JSON: %v", err)
	}
}

func (h *ContentHandler) sendError(w http.ResponseWriter, status int, message string) {
	h.sendJSON(w, status, map[string]string{
		"error":   http.StatusText(status),
		"message": message,
	})
}
