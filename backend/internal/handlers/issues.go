package handlers

import (
	"encoding/json"
	"net/http"
	"risk-analyzer/config"
	"risk-analyzer/internal/models"
	"risk-analyzer/internal/services"
	"strconv"
)

// IssueWithKeywords represents an issue enhanced with extracted keywords
type IssueWithKeywords struct {
	models.Issue
	Keywords []services.KeywordResult `json:"keywords"`
}

// IssuesHandler godoc
// @Summary Get all issues
// @Description Returns a list of all issues from the configuration
// @Tags issues
// @Accept json
// @Produce json
// @Success 200 {array} models.Issue
// @Router /issues [get]
func IssuesHandler(w http.ResponseWriter, r *http.Request) {
	issues, err := config.LoadFromFile("config/example_issues.json")
	if err != nil {
		http.Error(w, "Failed to load issues: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(issues); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// IssuesWithKeywordsHandler returns issues with extracted keywords
// IssuesWithKeywordsHandler godoc
// @Summary Get issues with keywords
// @Description Returns issues enhanced with extracted keywords
// @Tags issues
// @Accept json
// @Produce json
// @Param limit query int false "Maximum keywords per issue" default(10)
// @Success 200 {array} IssueWithKeywords
// @Router /issues/with-keywords [get]
func IssuesWithKeywordsHandler(w http.ResponseWriter, r *http.Request) {
	issues, err := config.LoadFromFile("config/example_issues.json")
	if err != nil {
		http.Error(w, "Failed to load issues: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Parse limit parameter (default to 10 keywords per issue)
	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	extractor := services.NewKeywordExtractor()
	var enrichedIssues []IssueWithKeywords

	for _, issue := range issues {
		keywords, err := extractor.ExtractTopKeywords(issue, limit)
		if err != nil {
			http.Error(w, "Failed to extract keywords: "+err.Error(), http.StatusInternalServerError)
			return
		}

		enrichedIssue := IssueWithKeywords{
			Issue:    issue,
			Keywords: keywords,
		}
		enrichedIssues = append(enrichedIssues, enrichedIssue)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(enrichedIssues); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// KeywordsOnlyHandler returns only keywords for all issues
// KeywordsOnlyHandler godoc
// @Summary Get keywords only
// @Description Returns only keywords for all issues
// @Tags issues
// @Accept json
// @Produce json
// @Param limit query int false "Maximum keywords per issue" default(5)
// @Success 200 {object} map[string][]string
// @Router /issues/keywords-only [get]
func KeywordsOnlyHandler(w http.ResponseWriter, r *http.Request) {
	issues, err := config.LoadFromFile("config/example_issues.json")
	if err != nil {
		http.Error(w, "Failed to load issues: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Parse limit parameter (default to 5 keywords per issue)
	limit := 5
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	extractor := services.NewKeywordExtractor()
	result := make(map[string][]string)

	for _, issue := range issues {
		keywords, err := extractor.ExtractKeywordStrings(issue, limit)
		if err != nil {
			http.Error(w, "Failed to extract keywords: "+err.Error(), http.StatusInternalServerError)
			return
		}
		result[issue.Id] = keywords
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}
