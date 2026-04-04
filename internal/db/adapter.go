package db

import "context"

type QueryResult struct {
	Columns     []string
	ColumnTypes []string // e.g. "INTEGER", "TEXT", "VARCHAR". nil if unavailable.
	Rows        [][]string
	Message     string
	Truncated   bool // true when rows were capped at the scan limit
}

type DBAdapter interface {
	Type() string
	Query(context.Context, string) (QueryResult, error)
	Tables(context.Context) ([]string, error)
	Columns(ctx context.Context, tableName string) ([]string, error)
	Schema(context.Context) (string, error)
	QuoteIdentifier(name string) string
	Close() error
}
