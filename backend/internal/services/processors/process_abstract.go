package processors

import (
	"context"
	"net/url"

	"risk-analyzer/internal/repositories"
	"risk-analyzer/internal/services"
)

// BaseProcessor contains shared dependencies and common helpers for all processors.
type BaseProcessor struct {
	Py      services.PythonClientInterface
	JobRepo repositories.JobRepository
	DocRepo repositories.DocumentRepository

	// CallbackBase is the base host + scheme for building the callback.
	// Example: "http://go-server:8080"
	CallbackBase string

	// CallbackPath is the common path used by all processors for callbacks.
	// Default: "/api/v1/documents/upload-callback"
	CallbackPath string
}

// CallbackURL builds an absolute callback URL (joins base + path). It always returns a string.
func (b *BaseProcessor) CallbackURL() string {
	if b.CallbackBase == "" {
		b.CallbackBase = "http://localhost:8080" // safe default for dev
	}
	if b.CallbackPath == "" {
		b.CallbackPath = "/api/v1/documents/upload-callback"
	}
	// try to join cleanly; fall back to simple concatenation
	if u, err := url.JoinPath(b.CallbackBase, b.CallbackPath); err == nil {
		return u
	}
	return b.CallbackBase + b.CallbackPath
}

// Default HandleCallback - no-op. Processors can override when they need custom handling.
func (b *BaseProcessor) HandleCallback(ctx context.Context, job *repositories.Job, payload map[string]interface{}) error {
	// Default behavior: state machine handles success/failure mapping.
	// If needed, concrete processors override this to store processor-specific data.
	return nil
}
