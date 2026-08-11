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

# Read-only session
asql --readonly @production

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
- **Column statistics** — press `d` for a per-column overlay: NULL rate, distinct count, min/max, plus a sparkline for date columns and a histogram for numeric ones
- **Bring & Join** — press `b` to copy the current result into a local SQLite database, `J` to switch to it, then JOIN results that came from different databases
- **Paging indicator** — status bar shows current position and column info (`col:name 1/100`)
- **Table sidebar** — browse tables, insert SELECT with one key
- **Export** — copy results as CSV / JSON / Markdown, or save to file
- **AI assistant** — generate SQL from natural language via any OpenAI-compatible API
- **Read-only sessions** — `--readonly` refuses statements that would write, so a production connection survives a mistyped `DELETE`

## Read-only Mode

Start asql with `--readonly` when you are connecting to a database you only
mean to look at:

```bash
asql --readonly @production
asql --readonly "postgres://user:pass@db.example.com:5432/app"
```

Every statement is classified before it is sent. Anything not recognized as
read-only is refused with a message naming what was rejected:

```
readonly: DELETE is not allowed (asql --readonly)
```

The status bar marks the connection with `ro` (`production:POSTGRES ro`) so the
mode is never invisible. SQLite databases are additionally opened through the
driver's read-only mode.

What the guard refuses, beyond the obvious `INSERT` / `UPDATE` / `DELETE` /
`DROP`:

- `SELECT` forms that write their result somewhere — PostgreSQL's `SELECT ... INTO backup`, MySQL's `SELECT ... INTO OUTFILE`
- multiple statements in one submission (`SELECT 1; DELETE FROM t`)
- data-modifying CTEs (`WITH gone AS (DELETE FROM t RETURNING *) SELECT * FROM gone`)
- `EXPLAIN ANALYZE` of a writing statement — PostgreSQL runs its target
- pragmas outside schema inspection, including the function form `PRAGMA query_only(0)`
- any keyword it does not recognize
- statements it cannot read with confidence — a backslash-escaped quote
  (`'it\'s'`) is read differently depending on the server's
  `NO_BACKSLASH_ESCAPES` / `standard_conforming_strings` setting, so it is
  refused rather than guessed at; write it as `'it''s'` instead

**This is not a sandbox.** It is there for the `DELETE` you did not mean to
run, not for a user who means to write. asql does not promise that a determined
statement cannot get through, and a read-only connection is not a substitute
for database permissions.

Bring & Join keeps working in a read-only session: the local bring database is
asql's own scratch space and stays writable, so you can still press `b` to
copy a result into it and `J` to join there.

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
| `d` | Open column statistics overlay |
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
| `b` | Bring current result into the local SQLite database (Bring & Join) |
| `J` | Switch to the local Bring & Join database |

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

### STATS mode

| Key | Action |
|-----|--------|
| `j` / `k` / `Down` / `Up` | Navigate columns |
| `q` / `Esc` | Close statistics overlay |

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

## Column Statistics

Press `d` in NORMAL mode to open the statistics overlay. It is computed in memory from the rows you already have — no extra query is sent.

| Field | Meaning |
|-------|---------|
| `NULL%` | Share of rows where the column is SQL NULL |
| `Distinct` | Number of distinct non-NULL values |
| `Min → Max` | Smallest and largest value (numeric when the values are numeric) |

The column under the cursor gets one extra line:

- **Sparkline** for date/timestamp columns — row counts bucketed by year, month or day, e.g. `▁▂▃▅▇▅▂▁  by month`
- **Histogram** for numeric columns — value distribution with the covered range, e.g. `▁▂▅█▇▃▁  0–100`

Both are skipped above 10,000 rows to keep the overlay instant.

## Bring & Join

asql does not federate databases. It brings data to you instead: run a query anywhere, copy the result into a local SQLite database, and JOIN it there.

1. Run a query on the first connection and press `b` — the result becomes local table `t1`.
2. Switch connection (`P`), run another query, press `b` again — that becomes `t2`.
3. Press `J` to switch to the local database, then query across them:

```sql
SELECT t1.name, t2.score
FROM t1 JOIN t2 ON t1.id = t2.id;
```

Values keep their types across the copy: numbers stay numbers, so they sort and JOIN numerically rather than as strings, and SQL NULL stays distinguishable from the text `NULL`.

Every bring is recorded in a `_asql_bring` table inside the local database, so you can always ask where a table came from:

```sql
SELECT * FROM _asql_bring;
```

| Column | Meaning |
|--------|---------|
| `n` | Creation order (matches the number in `t1`, `t2`, …) |
| `table_name` | Local table name |
| `source` | Connection the result came from |
| `row_count` / `col_count` | Size of the brought result |
| `truncated` | `1` when the source result hit the 10,000-row scan limit — the table is a partial view |
| `query` | The query that produced the data |

The local database lives in memory and is gone when asql exits. Export anything you want to keep with `e`.

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
ai:
  ai_endpoint: https://api.openai.com/v1
  ai_model: gpt-4o
  ai_api_key: sk-...
```

```yaml
# Ollama (local)
ai:
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
