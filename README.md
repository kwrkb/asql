[日本語](README.ja.md)

# asql

A lightweight TUI SQL client for **data observation** — quickly see, sort, and explore raw data to spot anomalies and form hypotheses. Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea). Supports SQLite, MySQL, and PostgreSQL.

![asql demo](docs/demo.gif)

## Philosophy

asql is not an analytics platform — it's a **data observation tool**.

Heavy lifting belongs in the cloud. asql is for the moment *before* that: quickly touching raw data, noticing anomalies, and forming hypotheses. It stays light, stays quiet, and becomes indispensable.

## Installation

Download a prebuilt binary from [GitHub Releases](https://github.com/kwrkb/asql/releases).

Or install with Go:

```bash
go install github.com/kwrkb/asql@latest
```

Or build from source:

```bash
git clone https://github.com/kwrkb/asql
cd asql
go build -o asql .
```

## Usage

```bash
# SQLite
asql <path-to-sqlite-file>

# MySQL
asql "mysql://user:password@host:3306/dbname"

# PostgreSQL
asql "postgres://user:password@host:5432/dbname"

# Connect via saved profile
asql @myprofile

# Save a connection as a profile
asql --save-profile myprofile "postgres://user:pass@host:5432/db"

# No arguments — select from saved profiles interactively
asql

# Help / version
asql --help
asql --version
```

## Features

- **Type-aware headers** — column types displayed alongside names (`name text`, `age int`)
- **NULL / empty distinction** — NULL stays `NULL`, empty strings shown as `""` so you never confuse them
- **In-place sorting** — press `s` to cycle sort (None → Asc → Desc) on the selected column; NULLs always sort last
- **Detail View** — press `Enter` to inspect a row field-by-field in an overlay; navigate fields with `j`/`k`, rows with `n`/`N`
- **Horizontal scrolling** — wide tables scroll column-by-column with `h`/`l`; status bar shows `[3/12]` column position
- **Tab completion** — press `Tab` in INSERT mode for context-aware table/column name completion
- **Query history** — recall previous queries with `Ctrl+P` / `Ctrl+N`; search history with `Ctrl+R`
- **Saved queries (Snippets)** — save frequently used queries with `Ctrl+S`; browse with `S` in NORMAL mode
- **Connection profiles** — save/load database connections; switch between them with `P` in NORMAL mode
- **Multi-connection** — connections stay open when switching profiles; no re-connect overhead
- **Side-by-side compare mode** — press `c` to pin current result and split the screen into left (pinned) / right (active) panes; use `Tab` to switch focus. Row-count differences and mismatched cells are highlighted immediately
- **Fast re-execution across connections** — press `R` to re-run the current query; in profile mode, `x` switches connection and immediately re-runs
- **Paging indicator** — status bar shows current position and column info (`col:name 1/100`)
- **Table sidebar** — browse tables, insert SELECT with one key
- **Export** — copy results as CSV / JSON / Markdown, or save to file
- **AI assistant** — generate SQL from natural language via any OpenAI-compatible API

## Compare Mode

![compare demo](docs/compare-demo.gif)

> Spot the diff between prod and staging in 3 seconds — right in your terminal.

## Key Bindings

### NORMAL mode

| Key | Action |
|-----|--------|
| `i` | Enter INSERT mode |
| `q` / `Ctrl+C` | Quit |
| `j` / `k` | Navigate result rows |
| `h` / `l` / `Left` / `Right` | Scroll columns horizontally |
| `PgUp` / `PgDn` | Page through results |
| `s` | Toggle sort on selected column (None → Asc → Desc) |
| `Enter` | Open Detail View for current row |
| `R` | Re-execute current query |
| `c` | Toggle compare mode (pin current result / close) |
| `Tab` | Switch focused pane in compare mode (left/right) |
| `t` | Toggle table sidebar |
| `e` | Open export menu |
| `S` | Open saved snippets |
| `Ctrl+S` | Save current query as snippet |
| `P` | Open connection profiles |
| `Ctrl+K` | Open AI assistant |

### INSERT mode

| Key | Action |
|-----|--------|
| `Esc` | Return to NORMAL mode |
| `Ctrl+Enter` / `Ctrl+J` | Execute query |
| `Tab` | Autocomplete table/column name |
| `Ctrl+P` / `Ctrl+N` | Previous / next query history |
| `Ctrl+R` | Search query history |
| `Ctrl+S` | Save current query as snippet |
| `Ctrl+L` | Clear editor |

**Completion popup (when active):**

| Key | Action |
|-----|--------|
| `Tab` / `Ctrl+N` / `Down` | Next completion item |
| `Ctrl+P` / `Up` | Previous completion item |
| `Enter` | Accept selected completion |
| `Esc` | Close popup |

### DETAIL mode

| Key | Action |
|-----|--------|
| `j` / `k` / `Down` / `Up` | Navigate fields |
| `n` / `l` | Next row |
| `N` / `h` | Previous row |
| `q` / `Esc` / `Enter` | Close Detail View |

### SIDEBAR mode

| Key | Action |
|-----|--------|
| `j` / `k` / `Down` / `Up` | Navigate tables |
| `Enter` | Insert `SELECT * FROM <table> LIMIT 100;` into editor and switch to INSERT mode |
| `t` / `Esc` | Close sidebar |

### PROFILE / SNIPPET mode

| Key | Action |
|-----|--------|
| `j` / `k` | Navigate items |
| `Enter` | Connect (PROFILE) / Load into editor (SNIPPET) |
| `x` | Switch connection and re-execute query (PROFILE only) |
| `a` | Add current connection / new snippet |
| `d` | Delete selected item |
| `Esc` | Close |

### EXPORT mode

| Key | Action |
|-----|--------|
| `j` / `k` / `Down` / `Up` | Navigate export options |
| `Enter` | Execute selected export |
| `Esc` | Close |

## Export

Press `e` in NORMAL mode after executing a query to open the export menu. Supported formats:

- **Copy as CSV** — clipboard
- **Copy as JSON** — clipboard (array of objects)
- **Copy as Markdown** — clipboard (GFM table)
- **Save to File (CSV)** — writes `result_YYYYMMDD_HHMMSS.csv` to current directory

## AI Assistant (Text-to-SQL)

asql can generate SQL from natural language using any OpenAI-compatible API.

Create a config file at `~/.config/asql/config.yaml`:

```yaml
ai:
  ai_endpoint: http://localhost:11434/v1   # OpenAI-compatible API endpoint
  ai_model: llama3                         # model name
  ai_api_key: ""                           # API key (optional for local models)
```

**All config fields:**

| Field | Description | Environment variable override |
|-------|-------------|-------------------------------|
| `ai.ai_endpoint` | OpenAI-compatible API base URL | `ASQL_AI_ENDPOINT` |
| `ai.ai_model` | Model name (e.g. `gpt-4o`, `llama3`) | `ASQL_AI_MODEL` |
| `ai.ai_api_key` | API key | `ASQL_AI_API_KEY` |

Environment variables take precedence over the config file. Both `ai_endpoint` and `ai_model` must be set (via file or env) to enable the AI feature.

**Examples:**

```yaml
# OpenAI
ai_endpoint: https://api.openai.com/v1
ai_model: gpt-4o
ai_api_key: sk-...

# Ollama (local)
ai_endpoint: http://localhost:11434/v1
ai_model: llama3
# ai_api_key not needed
```

Press `Ctrl+K` in NORMAL mode to open the AI prompt. The database schema is automatically included in the context for accurate table/column names.

If no config file is present and no environment variables are set, AI features are silently disabled.

## Development

```bash
go test ./...
go build
go vet ./...
```

## License

MIT — see [LICENSE](LICENSE)
