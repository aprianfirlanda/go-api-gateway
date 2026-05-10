package transform

import "time"

type TemplateStatus string

const (
	StatusDraft     TemplateStatus = "draft"
	StatusPublished TemplateStatus = "published"
	StatusArchived  TemplateStatus = "archived"
	StatusDisabled  TemplateStatus = "disabled"
)

type Template struct {
	ID             string
	TenantID       string
	APIProductID   string
	Name           string
	SourceProtocol string
	TargetProtocol string
	Version        int
	Status         TemplateStatus
	Request        Section
	Response       Section
	CreatedBy      string
	PublishedAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Section struct {
	Fields    map[string]string
	Sensitive []string
}

type ValidationError struct {
	Field   string
	Message string
}

type ValidationResult struct {
	Errors []ValidationError
}

func (r ValidationResult) Valid() bool {
	return len(r.Errors) == 0
}

type Direction string

const (
	DirectionRequest  Direction = "request"
	DirectionResponse Direction = "response"
)
