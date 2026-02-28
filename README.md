[日本語](README.ja.md)

# sqly

A terminal UI SQL client built with [Bubble Tea](https://github.com/charmbracelet/bubbletea). Currently supports SQLite, with MySQL/PostgreSQL planned. Includes an AI assistant that generates SQL from natural language via OpenAI-compatible APIs (Ollama, LM Studio, etc.).

## Demo

![sqly demo](docs/demo.gif)

## Installation

Download a prebuilt binary from [GitHub Releases](https://github.com/kwrkb/sqly/releases).

Or install with Go:

```bash
go install github.com/kwrkb/sqly@latest
```

Or build from source:

```bash
git clone https://github.com/kwrkb/sqly
cd sqly
go build -o sqly .
```

## Usage

```bash
sqly <path-to-sqlite-file>
```

### Key Bindings

| Key | Mode | Action |
|-----|------|--------|
| `i` | NORMAL | Enter INSERT mode |
| `Esc` | INSERT | Return to NORMAL mode |
| `Ctrl+Enter` / `Ctrl+J` | INSERT | Execute query |
| `j` / `k` | NORMAL | Navigate result rows |
| `t` | NORMAL | Open table sidebar |
| `j` / `k` | SIDEBAR | Navigate tables |
| `Enter` | SIDEBAR | Insert SELECT query for table |
| `Esc` / `t` | SIDEBAR | Close sidebar |
| `Ctrl+K` | NORMAL | Open AI assistant |
| `Enter` | AI | Generate SQL |
| `Esc` | AI | Cancel |
| `q` | NORMAL | Quit |

## AI Assistant (Text-to-SQL)

sqly can generate SQL from natural language using any OpenAI-compatible API. Create a config file at `~/.config/sqly/config.yaml`:

```yaml
ai:
  ai_endpoint: http://localhost:11434/v1   # Ollama
  ai_model: llama3
  ai_api_key: ""                           # optional (Ollama doesn't need one)
```

Press `Ctrl+K` in NORMAL mode to open the AI prompt. The database schema is automatically included in the context for accurate table/column names.

If no config file is present, AI features are silently disabled and sqly works as before.

## Development

```bash
# Run tests
go test ./...

# Build
go build

# Vet
go vet ./...
```

## License

MIT — see [LICENSE](LICENSE)
