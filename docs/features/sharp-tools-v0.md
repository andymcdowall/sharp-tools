# Sharp Tools — v0

## Overview

v0 of Sharp Tools: a CLI harness that parses natural language requests, looks up cached tools via structured key and tag overlap, and generates new tools via a Claude Code Docker sidecar when none is found. Generated tools are sandboxed, reviewed, cached, and executed in Docker.

---

## Tickets

### Ticket 01 — Project Scaffold & Config

**Branch:** ticket/sharp-tools-v0/01
**Depends on:** none
**Can run in parallel with:** none

**Scope:**
- Go module init and package skeleton
- `~/.sharp-tools/` directory creation on first run
- `config.toml` schema, defaults, and loading into a typed `Config` struct
- `ANTHROPIC_API_KEY` env var validation

**Directory structure to create:**
```
sharp-tools/
├── cmd/
│   └── sharp-tools/
│       └── main.go          # entry point, wires CLI
├── internal/
│   ├── config/
│   │   └── config.go        # Config struct, Load(), defaults
│   ├── db/
│   │   └── db.go            # DB init, migrations
│   ├── cache/
│   │   └── cache.go         # lookup pipeline
│   ├── intent/
│   │   └── intent.go        # intent parser
│   ├── sidecar/
│   │   └── sidecar.go       # Docker sidecar orchestration
│   ├── sandbox/
│   │   └── sandbox.go       # tool execution sandbox
│   └── cli/
│       └── cli.go           # command definitions
├── go.mod
└── go.sum
```

**`Config` struct** (in `internal/config/config.go`):
```go
type Config struct {
    Generation GenerationConfig
    Execution  ExecutionConfig
    Cache      CacheConfig
    Model      ModelConfig
}

type GenerationConfig struct {
    DefaultLanguage string // "go" | "python" | "typescript"
    MaxIterations   int    // vX — sidecar manages its own loop in v0
}

type ExecutionConfig struct {
    Sandbox       string // "docker" | "none"
    RequireReview bool
}

type CacheConfig struct {
    TagOverlapMinScore         float64
    SimilarityThresholdAccept  float64 // vX
    SimilarityThresholdPrompt  float64 // vX
    AskOnAmbiguousMatch        bool    // vX
}

type ModelConfig struct {
    IntentParser string
}
```

**Default config values:**
```toml
[generation]
default_language = "go"
max_iterations = 10

[execution]
sandbox = "docker"
require_review = true

[cache]
tag_overlap_min_score = 0.6
similarity_threshold_accept = 0.9
similarity_threshold_prompt = 0.7
ask_on_ambiguous_match = true

[model]
intent_parser = "claude-haiku-4-5-20251001"
```

**`Load()` behaviour:**
1. Check `~/.sharp-tools/config.toml` — if absent, write defaults and return them
2. Parse toml into `Config` struct
3. Validate `DefaultLanguage` is one of `"go"`, `"python"`, `"typescript"` — error otherwise

**`ANTHROPIC_API_KEY` handling:**
- Read from environment in `main.go` at startup
- If absent, print `"error: ANTHROPIC_API_KEY is not set"` and exit 1
- Store on a top-level `App` struct passed through the call chain — do not read `os.Getenv` anywhere else

**Recommended libraries:**
- `github.com/BurntSushi/toml` for config parsing

**Unit tests** (`internal/config/config_test.go`):
- `TestLoadDefaults`: call `Load()` pointing at a temp dir with no `config.toml` — assert returned config matches all default values and the file was created
- `TestLoadExistingConfig`: write a partial `config.toml` to a temp dir, call `Load()` — assert provided values are respected and unset values fall back to defaults
- `TestInvalidLanguage`: write a config with `default_language = "ruby"` — assert `Load()` returns an error
- `TestConfigFileCreatedOnMiss`: after `Load()` on an empty dir, assert `config.toml` now exists on disk

**Acceptance criteria:**

*Automated:*
- All unit tests pass
- `go build ./...` produces a binary with no errors or warnings

*Human-run:*
- Delete `~/.sharp-tools/config.toml`, run `sharp-tools config` — verify the file is created and default values are printed
- Unset `ANTHROPIC_API_KEY`, run any command — verify exit code is 1 and the error message is clear

---

### Ticket 02 — Persistence Layer

**Branch:** ticket/sharp-tools-v0/02
**Depends on:** 01
**Can run in parallel with:** 03, 05

**Scope:**
- SQLite database initialisation and schema migrations
- `tools`, `intents`, and `runs` tables
- FTS5 virtual table for tag search
- CRUD helper functions

**Recommended library:** `modernc.org/sqlite` — pure Go, no CGO required, no external `.so` dependency.

**Database location:** `~/.sharp-tools/sharp-tools.db`

**DDL (run in `db.go` on every startup via migration versioning):**
```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS tools (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT NOT NULL,
    language    TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    version     INTEGER NOT NULL DEFAULT 1,
    reviewed    INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS intents (
    id            TEXT PRIMARY KEY,
    tool_id       TEXT NOT NULL REFERENCES tools(id) ON DELETE CASCADE,
    raw_input     TEXT NOT NULL,
    canonical_key TEXT NOT NULL,
    tags          TEXT NOT NULL,   -- JSON array e.g. '["image","convert","tiff","png"]'
    created_at    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS runs (
    id          TEXT PRIMARY KEY,
    tool_id     TEXT NOT NULL REFERENCES tools(id) ON DELETE CASCADE,
    invoked_at  TEXT NOT NULL,
    exit_code   INTEGER NOT NULL,
    duration_ms INTEGER NOT NULL
);

CREATE VIRTUAL TABLE IF NOT EXISTS intents_fts USING fts5(
    tags,
    content='intents',
    content_rowid='rowid'
);
```

**CRUD helpers to implement in `internal/db/db.go`:**
```go
func InsertTool(db *sql.DB, t Tool) error
func InsertIntent(db *sql.DB, i Intent) error
func MarkToolReviewed(db *sql.DB, toolID string) error
func GetToolByID(db *sql.DB, id string) (*Tool, error)
func GetToolByCanonicalKey(db *sql.DB, key string) (*Tool, error)
func SearchToolsByTags(db *sql.DB, tags []string, minScore float64) (*Tool, error)
func ListTools(db *sql.DB) ([]Tool, error)
func DeleteTool(db *sql.DB, id string) error
func InsertRun(db *sql.DB, r Run) error
```

**`SearchToolsByTags` implementation note:**
- Query FTS5 table with the incoming tags joined as a space-separated string
- Score = (number of matching tags) / (total unique tags in query)
- Return the highest-scoring tool above `minScore`, or nil

**Migration versioning:**
- On startup, read current version from `schema_migrations`
- Apply any migrations with version > current in order
- Each migration is a Go function `func migrate_NNN(db *sql.DB) error`

**Types:**
```go
type Tool struct {
    ID          string
    Name        string
    Description string
    Language    string
    CreatedAt   time.Time
    Version     int
    Reviewed    bool
}

type Intent struct {
    ID           string
    ToolID       string
    RawInput     string
    CanonicalKey string
    Tags         []string
    CreatedAt    time.Time
}

type Run struct {
    ID         string
    ToolID     string
    InvokedAt  time.Time
    ExitCode   int
    DurationMs int64
}
```

**Unit tests** (`internal/db/db_test.go`) — use an in-memory SQLite DB (`file::memory:?cache=shared`):
- `TestInsertAndGetTool`: insert a tool, retrieve by ID — assert all fields match
- `TestGetToolByCanonicalKey`: insert a tool with a known key, retrieve by that key — assert match; assert unknown key returns nil
- `TestSearchToolsByTags_Hit`: insert a tool tagged `["image","convert","png"]`, search with `["image","convert","tiff","png"]` and minScore 0.6 — assert it is returned
- `TestSearchToolsByTags_Miss`: same tool, search with `["audio","encode"]` — assert nil returned
- `TestSearchToolsByTags_BelowThreshold`: insert tool with 1 matching tag out of 4 query tags (score 0.25) — assert nil returned at minScore 0.6
- `TestDeleteToolCascades`: insert tool + intent + run, delete tool — assert intent and run records are gone
- `TestMigrationsIdempotent`: run `Init()` twice on the same DB — assert no error and schema_migrations has no duplicate rows
- `TestMarkToolReviewed`: insert tool with `reviewed=false`, mark reviewed, retrieve — assert `reviewed=true`

**Acceptance criteria:**

*Automated:*
- All unit tests pass against an in-memory SQLite DB
- Migrations run without error on a fresh DB and on a DB that already has the latest schema

---

### Ticket 03 — Intent Parser

**Branch:** ticket/sharp-tools-v0/03
**Depends on:** 01
**Can run in parallel with:** 02, 05

**Scope:**
- Call `claude-haiku` via the Anthropic SDK with structured JSON output
- Parse response into a `StructuredIntent` struct
- Derive `CanonicalKey` from structured fields
- Embed the preferred tag list as normalisation hints in the system prompt

**Recommended library:** `github.com/anthropics/anthropic-sdk-go`

**`StructuredIntent` struct:**
```go
type StructuredIntent struct {
    Intent       string
    InputType    string      // "file" | "text" | "data" | "none"
    OutputType   string      // "file" | "text" | "data" | "none"
    Parameters   []Parameter
    Tags         []string
    CanonicalKey string      // derived in Go, not from model
}

type Parameter struct {
    Name     string // e.g. "source_path"
    Type     string // "filepath" | "string" | "integer" | "boolean"
    Required bool
}
```

**`CanonicalKey` derivation** (done in Go after parsing, not by the model):
- Extract the primary operation tag (first tag matching a verb e.g. "convert", "strip", "resize")
- Format: `operation:<op> | input:<input_type> | output:<output_type>`
- Example: `operation:convert | input:tiff | output:png`
- If input or output type cannot be determined, omit that segment

**System prompt for intent parser:**
```
You are an intent parser for a tool-generation system. Given a natural language request, output a JSON object with these fields:
- intent: short normalised description of what the tool should do
- input_type: one of "file", "text", "data", "none"
- output_type: one of "file", "text", "data", "none"
- parameters: array of {name, type, required} — the runtime inputs the tool will need
- tags: array of strings describing the tool

For tags, prefer terms from this list when they apply:
[insert full contents of docs/tags.md here at load time]

Use tags from the list when they fit. You may add novel tags for concepts not covered.
```

**API call configuration:**
- Model: `config.Model.IntentParser`
- Max tokens: 512
- Temperature: 0
- Use the SDK's tool use / structured output mode to enforce JSON schema

**Function signature:**
```go
func Parse(ctx context.Context, client *anthropic.Client, cfg config.ModelConfig, rawInput string) (*StructuredIntent, error)
```

**Error handling:**
- Malformed model response → return typed `ErrParseFailure` with raw response attached
- API call failure → wrap and return the SDK error

**Unit tests** (`internal/intent/intent_test.go`) — mock the HTTP client, do not make live API calls:
- `TestCanonicalKeyDerivation`: call the key derivation logic directly with pre-built `StructuredIntent` values — assert correct key format for: file conversion, text operation, no input/output type
- `TestTagNormalisation`: given a mock response containing `"jpeg"`, assert the parsed intent contains `"jpg"`; same for `"tif"` → `"tiff"`, `"yml"` → `"yaml"`
- `TestEquivalentInputsProduceSameKey`: mock two different raw inputs that mean the same thing (e.g. "convert tiff to png" and "tiff → png converter") returning the same structured output — assert same `CanonicalKey`
- `TestMalformedResponseError`: mock a response with invalid JSON — assert `ErrParseFailure` is returned, not a panic
- `TestParameterExtraction`: mock a response for a file conversion intent — assert `source_path` is present with `required=true` and type `filepath`

**Acceptance criteria:**

*Automated:*
- All unit tests pass with mocked HTTP client

*Human-run:*
- Run `sharp-tools "convert tiff to png"` with a cold cache and `ANTHROPIC_API_KEY` set — verify the intent is parsed and printed at debug log level before cache lookup proceeds (add a temporary debug print if needed)

---

### Ticket 04 — Cache Lookup

**Branch:** ticket/sharp-tools-v0/04
**Depends on:** 02, 03
**Can run in parallel with:** 05

**Scope:**
- Stage 1: exact `canonical_key` lookup
- Stage 2: FTS5 tag overlap scoring
- Returns a typed result indicating hit or miss

**Types:**
```go
type CacheResult int

const (
    CacheMiss CacheResult = iota
    CacheHit
)

type LookupResult struct {
    Result CacheResult
    Tool   *db.Tool  // nil on miss
}
```

**`Lookup` function:**
```go
func Lookup(db *sql.DB, intent *intent.StructuredIntent, cfg config.CacheConfig) (*LookupResult, error)
```

**Stage 1 — Structured Key Match:**
- Call `db.GetToolByCanonicalKey(canonical_key)`
- If a tool is returned → return `LookupResult{Result: CacheHit, Tool: tool}`

**Stage 2 — Tag Overlap:**
- Call `db.SearchToolsByTags(intent.Tags, cfg.TagOverlapMinScore)`
- If a tool is returned → return `LookupResult{Result: CacheHit, Tool: tool}`
- If nil → return `LookupResult{Result: CacheMiss}`

**Logging:**
- Log which stage produced the hit (or that both missed) at debug level
- Include the canonical key and tags in the log entry

**Unit tests** (`internal/cache/cache_test.go`) — use an in-memory SQLite DB seeded with known tools:
- `TestExactKeyHit`: seed a tool with a known canonical key, call `Lookup` with a matching intent — assert `CacheHit` and correct tool returned
- `TestExactKeyStopsAtStageOne`: seed a tool that would match by tags but not by key; seed a different tool that matches by key — assert the key-match tool is returned (Stage 1 terminates early)
- `TestTagOverlapHit`: seed a tool with tags `["image","convert","png"]`, call `Lookup` with intent tags `["image","convert","tiff","png"]` at minScore 0.6 — assert `CacheHit`
- `TestTagOverlapMiss`: same tool, call `Lookup` with intent tags `["audio","encode"]` — assert `CacheMiss`
- `TestBelowThresholdMiss`: seed a tool with 1 matching tag out of 4 query tags — assert `CacheMiss` at default threshold
- `TestEmptyCacheAlwaysMiss`: call `Lookup` on an empty DB — assert `CacheMiss`
- `TestNoLLMCallsInLookup`: assert no HTTP calls are made during any lookup path (inspect via a mock transport that errors on any call)

**Acceptance criteria:**

*Automated:*
- All unit tests pass
- No HTTP calls are made in any lookup code path

---

### Ticket 05 — Sidecar Orchestration

**Branch:** ticket/sharp-tools-v0/05
**Depends on:** 01
**Can run in parallel with:** 02, 03, 04

**Scope:**
- Docker container lifecycle management
- Prompt construction from `StructuredIntent`
- Manifest parsing from output volume

**Recommended library:** `github.com/docker/docker/client`

**Sidecar Docker images** (one per language, pre-built):
- `sharp-tools-sidecar-go` — `golang:1.23-alpine` + Claude Code installed via npm
- `sharp-tools-sidecar-python` — `python:3.12-slim` + Claude Code installed via npm
- `sharp-tools-sidecar-node` — `node:22-alpine` + Claude Code installed via npm

**Container configuration:**
```go
hostConfig := &container.HostConfig{
    Binds: []string{
        outputVolumePath + ":/output",
    },
    NetworkMode: "none",
    Resources: container.Resources{
        Memory:   512 * 1024 * 1024,
        NanoCPUs: 2 * 1e9,
    },
    AutoRemove: true,
}
```

**Environment variables passed to container:**
- `ANTHROPIC_API_KEY` — from host env
- `SHARP_TOOLS_LANGUAGE` — target language
- `SHARP_TOOLS_OUTPUT_DIR` — `/output`

**Prompt construction** — build from `StructuredIntent`:
```
Build a command-line tool in <language> that does the following:

<intent.Intent>

The tool must:
- Accept these parameters as CLI flags or positional arguments: <list parameters>
- Write output to the paths provided via parameters
- Handle these edge cases: <input_type>, <output_type> specifics
- Include unit tests in the appropriate test file for the language

Write all source files to /output/source/ and test files to /output/tests/.
When all tests pass, write /output/manifest.json with this exact structure:
{
  "tool_name": "<kebab-case name>",
  "description": "<one sentence>",
  "language": "<language>",
  "parameters": [{"name": "...", "type": "...", "required": true/false}],
  "tests_passed": true
}

If you cannot make all tests pass, write manifest.json with "tests_passed": false and your best attempt in /output/source/.
Do not exit until you have written manifest.json.
```

**`Manifest` struct:**
```go
type Manifest struct {
    ToolName    string      `json:"tool_name"`
    Description string      `json:"description"`
    Language    string      `json:"language"`
    Parameters  []Parameter `json:"parameters"`
    TestsPassed bool        `json:"tests_passed"`
}
```

**`Run` function:**
```go
func Run(ctx context.Context, dockerClient *client.Client, intent *intent.StructuredIntent, apiKey string, cfg config.Config) (*RunResult, error)

type RunResult struct {
    Manifest   *Manifest
    OutputPath string
    Success    bool
}
```

**`Run` implementation steps:**
1. Create a temp directory on the host for the output volume
2. Select the correct sidecar image based on intent and `cfg.Generation.DefaultLanguage`
3. Create and start the container with the constructed prompt as the Claude Code `-p` argument
4. Stream container logs to stderr at debug level
5. Wait for container to exit (10-minute timeout)
6. Read and parse `/output/manifest.json`
7. Return `RunResult`

**Error cases:**
- Docker not available → return descriptive error before attempting anything
- Container exits non-zero AND no `manifest.json` → return `ErrSidecarCrash`
- `manifest.json` missing after clean exit → return `ErrNoManifest`
- Timeout exceeded → kill container, return `ErrTimeout`

**Unit tests** (`internal/sidecar/sidecar_test.go`) — do not spin up real Docker containers:
- `TestPromptConstruction`: call the prompt builder with a known `StructuredIntent` — assert the output string contains the intent description, parameter names, and language
- `TestManifestParsing`: write a valid `manifest.json` to a temp file, call the manifest parser — assert all fields are correctly populated
- `TestManifestParsingTestsFailed`: parse a manifest with `"tests_passed": false` — assert `RunResult.Success` is false
- `TestMalformedManifest`: pass malformed JSON to the manifest parser — assert a typed error is returned, not a panic
- `TestImageSelection`: call the image selector with each of the three languages — assert the correct image name is returned for each

**Acceptance criteria:**

*Automated:*
- All unit tests pass without Docker

*Human-run (requires Docker and a valid `ANTHROPIC_API_KEY`):*
- Invoke `sidecar.Run` directly via a small test harness (or temporary CLI flag) with a simple intent like "write a Go program that reverses a string" — verify:
  - Container starts and exits cleanly
  - `manifest.json` is present in the output volume with `tests_passed: true`
  - Source and test files are present in `/output/source/` and `/output/tests/`
  - No dangling containers remain after the call (`docker ps -a` shows none)
- Invoke with Docker stopped — verify a clear error is returned before any container ops

---

### Ticket 06 — Code Review & Cache Write

**Branch:** ticket/sharp-tools-v0/06
**Depends on:** 02, 05
**Can run in parallel with:** none

**Scope:**
- Read source from sidecar output volume
- Display to user and prompt for confirmation (if `require_review = true`)
- Write tool to `~/.sharp-tools/tools/<tool-id>/` and insert records into SQLite

**Tool storage layout:**
```
~/.sharp-tools/tools/<tool-id>/
├── source/
│   └── main.go
├── bin/
│   └── tool         # Go only
└── tests/
    └── main_test.go
```

**`Review` function:**
```go
func Review(manifest *sidecar.Manifest, outputPath string, cfg config.ExecutionConfig) (bool, error)
// returns true = confirmed, false = rejected
```

**Review display:**
- Concatenate all files in `outputPath/source/` with `--- filename ---` headers
- Pipe through `$PAGER` if set, otherwise `less`, otherwise print directly to stdout
- After display, prompt: `"Cache and run this tool? [y/N]: "`
- Default is N — require explicit `y` or `yes`

**`Write` function:**
```go
func Write(db *sql.DB, manifest *sidecar.Manifest, outputPath string, rawInput string, canonicalKey string, tags []string) (*db.Tool, error)
```

**`Write` implementation steps:**
1. Generate a UUID v4 for `tool-id`
2. Create `~/.sharp-tools/tools/<tool-id>/source/` and `tests/` directories
3. Copy source files from `outputPath/source/` and test files from `outputPath/tests/`
4. For Go tools: compile the binary with `go build -o ~/.sharp-tools/tools/<tool-id>/bin/tool ./...` inside the source directory
5. Insert `Tool` record with `reviewed = true`
6. Insert `Intent` record with `raw_input`, `canonical_key`, `tags`
7. Return the inserted `Tool`

**Unit tests** (`internal/review/review_test.go`):
- `TestWriteCopiesSourceFiles`: call `Write` with a temp output directory containing known source files — assert the files appear at the correct destination path
- `TestWriteInsertsDBRecords`: call `Write` with an in-memory DB — assert a `Tool` and `Intent` record are present with correct field values
- `TestWriteCompilesGoBinary`: call `Write` with a minimal valid Go source file — assert `bin/tool` exists and is executable
- `TestWriteSkipsBinForPython`: call `Write` with a Python manifest — assert no `bin/` directory is created
- `TestWriteReturnsToolWithCorrectFields`: assert returned `Tool` has `Reviewed=true`, correct language, name from manifest
- `TestRejectLeavesNoDiskArtifacts`: simulate a reject (return false from `Review`) — assert `Write` is not called and no files exist at the tool path

**Acceptance criteria:**

*Automated:*
- All unit tests pass

*Human-run:*
- With `require_review = true`: run a full cache-miss flow — verify the source code is displayed in the pager before execution, and that typing `n` at the prompt causes the tool not to be cached (`sharp-tools list` shows nothing new)
- With `require_review = true`: confirm with `y` — verify the tool appears in `sharp-tools list` and the binary exists at `~/.sharp-tools/tools/<id>/bin/tool`
- With `require_review = false`: run a cache-miss flow — verify no prompt is shown and the tool is cached automatically

---

### Ticket 07 — Execution Sandbox

**Branch:** ticket/sharp-tools-v0/07
**Depends on:** 06
**Can run in parallel with:** none

**Scope:**
- Run a cached tool inside a Docker container with resolved parameters
- Stream output back to the user
- Record the run in the `runs` table

**`Execute` function:**
```go
func Execute(ctx context.Context, dockerClient *client.Client, tool *db.Tool, params map[string]string) error
```

**Container configuration:**
- Image: same language-specific sidecar image
- Network: `"none"`
- Memory: 256MB, CPUs: 1
- `AutoRemove: true`

**Parameter → bind mount mapping:**
- `filepath` parameters: bind-mount the parent directory read-only into `/input/<param-name>/`, write paths read-write into `/output/<param-name>/`
- Non-filepath parameters: pass as `SHARP_PARAM_<NAME>=<value>` environment variables

**Entrypoint construction:**
- Go: `/tool/bin/tool <flags>`
- Python: `python /tool/source/main.py <flags>`
- TypeScript: `npx tsx /tool/source/index.ts <flags>`
- Mount tool directory at `/tool` (read-only)

**Output streaming:**
- Attach to container stdout and stderr, stream in real time with `io.Copy`

**Run recording:**
- Record start time before container start
- Insert `Run` record on container exit with exit code and duration

**Error cases:**
- Non-zero exit code → return `ErrToolFailed{ExitCode: n}`
- Timeout (default 60s) → kill container, return `ErrToolTimeout`
- Missing required input file → validate before starting container, return `ErrMissingInput`

**Unit tests** (`internal/sandbox/sandbox_test.go`):
- `TestBindMountMapping_Filepath`: call the parameter-to-mount mapper with a `filepath` parameter — assert correct source and destination mount paths are generated
- `TestBindMountMapping_NonFilepath`: call with a `string` parameter — assert it appears in the env var list as `SHARP_PARAM_<NAME>` and not in mounts
- `TestEntrypointConstruction_Go`: call entrypoint builder with a Go tool — assert the entrypoint uses `/tool/bin/tool`
- `TestEntrypointConstruction_Python`: same for Python — assert `python /tool/source/main.py`
- `TestEntrypointConstruction_TypeScript`: same for TypeScript — assert `npx tsx /tool/source/index.ts`
- `TestMissingInputFileReturnsError`: call `Execute` with a `filepath` parameter pointing to a nonexistent file — assert `ErrMissingInput` is returned before any Docker call
- `TestRunRecordInserted`: mock the Docker client to return an immediate exit; call `Execute` with an in-memory DB — assert a `Run` record is present with correct exit code and a non-zero duration

**Acceptance criteria:**

*Automated:*
- All unit tests pass without Docker

*Human-run (requires Docker):*
- Run a cached Go tool (e.g. the tiff-to-png converter) with a real input file — verify output file is produced and stdout is streamed live, not buffered
- Run with a missing required input file — verify `ErrMissingInput` is shown before Docker starts (no container appears in `docker ps -a`)
- Run a tool that exits non-zero (e.g. pass an invalid input) — verify the exit code and stderr are shown to the user

---

### Ticket 08 — Runtime Context Prompting

**Branch:** ticket/sharp-tools-v0/08
**Depends on:** 03, 07
**Can run in parallel with:** none

**Scope:**
- Compare parameters required by a tool against what was provided in the original request
- Interactively prompt for any missing required parameters
- Return a fully resolved `map[string]string`

**`Resolve` function:**
```go
func Resolve(required []sidecar.Parameter, parsed *intent.StructuredIntent, rawInput string) (map[string]string, error)
```

**Resolution logic:**
1. For each required parameter: check if a value is inferrable from `rawInput` (e.g. a file path mentioned inline)
2. If found and valid for the type → use it without prompting
3. If not found and `required = true` → prompt interactively
4. If not found and `required = false` → omit from map

**Interactive prompting:**
- Use `bufio.NewReader(os.Stdin)` — no external libraries
- Prompt format: `"  <param_name> (<type>): "`
- Optional parameters: `"  <param_name> (<type>) [optional, press enter to skip]: "`
- For `filepath`: validate with `os.Stat` — if not found, print `"  file not found, try again:"` and re-prompt (max 3 attempts, then return `ErrMaxAttemptsExceeded`)
- For `integer`: validate with `strconv.Atoi`, re-prompt on failure
- For `string` and `boolean`: accept as-is

**Example interaction:**
```
→ Matched cached tool: tiff-to-png-converter
  "Converts TIFF image files to PNG format"

  source_path (filepath): /photos/image.tiff
  dest_path (filepath) [optional, press enter to skip]:
```

**Unit tests** (`internal/context/context_test.go`):
- `TestInlineFilepathExtraction`: call `Resolve` with a `rawInput` that contains a file path (e.g. `"convert /photos/image.tiff to png"`) — assert `source_path` is resolved without any stdin interaction
- `TestOptionalParamOmitted`: call `Resolve` with an optional param not in `rawInput`, simulate empty stdin input — assert the param is absent from the returned map
- `TestFilepathValidation_Valid`: simulate stdin with a path that exists (create a temp file) — assert it is accepted on first attempt
- `TestFilepathValidation_InvalidThenValid`: simulate stdin returning a nonexistent path then a valid path — assert the prompt retried and the valid path is returned
- `TestFilepathValidation_MaxAttempts`: simulate stdin returning three nonexistent paths — assert `ErrMaxAttemptsExceeded` is returned
- `TestIntegerValidation`: simulate stdin returning `"abc"` then `"42"` for an integer param — assert `"42"` is returned after re-prompt
- `TestAllRequiredResolved_NoPrompt`: call `Resolve` where all required params are inferrable from `rawInput` — assert no stdin read occurs

**Acceptance criteria:**

*Automated:*
- All unit tests pass with simulated stdin (use `strings.NewReader` to inject input)

*Human-run:*
- Run `sharp-tools "convert this tiff to png"` with a cached tool and no file path in the request — verify the prompt appears asking for `source_path`
- At the `source_path` prompt, enter a path that does not exist — verify the re-prompt appears with the "file not found" message
- Enter a nonexistent path three times — verify the command exits with `ErrMaxAttemptsExceeded` and a clear error message

---

### Ticket 09 — CLI

**Branch:** ticket/sharp-tools-v0/09
**Depends on:** 04, 07, 08
**Can run in parallel with:** none

**Scope:**
- Wire all packages into a working CLI
- Implement all six subcommands
- End-to-end flows: cache miss and cache hit

**Recommended library:** `github.com/spf13/cobra`

**Command structure:**
```
sharp-tools <request>           main flow
sharp-tools list                list cached tools
sharp-tools inspect <tool-id>   print source of a cached tool
sharp-tools run <tool-id>       run a cached tool directly
sharp-tools remove <tool-id>    delete a cached tool
sharp-tools config              print resolved config
```

**`sharp-tools "<request>"` flow:**
1. Parse intent via `intent.Parse`
2. Call `cache.Lookup`
3. If `CacheHit`: go to step 7
4. If `CacheMiss`: call `sidecar.Run`
5. If `RunResult.Success = false`: print failure, prompt `"Accept best attempt? [y/N]: "` — N exits 0, Y continues
6. Call `review.Review` if `require_review = true` — rejection exits 0
7. Call `review.Write` if new tool
8. Call `context.Resolve` for missing parameters
9. Call `sandbox.Execute`

**`sharp-tools list` output format:**
```
ID                                    NAME                    LANG  CREATED
a1b2c3d4-...                          tiff-to-png-converter   go    2026-04-17
e5f6g7h8-...                          strip-semicolons        go    2026-04-17
```

**`sharp-tools inspect <tool-id>`:**
- Fetch by ID (prefix match on first 8 chars is sufficient)
- Print each source file with `--- filename ---` header
- Exit 1 with `"tool not found: <id>"` if no match

**`sharp-tools run <tool-id> [flags]`:**
- Any `--<param>=<value>` flags are treated as pre-resolved parameters
- Remaining required parameters are collected via `context.Resolve`
- Call `sandbox.Execute`

**`sharp-tools remove <tool-id>`:**
- Confirm with `"Remove <name>? [y/N]: "` before deleting
- Call `db.DeleteTool` then delete `~/.sharp-tools/tools/<tool-id>/`
- Print `"Removed <name>"`

**`sharp-tools config`:**
- Print config in `key = value` format grouped by section
- Redact `ANTHROPIC_API_KEY` to `sk-ant-...****`

**Global error handling:**
- All errors → exit 1 with message to stderr prefixed `"error: "`
- Unrecognised subcommand → print help and exit 1

**Unit tests** (`internal/cli/cli_test.go`) — mock all dependencies (db, intent parser, cache, sidecar, sandbox):
- `TestCacheHitFlow`: mock cache returning `CacheHit`, mock sandbox succeeding — assert `sidecar.Run` is never called
- `TestCacheMissFlow`: mock cache returning `CacheMiss`, mock sidecar returning success — assert `sidecar.Run` is called once and `review.Write` is called
- `TestSidecarFailureSurfaced`: mock sidecar returning `Success=false` — assert the user is prompted to accept or discard
- `TestRemoveConfirmN`: simulate `n` at the remove confirmation — assert `db.DeleteTool` is not called
- `TestRemoveConfirmY`: simulate `y` — assert `db.DeleteTool` is called and the tool directory is deleted
- `TestListEmpty`: mock `db.ListTools` returning empty — assert output says no tools found (not a crash or blank output)
- `TestInspectNotFound`: call inspect with an unknown ID — assert exit 1 and "tool not found" message

**Acceptance criteria:**

*Automated:*
- All unit tests pass with mocked dependencies

*Human-run (full integration — requires Docker and `ANTHROPIC_API_KEY`):*
- **Cache miss end-to-end:** run `sharp-tools "write a tool that reverses a string from stdin"` on a cold cache — verify: intent is parsed, sidecar runs, source is shown for review, tool is cached, tool executes and produces correct output
- **Cache hit end-to-end:** run the same request again — verify: no sidecar is invoked, no review prompt, tool executes immediately
- **`sharp-tools list`:** verify the tool from the above steps appears with correct name, language, and date
- **`sharp-tools inspect <id>`:** verify the source code is printed with filename headers
- **`sharp-tools remove <id>`:** confirm removal — verify the tool no longer appears in `list` and `~/.sharp-tools/tools/<id>/` does not exist
- **`sharp-tools run <id> --param=value`:** verify the flag is accepted and skips the prompt for that parameter
- **Error path:** run with `ANTHROPIC_API_KEY` unset — verify exit 1 with a clear error before any other work begins
