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
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory failed: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.RunMigrations(ctx); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	toolID, err := db.InsertTool(ctx, "cascade-tool", "Cascade Tool", "Desc", []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("InsertTool failed: %v", err)
	}

	if _, err := db.InsertRun(ctx, toolID, "test input"); err != nil {
		t.Fatalf("InsertRun failed: %v", err)
	}

	// Sanity-check: both intents and a run exist before delete
	intents, err := db.GetToolIntents(ctx, toolID)
	if err != nil {
		t.Fatalf("GetToolIntents failed: %v", err)
	}
	if len(intents) == 0 {
		t.Fatal("expected intents to exist before delete")
	}

	var preRunCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM runs WHERE tool_id = ?`, toolID).Scan(&preRunCount); err != nil {
		t.Fatalf("counting runs before delete failed: %v", err)
	}
	if preRunCount == 0 {
		t.Fatal("expected run to exist before delete")
	}

	if err := db.DeleteTool(ctx, toolID); err != nil {
		t.Fatalf("DeleteTool failed: %v", err)
	}

	// Directly query intents — cascade should have removed them
	var intentCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM intents WHERE tool_id = ?`, toolID).Scan(&intentCount); err != nil {
		t.Fatalf("counting intents failed: %v", err)
	}
	if intentCount != 0 {
		t.Errorf("expected 0 intents after tool delete (ON DELETE CASCADE), got %d — is PRAGMA foreign_keys = ON set on the connection?", intentCount)
	}

	// Directly query runs — cascade should have removed them too
	var runCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM runs WHERE tool_id = ?`, toolID).Scan(&runCount); err != nil {
		t.Fatalf("counting runs failed: %v", err)
	}
	if runCount != 0 {
		t.Errorf("expected 0 runs after tool delete (ON DELETE CASCADE), got %d — is PRAGMA foreign_keys = ON set on the connection?", runCount)
	}
}

// TestIntegration_ConcurrentWritesFileDB verifies that concurrent writes to a
// file-based DB succeed without "database is locked" errors. SQLite requires
// SetMaxOpenConns(1) so all writes go through a single connection and the
// PRAGMA settings applied at open time remain in effect.
func TestIntegration_ConcurrentWritesFileDB(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "concurrent.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.RunMigrations(ctx); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	const goroutines = 20
	errs := make([]error, goroutines)
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := db.InsertTool(ctx, fmt.Sprintf("concurrent-tool-%d", i), fmt.Sprintf("Tool %d", i), "Desc", []string{"tag"})
			errs[i] = err
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d write failed (possible database is locked — SetMaxOpenConns should be 1 for SQLite): %v", i, err)
		}
	}

	tools, err := db.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	if len(tools) != goroutines {
		t.Errorf("expected %d tools, got %d — some writes may have been lost", goroutines, len(tools))
	}
}
