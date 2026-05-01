package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// TagOverlapThreshold is the minimum score (0-1) required for a search result
const TagOverlapThreshold = 0.3

// InsertTool inserts a new tool and returns its ID
func (db *DB) InsertTool(ctx context.Context, canonicalKey, name, description string, tags []string) (int64, error) {
	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return 0, fmt.Errorf("marshaling tags: %w", err)
	}

	result, err := db.ExecContext(ctx,
		`INSERT INTO tools (canonical_key, name, description, tags, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		canonicalKey, name, description, string(tagsJSON), time.Now(), time.Now(),
	)
	if err != nil {
		return 0, fmt.Errorf("inserting tool: %w", err)
	}

	toolID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("getting last insert ID: %w", err)
	}

	// Insert intents for each tag
	if len(tags) > 0 {
		for _, tag := range tags {
			_, err := db.ExecContext(ctx,
				`INSERT INTO intents (tool_id, tag, created_at) VALUES (?, ?, ?)`,
				toolID, tag, time.Now(),
			)
			if err != nil {
				return 0, fmt.Errorf("inserting intent: %w", err)
			}
		}
	}

	return toolID, nil
}

// GetToolByID retrieves a tool by its ID
func (db *DB) GetToolByID(ctx context.Context, id int64) (*Tool, error) {
	var tool Tool
	var tagsJSON string

	err := db.QueryRowContext(ctx,
		`SELECT id, canonical_key, name, description, tags, reviewed, created_at, updated_at
		 FROM tools WHERE id = ?`,
		id,
	).Scan(&tool.ID, &tool.CanonicalKey, &tool.Name, &tool.Description, &tagsJSON, &tool.Reviewed, &tool.CreatedAt, &tool.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting tool by ID: %w", err)
	}

	// Parse tags from JSON
	if err := json.Unmarshal([]byte(tagsJSON), &tool.Tags); err != nil {
		tool.Tags = tagsJSON // Keep original if unmarshal fails
	}

	return &tool, nil
}


// GetToolByCanonicalKey retrieves a tool by its canonical key
func (db *DB) GetToolByCanonicalKey(ctx context.Context, canonicalKey string) (*Tool, error) {
	var tool Tool
	var tagsJSON string

	err := db.QueryRowContext(ctx,
		`SELECT id, canonical_key, name, description, tags, reviewed, created_at, updated_at
		 FROM tools WHERE canonical_key = ?`,
		canonicalKey,
	).Scan(&tool.ID, &tool.CanonicalKey, &tool.Name, &tool.Description, &tagsJSON, &tool.Reviewed, &tool.CreatedAt, &tool.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting tool by canonical key: %w", err)
	}

	// Parse tags from JSON
	if err := json.Unmarshal([]byte(tagsJSON), &tool.Tags); err != nil {
		tool.Tags = tagsJSON // Keep original if unmarshal fails
	}

	return &tool, nil
}

// ListTools retrieves all tools
func (db *DB) ListTools(ctx context.Context) ([]Tool, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, canonical_key, name, description, tags, reviewed, created_at, updated_at
		 FROM tools ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing tools: %w", err)
	}
	defer rows.Close()

	var tools []Tool
	for rows.Next() {
		var tool Tool
		var tagsJSON string

		if err := rows.Scan(&tool.ID, &tool.CanonicalKey, &tool.Name, &tool.Description, &tagsJSON, &tool.Reviewed, &tool.CreatedAt, &tool.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning tool: %w", err)
		}

		// Parse tags from JSON
		if err := json.Unmarshal([]byte(tagsJSON), &tool.Tags); err != nil {
			tool.Tags = tagsJSON
		}

		tools = append(tools, tool)
	}

	return tools, rows.Err()
}

// MarkToolReviewed updates the reviewed status of a tool
func (db *DB) MarkToolReviewed(ctx context.Context, toolID int64, reviewed bool) error {
	result, err := db.ExecContext(ctx,
		`UPDATE tools SET reviewed = ?, updated_at = ? WHERE id = ?`,
		reviewed, time.Now(), toolID,
	)
	if err != nil {
		return fmt.Errorf("marking tool reviewed: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("tool not found: %d", toolID)
	}

	return nil
}

// DeleteTool deletes a tool and cascades to related records
func (db *DB) DeleteTool(ctx context.Context, toolID int64) error {
	result, err := db.ExecContext(ctx, `DELETE FROM tools WHERE id = ?`, toolID)
	if err != nil {
		return fmt.Errorf("deleting tool: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("tool not found: %d", toolID)
	}

	return nil
}

// SearchToolsByTags searches for tools using FTS5 tag overlap scoring
// Score = matching tags / total query tags
func (db *DB) SearchToolsByTags(ctx context.Context, queryTags []string) ([]ToolSearchResult, error) {
	if len(queryTags) == 0 {
		return nil, nil
	}

	// Escape special FTS5 characters and prepare search terms
	var searchTerms []string
	for _, tag := range queryTags {
		// Escape special characters and wrap in quotes
		escaped := strings.ReplaceAll(tag, `"`, `""`)
		searchTerms = append(searchTerms, fmt.Sprintf(`"%s"`, escaped))
	}

	// Use OR to match any of the tags
	searchQuery := strings.Join(searchTerms, " OR ")

	// Query FTS for matching tags
	rows, err := db.QueryContext(ctx,
		`SELECT i.tool_id, COUNT(DISTINCT i.tag) as matched_tags
		 FROM intents_fts f
		 JOIN intents i ON f.rowid = i.id
		 WHERE intents_fts MATCH ?
		 GROUP BY i.tool_id`,
		searchQuery,
	)
	if err != nil {
		return nil, fmt.Errorf("searching tools by tags: %w", err)
	}

	type match struct {
		toolID int64
		score  float64
	}
	var matches []match
	for rows.Next() {
		var toolID int64
		var matchedTags int
		if err := rows.Scan(&toolID, &matchedTags); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scanning search result: %w", err)
		}
		score := float64(matchedTags) / float64(len(queryTags))
		if score >= TagOverlapThreshold {
			matches = append(matches, match{toolID, score})
		}
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var results []ToolSearchResult
	for _, m := range matches {
		tool, err := db.GetToolByID(ctx, m.toolID)
		if err != nil {
			return nil, fmt.Errorf("getting tool: %w", err)
		}
		if tool != nil {
			results = append(results, ToolSearchResult{Tool: *tool, Score: m.score})
		}
	}

	return results, nil
}

// GetToolIntents retrieves all intents for a given tool ID
func (db *DB) GetToolIntents(ctx context.Context, toolID int64) ([]Intent, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, tool_id, tag, created_at FROM intents WHERE tool_id = ? ORDER BY id`,
		toolID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting tool intents: %w", err)
	}
	defer rows.Close()

	var intents []Intent
	for rows.Next() {
		var intent Intent
		if err := rows.Scan(&intent.ID, &intent.ToolID, &intent.Tag, &intent.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning intent: %w", err)
		}
		intents = append(intents, intent)
	}
	return intents, rows.Err()
}

// InsertIntent inserts a new intent/tag for a tool
func (db *DB) InsertIntent(ctx context.Context, toolID int64, tag string) (int64, error) {
	result, err := db.ExecContext(ctx,
		`INSERT INTO intents (tool_id, tag, created_at) VALUES (?, ?, ?)`,
		toolID, tag, time.Now(),
	)
	if err != nil {
		return 0, fmt.Errorf("inserting intent: %w", err)
	}

	intentID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("getting last insert ID: %w", err)
	}

	return intentID, nil
}

// InsertRun inserts a new run record
func (db *DB) InsertRun(ctx context.Context, toolID int64, input string) (int64, error) {
	result, err := db.ExecContext(ctx,
		`INSERT INTO runs (tool_id, input, status, started_at) VALUES (?, ?, ?, ?)`,
		toolID, input, "running", time.Now(),
	)
	if err != nil {
		return 0, fmt.Errorf("inserting run: %w", err)
	}

	runID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("getting last insert ID: %w", err)
	}

	return runID, nil
}
