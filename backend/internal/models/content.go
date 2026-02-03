package models

import (
	"time"
)

type ContentSourceType string

const (
	SourceTypeFile     ContentSourceType = "file"
	SourceTypeJira     ContentSourceType = "jira"
	SourceTypeNotion   ContentSourceType = "notion"
	SourceTypeMarkdown ContentSourceType = "markdown"
	SourceTypeJSON     ContentSourceType = "json"
)

type ContentSource struct {
	ID         string                 `json:"id"`
	Type       ContentSourceType      `json:"type"`
	ExternalID string                 `json:"external_id,omitempty"`
	Title      string                 `json:"title,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
}

type JiraIssuePayload struct {
	Key         string           `json:"key"`
	Summary     string           `json:"summary"`
	Description string           `json:"description"`
	Status      string           `json:"status"`
	Priority    string           `json:"priority"`
	IssueType   string           `json:"issue_type"`
	Project     string           `json:"project"`
	Reporter    string           `json:"reporter"`
	Assignee    string           `json:"assignee"`
	Created     time.Time        `json:"created"`
	Updated     time.Time        `json:"updated"`
	Comments    []JiraComment    `json:"comments,omitempty"`
	Labels      []string         `json:"labels,omitempty"`
	Components  []string         `json:"components,omitempty"`
	Attachments []JiraAttachment `json:"attachments,omitempty"`
}

type JiraComment struct {
	ID      string    `json:"id"`
	Author  string    `json:"author"`
	Body    string    `json:"body"`
	Created time.Time `json:"created"`
}

type JiraAttachment struct {
	ID        string `json:"id"`
	Filename  string `json:"filename"`
	URL       string `json:"url"`
	Size      int64  `json:"size"`
	CreatedAt string `json:"created_at"`
}

type NotionPagePayload struct {
	ID           string                 `json:"id"`
	Title        string                 `json:"title"`
	URL          string                 `json:"url"`
	Created      time.Time              `json:"created"`
	LastEdited   time.Time              `json:"last_edited"`
	CreatedBy    string                 `json:"created_by"`
	LastEditedBy string                 `json:"last_edited_by"`
	Properties   map[string]interface{} `json:"properties,omitempty"`
	Content      []NotionBlock          `json:"content"`
}

type NotionBlock struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Content  string           `json:"content,omitempty"`
	RichText []NotionRichText `json:"rich_text,omitempty"`
	Children []NotionBlock    `json:"children,omitempty"`
}

type NotionRichText struct {
	Text   string `json:"text"`
	Bold   bool   `json:"bold,omitempty"`
	Italic bool   `json:"italic,omitempty"`
	Code   bool   `json:"code,omitempty"`
	Link   string `json:"link,omitempty"`
}

type ContentIngestRequest struct {
	SourceType       string             `json:"source_type"`
	Collection       string             `json:"collection"`
	JiraPayload      *JiraIssuePayload  `json:"jira,omitempty"`
	NotionPayload    *NotionPagePayload `json:"notion,omitempty"`
	MarkdownContent  string             `json:"markdown,omitempty"`
	JSONContent      interface{}        `json:"json,omitempty"`
	ChunkingStrategy string             `json:"chunking_strategy"`
	ChunkSize        int                `json:"chunk_size"`
	ChunkOverlap     int                `json:"chunk_overlap"`
	ExtractMetadata  bool               `json:"extract_metadata"`
	NumQuestions     int                `json:"num_questions"`
}

func (r *ContentIngestRequest) Validate() error {
	if r.SourceType == "" {
		return &ValidationError{Field: "source_type", Message: "source_type is required"}
	}
	if r.Collection == "" {
		return &ValidationError{Field: "collection", Message: "collection is required"}
	}

	validTypes := map[ContentSourceType]bool{
		SourceTypeJira:     true,
		SourceTypeNotion:   true,
		SourceTypeMarkdown: true,
		SourceTypeJSON:     true,
	}

	if !validTypes[ContentSourceType(r.SourceType)] {
		return &ValidationError{Field: "source_type", Message: "invalid source_type"}
	}

	if r.SourceType == string(SourceTypeJira) && r.JiraPayload == nil {
		return &ValidationError{Field: "jira", Message: "jira payload is required for jira source_type"}
	}

	if r.SourceType == string(SourceTypeNotion) && r.NotionPayload == nil {
		return &ValidationError{Field: "notion", Message: "notion payload is required for notion source_type"}
	}

	if (r.SourceType == string(SourceTypeMarkdown) || r.SourceType == string(SourceTypeJSON)) && r.JSONContent == nil && r.MarkdownContent == "" {
		return &ValidationError{Field: "content", Message: "content is required for markdown/json source_type"}
	}

	return nil
}

type ContentIngestResponse struct {
	DocumentID string                 `json:"document_id"`
	JobID      string                 `json:"job_id,omitempty"`
	SourceType string                 `json:"source_type"`
	Collection string                 `json:"collection"`
	Status     string                 `json:"status"`
	Message    string                 `json:"message"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}
