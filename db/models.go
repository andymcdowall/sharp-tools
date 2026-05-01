package db

import "time"

type Tool struct {
	ID          string
	Name        string
	Description string
	Language    string
	CreatedAt   time.Time
	Version     int
	Reviewed    bool
}

type Intent struct {
	ID           string
	ToolID       string
	RawInput     string
	CanonicalKey string
	Tags         []string
	CreatedAt    time.Time
}

type Run struct {
	ID         string
	ToolID     string
	InvokedAt  time.Time
	ExitCode   int
	DurationMs int64
}

// Migration is an internal record of an applied schema migration.
type Migration struct {
	ID        int64
	Version   string
	AppliedAt time.Time
}
