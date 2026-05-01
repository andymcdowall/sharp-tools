package db

import (
	"context"
	"testing"
)

func TestInsertAndGetTool(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("failed to open memory database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.RunMigrations(ctx); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	toolID, err := db.InsertTool(ctx, "test-tool", "Test Tool", "A test tool", []string{"test", "sample"})
	if err != nil {
		t.Fatalf("failed to insert tool: %v", err)
	}

	tool, err := db.GetToolByID(ctx, toolID)
	if err != nil {
		t.Fatalf("failed to get tool: %v", err)
	}

	if tool == nil || tool.ID != toolID || tool.CanonicalKey != "test-tool" {
		t.Error("tool retrieval failed")
	}
}

func TestGetToolByCanonicalKey(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("failed to open memory database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.RunMigrations(ctx); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	_, err = db.InsertTool(ctx, "my-key", "My Tool", "Description", []string{"tag1"})
	if err != nil {
		t.Fatalf("failed to insert tool: %v", err)
	}

	tool, err := db.GetToolByCanonicalKey(ctx, "my-key")
	if err != nil || tool == nil || tool.Name != "My Tool" {
		t.Error("failed to get tool by canonical key")
	}

	tool, _ = db.GetToolByCanonicalKey(ctx, "nonexistent")
	if tool != nil {
		t.Error("expected nil for non-existent key")
	}
}

func TestSearchToolsByTags_Hit(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("failed to open memory database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.RunMigrations(ctx); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	if _, err := db.InsertTool(ctx, "tool1", "Tool One", "Desc1", []string{"image", "process"}); err != nil {
		t.Fatalf("InsertTool failed: %v", err)
	}
	if _, err := db.InsertTool(ctx, "tool2", "Tool Two", "Desc2", []string{"image", "resize"}); err != nil {
		t.Fatalf("InsertTool failed: %v", err)
	}

	results, err := db.SearchToolsByTags(ctx, []string{"image"})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected search results")
	}
}

func TestSearchToolsByTags_Miss(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("failed to open memory database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.RunMigrations(ctx); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	if _, err := db.InsertTool(ctx, "tool1", "Tool One", "Desc", []string{"specific"}); err != nil {
		t.Fatalf("InsertTool failed: %v", err)
	}

	results, err := db.SearchToolsByTags(ctx, []string{"nonexistent"})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) != 0 {
		t.Error("expected no results")
	}
}

func TestSearchToolsByTags_BelowThreshold(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("failed to open memory database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.RunMigrations(ctx); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	if _, err := db.InsertTool(ctx, "tool1", "Tool One", "Desc", []string{"tag1"}); err != nil {
		t.Fatalf("InsertTool failed: %v", err)
	}

	results, err := db.SearchToolsByTags(ctx, []string{"tag1", "tag2", "tag3", "tag4"})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) != 0 {
		t.Error("expected no results below threshold")
	}
}

func TestDeleteToolCascades(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("failed to open memory database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.RunMigrations(ctx); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	toolID, err := db.InsertTool(ctx, "delete-tool", "Delete Tool", "Desc", []string{"temp"})
	if err != nil {
		t.Fatalf("InsertTool failed: %v", err)
	}

	intents, err := db.GetToolIntents(ctx, toolID)
	if err != nil {
		t.Fatalf("GetToolIntents failed: %v", err)
	}
	if len(intents) == 0 {
		t.Fatal("expected intents to exist before delete")
	}

	if err := db.DeleteTool(ctx, toolID); err != nil {
		t.Fatalf("DeleteTool failed: %v", err)
	}

	tool, _ := db.GetToolByID(ctx, toolID)
	if tool != nil {
		t.Error("expected tool to be nil after delete")
	}

	remaining, err := db.GetToolIntents(ctx, toolID)
	if err != nil {
		t.Fatalf("GetToolIntents after delete failed: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("expected 0 intents after cascade delete, got %d", len(remaining))
	}
}

func TestMigrationsIdempotent(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("failed to open memory database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.RunMigrations(ctx); err != nil {
		t.Fatalf("first RunMigrations failed: %v", err)
	}
	if err := db.RunMigrations(ctx); err != nil {
		t.Fatalf("second RunMigrations failed: %v", err)
	}

	applied, _ := db.IsMigrationApplied(ctx, "001_initial")
	if !applied {
		t.Error("expected migration to be applied")
	}
}

func TestMarkToolReviewed(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("failed to open memory database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.RunMigrations(ctx); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	toolID, err := db.InsertTool(ctx, "review-tool", "Review Tool", "Desc", []string{"test"})
	if err != nil {
		t.Fatalf("InsertTool failed: %v", err)
	}

	tool, err := db.GetToolByID(ctx, toolID)
	if err != nil {
		t.Fatalf("GetToolByID failed: %v", err)
	}
	if tool.Reviewed {
		t.Error("expected reviewed=false initially")
	}

	if err := db.MarkToolReviewed(ctx, toolID, true); err != nil {
		t.Fatalf("MarkToolReviewed failed: %v", err)
	}

	tool, err = db.GetToolByID(ctx, toolID)
	if err != nil {
		t.Fatalf("GetToolByID after mark failed: %v", err)
	}
	if !tool.Reviewed {
		t.Error("expected reviewed=true after marking")
	}
}
