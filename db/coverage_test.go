package db

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOpen(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if err := db.RunMigrations(context.Background()); err != nil {
		t.Fatalf("RunMigrations on file db failed: %v", err)
	}
}

func TestOpen_MkdirFails(t *testing.T) {
	tmpDir := t.TempDir()
	blockingFile := filepath.Join(tmpDir, "notadir")
	if err := os.WriteFile(blockingFile, []byte("x"), 0644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	_, err := Open(filepath.Join(blockingFile, "subdir", "test.db"))
	if err == nil {
		t.Error("expected error when parent path is a file")
	}
}

// TestDefaultDBPath verifies the function returns a valid path containing
// ".sharp-tools" and does not fall back to a tilde literal on success.
func TestDefaultDBPath(t *testing.T) {
	path, err := DefaultDBPath()
	if err != nil {
		t.Fatalf("DefaultDBPath returned error: %v", err)
	}
	if path == "" {
		t.Error("expected non-empty path")
	}
	if !strings.Contains(path, ".sharp-tools") {
		t.Errorf("expected path to contain .sharp-tools, got %q", path)
	}
	if strings.HasPrefix(path, "~") {
		t.Errorf("path must not start with tilde literal, got %q", path)
	}
}

// TestOpenMemory_Ping verifies OpenMemory pings the connection (covers the
// new Ping call that makes it consistent with Open).
func TestOpenMemory_Ping(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory failed: %v", err)
	}
	defer db.Close()
	// Ping implicitly succeeded if OpenMemory returned no error.
}

// TestApplyMigration_RollbackOnFailure verifies that a bad SQL statement
// inside a migration causes an error to be returned (not silently swallowed).
// This covers the `:=` shadowing bug that was fixed.
func TestApplyMigration_RollbackOnFailure(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer db.Close()

	// Manually create the migrations table so applyMigration can record versions.
	if _, err := db.ExecContext(context.Background(), SchemaMigrationsTable); err != nil {
		t.Fatalf("creating migrations table: %v", err)
	}

	bad := struct {
		Version string
		SQL     []string
	}{
		Version: "bad_migration",
		SQL:     []string{"THIS IS NOT VALID SQL !!!"},
	}

	if err := db.applyMigration(context.Background(), bad); err == nil {
		t.Error("expected error from invalid SQL in migration, got nil")
	}

	// The failed migration must not have been recorded.
	applied, err := db.IsMigrationApplied(context.Background(), "bad_migration")
	if err != nil {
		t.Fatalf("IsMigrationApplied: %v", err)
	}
	if applied {
		t.Error("failed migration must not be recorded in schema_migrations")
	}
}

func TestListTools(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	tools, err := db.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools on empty db failed: %v", err)
	}
	if len(tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(tools))
	}

	db.InsertTool(ctx, newTool("a", "Tool A", "go"))
	db.InsertTool(ctx, newTool("b", "Tool B", "python"))

	tools, err = db.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	if len(tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(tools))
	}
}

func TestInsertIntent(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	db.InsertTool(ctx, newTool("t1", "Intent Tool", "go"))

	intent := Intent{
		ID:           "i1",
		ToolID:       "t1",
		RawInput:     "convert tiff to png",
		CanonicalKey: "op:convert|in:tiff|out:png",
		Tags:         []string{"image", "convert", "tiff", "png"},
		CreatedAt:    time.Now(),
	}
	if err := db.InsertIntent(ctx, intent); err != nil {
		t.Fatalf("InsertIntent: %v", err)
	}

	intents, err := db.GetToolIntents(ctx, "t1")
	if err != nil {
		t.Fatalf("GetToolIntents: %v", err)
	}
	if len(intents) != 1 {
		t.Fatalf("expected 1 intent, got %d", len(intents))
	}
	if intents[0].CanonicalKey != "op:convert|in:tiff|out:png" {
		t.Errorf("wrong canonical key: %s", intents[0].CanonicalKey)
	}
	if len(intents[0].Tags) != 4 {
		t.Errorf("expected 4 tags, got %v", intents[0].Tags)
	}
}

func TestInsertRun(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	db.InsertTool(ctx, newTool("t1", "Run Tool", "go"))

	run := Run{
		ID:         "r1",
		ToolID:     "t1",
		InvokedAt:  time.Now(),
		ExitCode:   0,
		DurationMs: 250,
	}
	if err := db.InsertRun(ctx, run); err != nil {
		t.Fatalf("InsertRun: %v", err)
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM runs WHERE id = 'r1'`).Scan(&count); err != nil {
		t.Fatalf("counting run: %v", err)
	}
	if count != 1 {
		t.Errorf("expected run to exist, count=%d", count)
	}
}

func TestGetMigrationVersions(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	versions, err := db.GetMigrationVersions(ctx)
	if err != nil {
		t.Fatalf("GetMigrationVersions: %v", err)
	}
	if len(versions) == 0 {
		t.Fatal("expected at least one migration version")
	}
	if versions[0].Version != "001_initial" {
		t.Errorf("expected 001_initial, got %q", versions[0].Version)
	}
}

func TestMarkToolReviewed_NotFound(t *testing.T) {
	db := setupDB(t)
	if err := db.MarkToolReviewed(context.Background(), "nonexistent"); err == nil {
		t.Error("expected error for non-existent tool")
	}
}

func TestDeleteTool_NotFound(t *testing.T) {
	db := setupDB(t)
	if err := db.DeleteTool(context.Background(), "nonexistent"); err == nil {
		t.Error("expected error for non-existent tool")
	}
}

func TestInsertTool_DuplicateKey(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	db.InsertTool(ctx, newTool("dup", "Tool", "go"))
	if err := db.InsertTool(ctx, newTool("dup", "Tool", "go")); err == nil {
		t.Error("expected error for duplicate ID")
	}
}

func TestSearchToolsByTags_Empty(t *testing.T) {
	db := setupDB(t)
	got, err := db.SearchToolsByTags(context.Background(), nil, 0.6)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Error("expected nil for empty tag query")
	}
}

// TestSearchToolsByTags_ReturnsHighestScore verifies that when multiple intents
// match, the tool with the best score is returned.
func TestSearchToolsByTags_ReturnsHighestScore(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	db.InsertTool(ctx, newTool("t1", "Partial Match", "go"))
	db.InsertIntent(ctx, newIntent("i1", "t1", "key1", []string{"image", "convert"}))

	db.InsertTool(ctx, newTool("t2", "Better Match", "go"))
	db.InsertIntent(ctx, newIntent("i2", "t2", "key2", []string{"image", "convert", "png"}))

	got, err := db.SearchToolsByTags(ctx, []string{"image", "convert", "png"}, 0.6)
	if err != nil {
		t.Fatalf("SearchToolsByTags: %v", err)
	}
	if got == nil {
		t.Fatal("expected a result")
	}
	if got.ID != "t2" {
		t.Errorf("expected higher-scoring tool t2, got %s", got.ID)
	}
}
