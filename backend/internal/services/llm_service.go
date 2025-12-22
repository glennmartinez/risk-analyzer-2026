package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"risk-analyzer/internal/models"
	"time"
)

const (
	LMStudioBaseURL = "http://localhost:1234/v1"
	DefaultModel    = "llama-3.2-3b-instruct"
)

// LMStudioRequest represents the request format for LM Studio API
type LMStudioRequest struct {
	Model       string               `json:"model"`
	Messages    []models.ChatMessage `json:"messages"`
	Temperature float64              `json:"temperature"`
	MaxTokens   int                  `json:"max_tokens"`
	Stream      bool                 `json:"stream"`
}

// LMStudioResponse represents the response from LM Studio API
type LMStudioResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// LLMService handles communication with LM Studio
type LLMService struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

// NewLLMService creates a new LLM service instance
func NewLLMService() *LLMService {
	return &LLMService{
		baseURL: LMStudioBaseURL,
		model:   DefaultModel,
		httpClient: &http.Client{
			Timeout: 120 * time.Second, // LLMs can be slow
		},
	}
}

// Chat sends a message to LM Studio and returns the response
func (s *LLMService) Chat(ctx context.Context, request models.ChatRequest) (*models.ChatResponse, error) {
	// Build messages array with history
	messages := make([]models.ChatMessage, 0, len(request.History)+2)

	// Add system message
	messages = append(messages, models.ChatMessage{
		Role:    "system",
		Content: "You are an expert software testing consultant having a casual conversation with a developer. Answer the user's question directly and conversationally, like you're explaining it over coffee—use natural language, short paragraphs, and a friendly tone.\n\nBase your answer ONLY on the provided context chunks. Do NOT mention chapter numbers, page numbers, book structure, audiences, or self-assessment tests unless the user specifically asks about the book itself. Do NOT list out tables of contents or say things like \"the book covers Chapter X on Y\". Focus purely on the content and principles—explain them as timeless ideas, not as \"the book says\".\n\nIf the context doesn't fully answer the question, say so honestly instead of forcing it.",
	})

	// Add conversation history
	messages = append(messages, request.History...)

	// Add current user message
	messages = append(messages, models.ChatMessage{
		Role:    "user",
		Content: request.Message,
	})

	// Create LM Studio request
	lmRequest := LMStudioRequest{
		Model:       s.model,
		Messages:    messages,
		Temperature: 0.7,
		MaxTokens:   -1, // No limit
		Stream:      false,
	}

	// Marshal request to JSON
	jsonBody, err := json.Marshal(lmRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", s.baseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to LM Studio: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for non-200 status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LM Studio returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var lmResponse LMStudioResponse
	if err := json.Unmarshal(body, &lmResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract assistant message
	if len(lmResponse.Choices) == 0 {
		return nil, fmt.Errorf("no response from LM Studio")
	}

	return &models.ChatResponse{
		Message: lmResponse.Choices[0].Message.Content,
		Status:  "success",
	}, nil
}

// HealthCheck verifies LM Studio is running and has a model loaded
func (s *LLMService) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", s.baseURL+"/models", nil)
	if err != nil {
		return err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("LM Studio not reachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("LM Studio returned status %d", resp.StatusCode)
	}

	return nil
}
