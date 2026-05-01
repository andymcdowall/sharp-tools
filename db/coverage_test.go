package db

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpen(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.RunMigrations(ctx); err != nil {
		t.Fatalf("RunMigrations on file db failed: %v", err)
	}
}

func TestOpen_MkdirFails(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a file where a directory would need to be
	blockingFile := filepath.Join(tmpDir, "notadir")
	if err := os.WriteFile(blockingFile, []byte("x"), 0644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	// Try to open a DB under the file path — MkdirAll will fail
	_, err := Open(filepath.Join(blockingFile, "subdir", "test.db"))
	if err == nil {
		t.Error("expected error when parent path is a file")
	}
}

func TestDefaultDBPath(t *testing.T) {
	path := DefaultDBPath()
	if path == "" {
		t.Error("expected non-empty path")
	}
	if !strings.Contains(path, ".sharp-tools") {
		t.Errorf("expected path to contain .sharp-tools, got %q", path)
	}
}

func TestListTools(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory failed: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.RunMigrations(ctx); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	tools, err := db.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools on empty db failed: %v", err)
	}
	if len(tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(tools))
	}

	if _, err := db.InsertTool(ctx, "list-tool-a", "Tool A", "Desc", []string{"tag1"}); err != nil {
		t.Fatalf("InsertTool failed: %v", err)
	}
	if _, err := db.InsertTool(ctx, "list-tool-b", "Tool B", "Desc", []string{"tag2"}); err != nil {
		t.Fatalf("InsertTool failed: %v", err)
	}

	tools, err = db.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	if len(tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(tools))
	}
}

func TestInsertIntent(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory failed: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.RunMigrations(ctx); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	toolID, err := db.InsertTool(ctx, "intent-test-tool", "Intent Tool", "Desc", nil)
	if err != nil {
		t.Fatalf("InsertTool failed: %v", err)
	}

	intentID, err := db.InsertIntent(ctx, toolID, "extra-tag")
	if err != nil {
		t.Fatalf("InsertIntent failed: %v", err)
	}
	if intentID == 0 {
		t.Error("expected non-zero intent ID")
	}

	intents, _ := db.GetToolIntents(ctx, toolID)
	if len(intents) != 1 || intents[0].Tag != "extra-tag" {
		t.Errorf("expected intent with tag 'extra-tag', got %+v", intents)
	}
}

func TestInsertRun(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory failed: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.RunMigrations(ctx); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	toolID, err := db.InsertTool(ctx, "run-test-tool", "Run Tool", "Desc", nil)
	if err != nil {
		t.Fatalf("InsertTool failed: %v", err)
	}

	runID, err := db.InsertRun(ctx, toolID, "some input")
	if err != nil {
		t.Fatalf("InsertRun failed: %v", err)
	}
	if runID == 0 {
		t.Error("expected non-zero run ID")
	}
}

func TestGetMigrationVersions(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory failed: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.RunMigrations(ctx); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	versions, err := db.GetMigrationVersions(ctx)
	if err != nil {
		t.Fatalf("GetMigrationVersions failed: %v", err)
	}
	if len(versions) == 0 {
		t.Fatal("expected at least one migration version")
	}
	if versions[0].Version != "001_initial" {
		t.Errorf("expected version '001_initial', got %q", versions[0].Version)
	}
}

func TestMarkToolReviewed_NotFound(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory failed: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.RunMigrations(ctx); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	err = db.MarkToolReviewed(ctx, 9999, true)
	if err == nil {
		t.Error("expected error for non-existent tool ID")
	}
}

func TestDeleteTool_NotFound(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory failed: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.RunMigrations(ctx); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	err = db.DeleteTool(ctx, 9999)
	if err == nil {
		t.Error("expected error for non-existent tool ID")
	}
}

func TestInsertTool_DuplicateKey(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory failed: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.RunMigrations(ctx); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	_, err = db.InsertTool(ctx, "dup-key", "Tool", "Desc", nil)
	if err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	_, err = db.InsertTool(ctx, "dup-key", "Tool", "Desc", nil)
	if err == nil {
		t.Error("expected error for duplicate canonical key")
	}
}

func TestSearchToolsByTags_Empty(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory failed: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.RunMigrations(ctx); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	results, err := db.SearchToolsByTags(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results != nil {
		t.Error("expected nil results for empty query")
	}
}
