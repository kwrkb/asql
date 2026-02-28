[日本語](README.ja.md)

# sqly

A terminal UI SQL client built with [Bubble Tea](https://github.com/charmbracelet/bubbletea). Currently supports SQLite, with MySQL/PostgreSQL planned.

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
| `q` | NORMAL | Quit |

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
