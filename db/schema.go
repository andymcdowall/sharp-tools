package db

const (
	SchemaMigrationsTable = `CREATE TABLE IF NOT EXISTS schema_migrations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		version TEXT NOT NULL UNIQUE,
		applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	ToolsTable = `CREATE TABLE IF NOT EXISTS tools (
		id          TEXT PRIMARY KEY,
		name        TEXT NOT NULL,
		description TEXT NOT NULL,
		language    TEXT NOT NULL,
		created_at  TEXT NOT NULL,
		version     INTEGER NOT NULL DEFAULT 1,
		reviewed    INTEGER NOT NULL DEFAULT 0
	);`

	IntentsTable = `CREATE TABLE IF NOT EXISTS intents (
		id            TEXT PRIMARY KEY,
		tool_id       TEXT NOT NULL REFERENCES tools(id) ON DELETE CASCADE,
		raw_input     TEXT NOT NULL,
		canonical_key TEXT NOT NULL,
		tags          TEXT NOT NULL,
		created_at    TEXT NOT NULL
	);`

	RunsTable = `CREATE TABLE IF NOT EXISTS runs (
		id          TEXT PRIMARY KEY,
		tool_id     TEXT NOT NULL REFERENCES tools(id) ON DELETE CASCADE,
		invoked_at  TEXT NOT NULL,
		exit_code   INTEGER NOT NULL,
		duration_ms INTEGER NOT NULL
	);`

	IntentsFTSTable = `CREATE VIRTUAL TABLE IF NOT EXISTS intents_fts USING fts5(
		tags,
		content='intents',
		content_rowid='rowid'
	);`

	IntentsFTSInsertTrigger = `CREATE TRIGGER IF NOT EXISTS intents_ai AFTER INSERT ON intents BEGIN
		INSERT INTO intents_fts(rowid, tags) VALUES (new.rowid, new.tags);
	END;`

	IntentsFTSDeleteTrigger = `CREATE TRIGGER IF NOT EXISTS intents_ad AFTER DELETE ON intents BEGIN
		DELETE FROM intents_fts WHERE rowid = old.rowid;
	END;`

	IntentsFTSUpdateTrigger = `CREATE TRIGGER IF NOT EXISTS intents_au AFTER UPDATE ON intents BEGIN
		DELETE FROM intents_fts WHERE rowid = old.rowid;
		INSERT INTO intents_fts(rowid, tags) VALUES (new.rowid, new.tags);
	END;`

	// idx_intents_canonical_key supports GetToolByCanonicalKey joins.
	IntentCanonicalKeyIndex = `CREATE INDEX IF NOT EXISTS idx_intents_canonical_key ON intents(canonical_key);`
	IntentToolIDIndex       = `CREATE INDEX IF NOT EXISTS idx_intents_tool_id ON intents(tool_id);`
	RunToolIDIndex          = `CREATE INDEX IF NOT EXISTS idx_runs_tool_id ON runs(tool_id);`
)

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
			IntentCanonicalKeyIndex,
			IntentToolIDIndex,
			RunToolIDIndex,
		},
	},
}
