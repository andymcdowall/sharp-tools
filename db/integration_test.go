package db

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
)

// TestIntegration_CascadeDeleteRemovesIntents verifies that deleting a tool
// removes its associated intents and runs via ON DELETE CASCADE.
// This requires PRAGMA foreign_keys = ON to be set on the connection.
func TestIntegration_CascadeDeleteRemovesIntents(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	if err := db.InsertTool(ctx, newTool("cascade-tool", "Cascade Tool", "go")); err != nil {
		t.Fatalf("InsertTool: %v", err)
	}
	if err := db.InsertIntent(ctx, newIntent("ci1", "cascade-tool", "key", []string{"a", "b", "c"})); err != nil {
		t.Fatalf("InsertIntent: %v", err)
	}
	if err := db.InsertRun(ctx, newRun("cr1", "cascade-tool")); err != nil {
		t.Fatalf("InsertRun: %v", err)
	}

	intents, err := db.GetToolIntents(ctx, "cascade-tool")
	if err != nil {
		t.Fatalf("GetToolIntents: %v", err)
	}
	if len(intents) == 0 {
		t.Fatal("expected intents before delete")
	}

	var preRunCount int
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM runs WHERE tool_id = ?`, "cascade-tool").Scan(&preRunCount)
	if preRunCount == 0 {
		t.Fatal("expected run before delete")
	}

	if err := db.DeleteTool(ctx, "cascade-tool"); err != nil {
		t.Fatalf("DeleteTool: %v", err)
	}

	var intentCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM intents WHERE tool_id = ?`, "cascade-tool").Scan(&intentCount); err != nil {
		t.Fatalf("counting intents: %v", err)
	}
	if intentCount != 0 {
		t.Errorf("expected 0 intents after tool delete (ON DELETE CASCADE), got %d", intentCount)
	}

	var runCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM runs WHERE tool_id = ?`, "cascade-tool").Scan(&runCount); err != nil {
		t.Fatalf("counting runs: %v", err)
	}
	if runCount != 0 {
		t.Errorf("expected 0 runs after tool delete (ON DELETE CASCADE), got %d", runCount)
	}
}

// TestIntegration_ConcurrentWritesFileDB verifies that concurrent writes to a
// file-based DB succeed without "database is locked" errors. SQLite requires
// SetMaxOpenConns(1) so all writes go through a single connection.
func TestIntegration_ConcurrentWritesFileDB(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "concurrent.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.RunMigrations(ctx); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	const goroutines = 20
	errs := make([]error, goroutines)
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = db.InsertTool(ctx, newTool(
				fmt.Sprintf("concurrent-tool-%d", i),
				fmt.Sprintf("Tool %d", i),
				"go",
			))
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d write failed (possible database is locked — SetMaxOpenConns must be 1 for SQLite): %v", i, err)
		}
	}

	tools, err := db.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools) != goroutines {
		t.Errorf("expected %d tools, got %d", goroutines, len(tools))
	}
}
