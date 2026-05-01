package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// InsertTool inserts a new tool record. The caller must set t.ID before calling.
func (db *DB) InsertTool(ctx context.Context, t Tool) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO tools (id, name, description, language, created_at, version, reviewed)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Name, t.Description, t.Language,
		t.CreatedAt.UTC().Format(time.RFC3339),
		t.Version, boolToInt(t.Reviewed),
	)
	if err != nil {
		return fmt.Errorf("inserting tool: %w", err)
	}
	return nil
}

// InsertIntent inserts a new intent record. The caller must set i.ID before calling.
func (db *DB) InsertIntent(ctx context.Context, i Intent) error {
	tagsJSON, err := json.Marshal(i.Tags)
	if err != nil {
		return fmt.Errorf("marshaling tags: %w", err)
	}
	_, err = db.ExecContext(ctx,
		`INSERT INTO intents (id, tool_id, raw_input, canonical_key, tags, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		i.ID, i.ToolID, i.RawInput, i.CanonicalKey,
		string(tagsJSON),
		i.CreatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("inserting intent: %w", err)
	}
	return nil
}

// MarkToolReviewed sets the reviewed flag to true for the given tool.
func (db *DB) MarkToolReviewed(ctx context.Context, toolID string) error {
	result, err := db.ExecContext(ctx,
		`UPDATE tools SET reviewed = 1 WHERE id = ?`,
		toolID,
	)
	if err != nil {
		return fmt.Errorf("marking tool reviewed: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("tool not found: %s", toolID)
	}
	return nil
}

// GetToolByID retrieves a tool by its string ID.
func (db *DB) GetToolByID(ctx context.Context, id string) (*Tool, error) {
	return scanTool(db.QueryRowContext(ctx,
		`SELECT id, name, description, language, created_at, version, reviewed
		 FROM tools WHERE id = ?`, id))
}

// GetToolByCanonicalKey retrieves a tool via its associated intent's canonical key.
func (db *DB) GetToolByCanonicalKey(ctx context.Context, canonicalKey string) (*Tool, error) {
	return scanTool(db.QueryRowContext(ctx,
		`SELECT t.id, t.name, t.description, t.language, t.created_at, t.version, t.reviewed
		 FROM tools t
		 JOIN intents i ON i.tool_id = t.id
		 WHERE i.canonical_key = ?
		 LIMIT 1`,
		canonicalKey))
}

// SearchToolsByTags searches for the highest-scoring tool by FTS5 tag overlap.
// Score = matching tags / total query tags. Returns nil if no tool meets minScore.
func (db *DB) SearchToolsByTags(ctx context.Context, tags []string, minScore float64) (*Tool, error) {
	if len(tags) == 0 {
		return nil, nil
	}

	var searchTerms []string
	for _, tag := range tags {
		escaped := strings.ReplaceAll(tag, `"`, `""`)
		searchTerms = append(searchTerms, fmt.Sprintf(`"%s"`, escaped))
	}
	searchQuery := strings.Join(searchTerms, " OR ")

	rows, err := db.QueryContext(ctx,
		`SELECT i.tool_id, i.tags
		 FROM intents i
		 WHERE i.rowid IN (SELECT rowid FROM intents_fts WHERE intents_fts MATCH ?)`,
		searchQuery,
	)
	if err != nil {
		return nil, fmt.Errorf("searching tools by tags: %w", err)
	}
	defer rows.Close()

	querySet := make(map[string]bool, len(tags))
	for _, t := range tags {
		querySet[strings.ToLower(t)] = true
	}

	var bestID string
	var bestScore float64

	for rows.Next() {
		var toolID, tagsJSON string
		if err := rows.Scan(&toolID, &tagsJSON); err != nil {
			return nil, fmt.Errorf("scanning search result: %w", err)
		}

		var toolTags []string
		if err := json.Unmarshal([]byte(tagsJSON), &toolTags); err != nil {
			continue
		}

		counted := make(map[string]bool)
		matched := 0
		for _, tt := range toolTags {
			lower := strings.ToLower(tt)
			if querySet[lower] && !counted[lower] {
				matched++
				counted[lower] = true
			}
		}
		score := float64(matched) / float64(len(tags))
		if score >= minScore && score > bestScore {
			bestScore = score
			bestID = toolID
		}
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	if bestID == "" {
		return nil, nil
	}

	return db.GetToolByID(ctx, bestID)
}

// ListTools retrieves all tools ordered by name.
func (db *DB) ListTools(ctx context.Context) ([]Tool, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, name, description, language, created_at, version, reviewed FROM tools ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing tools: %w", err)
	}
	defer rows.Close()

	var tools []Tool
	for rows.Next() {
		t, err := scanToolRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning tool: %w", err)
		}
		tools = append(tools, *t)
	}
	return tools, rows.Err()
}

// DeleteTool deletes a tool and cascades to its intents and runs.
func (db *DB) DeleteTool(ctx context.Context, id string) error {
	result, err := db.ExecContext(ctx, `DELETE FROM tools WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting tool: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("tool not found: %s", id)
	}
	return nil
}

// InsertRun inserts a new run record. The caller must set r.ID before calling.
func (db *DB) InsertRun(ctx context.Context, r Run) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO runs (id, tool_id, invoked_at, exit_code, duration_ms) VALUES (?, ?, ?, ?, ?)`,
		r.ID, r.ToolID, r.InvokedAt.UTC().Format(time.RFC3339), r.ExitCode, r.DurationMs,
	)
	if err != nil {
		return fmt.Errorf("inserting run: %w", err)
	}
	return nil
}

// GetToolIntents retrieves all intents for a given tool ID.
func (db *DB) GetToolIntents(ctx context.Context, toolID string) ([]Intent, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, tool_id, raw_input, canonical_key, tags, created_at FROM intents WHERE tool_id = ? ORDER BY id`,
		toolID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting tool intents: %w", err)
	}
	defer rows.Close()

	var intents []Intent
	for rows.Next() {
		var i Intent
		var tagsJSON, createdAtStr string
		if err := rows.Scan(&i.ID, &i.ToolID, &i.RawInput, &i.CanonicalKey, &tagsJSON, &createdAtStr); err != nil {
			return nil, fmt.Errorf("scanning intent: %w", err)
		}
		if err := json.Unmarshal([]byte(tagsJSON), &i.Tags); err != nil {
			return nil, fmt.Errorf("parsing tags: %w", err)
		}
		if t, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
			i.CreatedAt = t
		}
		intents = append(intents, i)
	}
	return intents, rows.Err()
}

// scanTool scans a single tool row from a *sql.Row query result.
func scanTool(row *sql.Row) (*Tool, error) {
	var t Tool
	var reviewed int
	var createdAtStr string
	err := row.Scan(&t.ID, &t.Name, &t.Description, &t.Language, &createdAtStr, &t.Version, &reviewed)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if ts, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
		t.CreatedAt = ts
	}
	t.Reviewed = reviewed != 0
	return &t, nil
}

// scanToolRow scans a tool from a *sql.Rows cursor.
func scanToolRow(rows *sql.Rows) (*Tool, error) {
	var t Tool
	var reviewed int
	var createdAtStr string
	if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.Language, &createdAtStr, &t.Version, &reviewed); err != nil {
		return nil, err
	}
	if ts, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
		t.CreatedAt = ts
	}
	t.Reviewed = reviewed != 0
	return &t, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
