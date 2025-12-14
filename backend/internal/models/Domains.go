package models

import (
	"database/sql"
	"encoding/json"
	"time"
)

// =============================================================================
// DOMAIN MODEL
// =============================================================================

// Domain - Database Entity (matches MySQL table)
type Domain struct {
	ID          int64          `db:"id"`
	Name        string         `db:"name"`
	Description string         `db:"description"`
	Keywords    sql.NullString `db:"keywords"` // JSON array stored as TEXT
	RiskLevel   string         `db:"risk_level"`
	Teams       sql.NullString `db:"teams"` // JSON array stored as TEXT
	CreatedAt   time.Time      `db:"created_at"`
	UpdatedAt   time.Time      `db:"updated_at"`
}

// DomainDTO - API Request/Response (what clients see)
type DomainDTO struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Keywords    []string  `json:"keywords"`
	RiskLevel   string    `json:"riskLevel"`
	Teams       []string  `json:"teams"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// ToDTO - Convert database entity to API response
func (d *Domain) ToDTO() DomainDTO {
	dto := DomainDTO{
		ID:          d.ID,
		Name:        d.Name,
		Description: d.Description,
		RiskLevel:   d.RiskLevel,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}

	// Parse JSON arrays
	if d.Keywords.Valid {
		json.Unmarshal([]byte(d.Keywords.String), &dto.Keywords)
	}
	if d.Teams.Valid {
		json.Unmarshal([]byte(d.Teams.String), &dto.Teams)
	}

	return dto
}

// FromDTO - Convert API request to database entity
func DomainFromDTO(dto DomainDTO) Domain {
	domain := Domain{
		ID:          dto.ID,
		Name:        dto.Name,
		Description: dto.Description,
		RiskLevel:   dto.RiskLevel,
		CreatedAt:   dto.CreatedAt,
		UpdatedAt:   dto.UpdatedAt,
	}

	// Convert arrays to JSON strings
	if len(dto.Keywords) > 0 {
		keywords, _ := json.Marshal(dto.Keywords)
		domain.Keywords = sql.NullString{String: string(keywords), Valid: true}
	}
	if len(dto.Teams) > 0 {
		teams, _ := json.Marshal(dto.Teams)
		domain.Teams = sql.NullString{String: string(teams), Valid: true}
	}

	return domain
}

// =============================================================================
// SYSTEM COMPONENT MODEL (different from Component enum in issues.go)
// =============================================================================

// SystemComponent - Database Entity for component/service registry
type SystemComponent struct {
	ID          int64          `db:"id"`
	Name        string         `db:"name"`
	Description string         `db:"description"`
	DomainID    sql.NullInt64  `db:"domain_id"`   // FK to domains table
	Keywords    sql.NullString `db:"keywords"`    // JSON array stored as TEXT
	Owner       string         `db:"owner"`       // team responsible
	Criticality string         `db:"criticality"` // high, medium, low
	CreatedAt   time.Time      `db:"created_at"`
	UpdatedAt   time.Time      `db:"updated_at"`
}

// SystemComponentDTO - API Request/Response
type SystemComponentDTO struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	DomainID    *int64    `json:"domainId,omitempty"`
	DomainName  string    `json:"domainName,omitempty"` // populated on read
	Keywords    []string  `json:"keywords"`
	Owner       string    `json:"owner"`
	Criticality string    `json:"criticality"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// ToDTO - Convert database entity to API response
func (c *SystemComponent) ToDTO() SystemComponentDTO {
	dto := SystemComponentDTO{
		ID:          c.ID,
		Name:        c.Name,
		Description: c.Description,
		Owner:       c.Owner,
		Criticality: c.Criticality,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}

	// Handle nullable domain FK
	if c.DomainID.Valid {
		dto.DomainID = &c.DomainID.Int64
	}

	// Parse JSON keywords
	if c.Keywords.Valid {
		json.Unmarshal([]byte(c.Keywords.String), &dto.Keywords)
	}

	return dto
}

// FromDTO - Convert API request to database entity
func SystemComponentFromDTO(dto SystemComponentDTO) SystemComponent {
	component := SystemComponent{
		ID:          dto.ID,
		Name:        dto.Name,
		Description: dto.Description,
		Owner:       dto.Owner,
		Criticality: dto.Criticality,
		CreatedAt:   dto.CreatedAt,
		UpdatedAt:   dto.UpdatedAt,
	}

	// Handle nullable domain FK
	if dto.DomainID != nil {
		component.DomainID = sql.NullInt64{Int64: *dto.DomainID, Valid: true}
	}

	// Convert keywords to JSON
	if len(dto.Keywords) > 0 {
		keywords, _ := json.Marshal(dto.Keywords)
		component.Keywords = sql.NullString{String: string(keywords), Valid: true}
	}

	return component
}

// =============================================================================
// CREATE REQUEST DTOs (for POST/PUT without IDs and timestamps)
// =============================================================================

type CreateDomainRequest struct {
	Name        string   `json:"name" validate:"required"`
	Description string   `json:"description"`
	Keywords    []string `json:"keywords"`
	RiskLevel   string   `json:"riskLevel"`
	Teams       []string `json:"teams"`
}

type CreateSystemComponentRequest struct {
	Name        string   `json:"name" validate:"required"`
	Description string   `json:"description"`
	DomainID    *int64   `json:"domainId"`
	Keywords    []string `json:"keywords"`
	Owner       string   `json:"owner"`
	Criticality string   `json:"criticality"`
}
