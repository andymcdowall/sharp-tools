package db

import (
	"context"
	"testing"
	"time"
)

func newTool(id, name, lang string) Tool {
	return Tool{ID: id, Name: name, Description: "desc", Language: lang, CreatedAt: time.Now(), Version: 1}
}

func newIntent(id, toolID, key string, tags []string) Intent {
	return Intent{ID: id, ToolID: toolID, RawInput: "raw", CanonicalKey: key, Tags: tags, CreatedAt: time.Now()}
}

func newRun(id, toolID string) Run {
	return Run{ID: id, ToolID: toolID, InvokedAt: time.Now(), ExitCode: 0, DurationMs: 100}
}

func setupDB(t *testing.T) *DB {
	t.Helper()
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.RunMigrations(context.Background()); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	return db
}

func TestInsertAndGetTool(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	tool := newTool("t1", "Test Tool", "go")
	if err := db.InsertTool(ctx, tool); err != nil {
		t.Fatalf("InsertTool: %v", err)
	}

	got, err := db.GetToolByID(ctx, "t1")
	if err != nil {
		t.Fatalf("GetToolByID: %v", err)
	}
	if got == nil {
		t.Fatal("expected tool, got nil")
	}
	if got.ID != "t1" || got.Name != "Test Tool" || got.Language != "go" || got.Version != 1 {
		t.Errorf("field mismatch: %+v", got)
	}
	if got.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestGetToolByCanonicalKey(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	if err := db.InsertTool(ctx, newTool("t1", "My Tool", "go")); err != nil {
		t.Fatalf("InsertTool: %v", err)
	}
	if err := db.InsertIntent(ctx, newIntent("i1", "t1", "my-key", []string{"tag1"})); err != nil {
		t.Fatalf("InsertIntent: %v", err)
	}

	got, err := db.GetToolByCanonicalKey(ctx, "my-key")
	if err != nil || got == nil || got.Name != "My Tool" {
		t.Errorf("expected My Tool, got %v (err %v)", got, err)
	}

	missing, err := db.GetToolByCanonicalKey(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if missing != nil {
		t.Error("expected nil for unknown key")
	}
}

// TestSearchToolsByTags_Hit uses the exact scenario from the spec:
// stored tags ["image","convert","png"], query ["image","convert","tiff","png"] at minScore 0.6.
// Score = 3/4 = 0.75 >= 0.6 → hit.
func TestSearchToolsByTags_Hit(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	if err := db.InsertTool(ctx, newTool("t1", "PNG Converter", "go")); err != nil {
		t.Fatalf("InsertTool: %v", err)
	}
	if err := db.InsertIntent(ctx, newIntent("i1", "t1", "op:convert|in:tiff|out:png", []string{"image", "convert", "png"})); err != nil {
		t.Fatalf("InsertIntent: %v", err)
	}

	got, err := db.SearchToolsByTags(ctx, []string{"image", "convert", "tiff", "png"}, 0.6)
	if err != nil {
		t.Fatalf("SearchToolsByTags: %v", err)
	}
	if got == nil {
		t.Fatal("expected a hit (score 0.75 >= 0.6), got nil")
	}
	if got.ID != "t1" {
		t.Errorf("expected tool t1, got %s", got.ID)
	}
}

func TestSearchToolsByTags_Miss(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	if err := db.InsertTool(ctx, newTool("t1", "PNG Converter", "go")); err != nil {
		t.Fatalf("InsertTool: %v", err)
	}
	if err := db.InsertIntent(ctx, newIntent("i1", "t1", "key", []string{"image", "convert", "png"})); err != nil {
		t.Fatalf("InsertIntent: %v", err)
	}

	got, err := db.SearchToolsByTags(ctx, []string{"audio", "encode"}, 0.6)
	if err != nil {
		t.Fatalf("SearchToolsByTags: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

// TestSearchToolsByTags_BelowThreshold: 1 matching tag out of 4 query tags → score 0.25 < 0.6.
func TestSearchToolsByTags_BelowThreshold(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	if err := db.InsertTool(ctx, newTool("t1", "Tool", "go")); err != nil {
		t.Fatalf("InsertTool: %v", err)
	}
	if err := db.InsertIntent(ctx, newIntent("i1", "t1", "key", []string{"tag1"})); err != nil {
		t.Fatalf("InsertIntent: %v", err)
	}

	got, err := db.SearchToolsByTags(ctx, []string{"tag1", "tag2", "tag3", "tag4"}, 0.6)
	if err != nil {
		t.Fatalf("SearchToolsByTags: %v", err)
	}
	if got != nil {
		t.Error("expected nil (score 0.25 below threshold 0.6)")
	}
}

// TestDeleteToolCascades inserts tool + intent + run, deletes the tool,
// and asserts all related records are removed via ON DELETE CASCADE.
func TestDeleteToolCascades(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	if err := db.InsertTool(ctx, newTool("t1", "Cascade Tool", "go")); err != nil {
		t.Fatalf("InsertTool: %v", err)
	}
	if err := db.InsertIntent(ctx, newIntent("i1", "t1", "key", []string{"temp"})); err != nil {
		t.Fatalf("InsertIntent: %v", err)
	}
	if err := db.InsertRun(ctx, newRun("r1", "t1")); err != nil {
		t.Fatalf("InsertRun: %v", err)
	}

	if err := db.DeleteTool(ctx, "t1"); err != nil {
		t.Fatalf("DeleteTool: %v", err)
	}

	if tool, _ := db.GetToolByID(ctx, "t1"); tool != nil {
		t.Error("tool should be gone after delete")
	}

	var intentCount int
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM intents WHERE tool_id = ?`, "t1").Scan(&intentCount)
	if intentCount != 0 {
		t.Errorf("expected 0 intents after cascade delete, got %d", intentCount)
	}

	var runCount int
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM runs WHERE tool_id = ?`, "t1").Scan(&runCount)
	if runCount != 0 {
		t.Errorf("expected 0 runs after cascade delete, got %d", runCount)
	}
}

func TestMigrationsIdempotent(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	// RunMigrations was already called by setupDB; call it again.
	if err := db.RunMigrations(ctx); err != nil {
		t.Fatalf("second RunMigrations failed: %v", err)
	}

	var count int
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version = '001_initial'`).Scan(&count)
	if count != 1 {
		t.Errorf("expected exactly 1 row for 001_initial, got %d", count)
	}
}

func TestMarkToolReviewed(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	if err := db.InsertTool(ctx, newTool("t1", "Review Tool", "go")); err != nil {
		t.Fatalf("InsertTool: %v", err)
	}

	got, err := db.GetToolByID(ctx, "t1")
	if err != nil {
		t.Fatalf("GetToolByID: %v", err)
	}
	if got.Reviewed {
		t.Error("expected Reviewed=false initially")
	}

	if err := db.MarkToolReviewed(ctx, "t1"); err != nil {
		t.Fatalf("MarkToolReviewed: %v", err)
	}

	got, err = db.GetToolByID(ctx, "t1")
	if err != nil {
		t.Fatalf("GetToolByID after mark: %v", err)
	}
	if !got.Reviewed {
		t.Error("expected Reviewed=true after MarkToolReviewed")
	}
}
