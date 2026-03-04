[日本語](README.ja.md)

# asql

A lightweight TUI SQL client for **data observation** — quickly see, sort, and explore raw data to spot anomalies and form hypotheses. Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea). Supports SQLite, MySQL, and PostgreSQL.

## Demo

![asql demo](docs/demo.gif)

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
- **Paging indicator** — status bar shows current position and column info (`col:name 1/100`)
- **Table sidebar** — browse tables, insert SELECT with one key
- **Export** — copy results as CSV / JSON / Markdown, or save to file
- **AI assistant** — generate SQL from natural language via any OpenAI-compatible API

## Key Bindings

| Key | Mode | Action |
|-----|------|--------|
| `i` | NORMAL | Enter INSERT mode |
| `Esc` | INSERT | Return to NORMAL mode |
| `Ctrl+Enter` / `Ctrl+J` | INSERT | Execute query |
| `Tab` | INSERT | Autocomplete table/column name |
| `Ctrl+P` / `Ctrl+N` | INSERT | Previous / next query history |
| `Ctrl+R` | INSERT | Search query history |
| `Ctrl+S` | INSERT | Save current query as snippet |
| `Ctrl+L` | INSERT | Clear editor |
| `j` / `k` | NORMAL | Navigate result rows |
| `h` / `l` | NORMAL | Scroll columns horizontally |
| `s` | NORMAL | Toggle sort on selected column |
| `Enter` | NORMAL | Open Detail View for current row |
| `PgUp` / `PgDn` | NORMAL | Page through results |
| `t` | NORMAL | Open table sidebar |
| `S` | NORMAL | Open saved snippets |
| `P` | NORMAL | Open connection profiles |
| `e` | NORMAL | Open export menu |
| `Ctrl+K` | NORMAL | Open AI assistant |
| `Ctrl+C` | *any* | Cancel running query/AI, or quit |
| `q` | NORMAL | Quit |

## Export

Press `e` in NORMAL mode after executing a query to open the export menu. Supported formats:

- **Copy as CSV** — clipboard
- **Copy as JSON** — clipboard (array of objects)
- **Copy as Markdown** — clipboard (GFM table)
- **Save to File (CSV)** — writes `result_YYYYMMDD_HHMMSS.csv` to current directory

## AI Assistant (Text-to-SQL)

asql can generate SQL from natural language using any OpenAI-compatible API. Create a config file at `~/.config/asql/config.yaml`:

```yaml
ai:
  ai_endpoint: http://localhost:11434/v1   # Ollama
  ai_model: llama3
  ai_api_key: ""                           # optional (Ollama doesn't need one)
```

Press `Ctrl+K` in NORMAL mode to open the AI prompt. The database schema is automatically included in the context for accurate table/column names.

If no config file is present, AI features are silently disabled.

## Development

```bash
go test ./...
go build
go vet ./...
```

## License

MIT — see [LICENSE](LICENSE)
