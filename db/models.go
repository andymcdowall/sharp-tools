package db

import (
	"time"
)

// Tool represents a tool in the database
type Tool struct {
	ID             int64
	CanonicalKey   string
	Name           string
	Description    string
	Tags           string // JSON array of tags
	Reviewed       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Intent represents an intent/tag entry in the database
type Intent struct {
	ID        int64
	ToolID    int64
	Tag       string
	CreatedAt time.Time
}

// Run represents a tool execution in the database
type Run struct {
	ID         int64
	ToolID     int64
	Input      string
	Output     string
	Error      string
	Status     string
	StartedAt  time.Time
	FinishedAt time.Time
}

// Migration represents a schema migration record
type Migration struct {
	ID        int64
	Version   string
	AppliedAt time.Time
}

// ToolSearchResult represents a tool with its search score
type ToolSearchResult struct {
	Tool
	Score float64
}