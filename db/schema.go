package db

// Schema definitions for all tables
const (
	// Schema migrations table
	SchemaMigrationsTable = `CREATE TABLE IF NOT EXISTS schema_migrations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		version TEXT NOT NULL UNIQUE,
		applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	// Tools table
	ToolsTable = `CREATE TABLE IF NOT EXISTS tools (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		canonical_key TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		description TEXT,
		tags TEXT DEFAULT '[]',
		reviewed INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	// Intents table
	IntentsTable = `CREATE TABLE IF NOT EXISTS intents (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tool_id INTEGER NOT NULL,
		tag TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (tool_id) REFERENCES tools(id) ON DELETE CASCADE
	);`

	// Runs table
	RunsTable = `CREATE TABLE IF NOT EXISTS runs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tool_id INTEGER NOT NULL,
		input TEXT,
		output TEXT,
		error TEXT,
		status TEXT DEFAULT 'pending',
		started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		finished_at DATETIME,
		FOREIGN KEY (tool_id) REFERENCES tools(id) ON DELETE CASCADE
	);`

	// Intents FTS5 virtual table for tag search
	IntentsFTSTable = `CREATE VIRTUAL TABLE IF NOT EXISTS intents_fts USING fts5(
		tag,
		content='intents',
		content_rowid='id'
	);`

	// Triggers to keep FTS in sync
	IntentsFTSInsertTrigger = `CREATE TRIGGER IF NOT EXISTS intents_ai AFTER INSERT ON intents BEGIN
		INSERT INTO intents_fts(rowid, tag) VALUES (new.id, new.tag);
	END;`

	IntentsFTSDeleteTrigger = `CREATE TRIGGER IF NOT EXISTS intents_ad AFTER DELETE ON intents BEGIN
		DELETE FROM intents_fts WHERE rowid = old.id;
	END;`

	IntentsFTSUpdateTrigger = `CREATE TRIGGER IF NOT EXISTS intents_au AFTER UPDATE ON intents BEGIN
		DELETE FROM intents_fts WHERE rowid = old.id;
		INSERT INTO intents_fts(rowid, tag) VALUES (new.id, new.tag);
	END;`

	// Indexes
	ToolCanonicalKeyIndex = `CREATE INDEX IF NOT EXISTS idx_tools_canonical_key ON tools(canonical_key);`
	IntentToolIDIndex = `CREATE INDEX IF NOT EXISTS idx_intents_tool_id ON intents(tool_id);`
	IntentTagIndex = `CREATE INDEX IF NOT EXISTS idx_intents_tag ON intents(tag);`
	RunToolIDIndex = `CREATE INDEX IF NOT EXISTS idx_runs_tool_id ON runs(tool_id);`
)

// Migrations define the schema version history
var migrations = []struct {
	Version string
	SQL     []string
}{
	{
		Version: "001_initial",
		SQL: []string{
			ToolsTable,
			IntentsTable,
			RunsTable,
			IntentsFTSTable,
			IntentsFTSInsertTrigger,
			IntentsFTSDeleteTrigger,
			IntentsFTSUpdateTrigger,
			ToolCanonicalKeyIndex,
			IntentToolIDIndex,
			IntentTagIndex,
			RunToolIDIndex,
		},
	},
}