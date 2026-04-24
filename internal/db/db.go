package db

// Tool represents a cached tool
type Tool struct {
	ID          string
	Name        string
	Description string
	Language    string
	CreatedAt   string
	Version     int
	Reviewed    bool
}

// Intent represents an intent record
type Intent struct {
	ID           string
	ToolID       string
	RawInput     string
	CanonicalKey string
	Tags         []string
	CreatedAt    string
}

// Run represents a tool execution record
type Run struct {
	ID         string
	ToolID     string
	InvokedAt  string
	ExitCode   int
	DurationMs int64
}