package db

import "context"

type QueryResult struct {
	Columns     []string
	ColumnTypes []string // e.g. "INTEGER", "TEXT", "VARCHAR". nil if unavailable.
	Rows        [][]string
	Message     string
}

type DBAdapter interface {
	Type() string
	Query(context.Context, string) (QueryResult, error)
	Tables(context.Context) ([]string, error)
	Schema(context.Context) (string, error)
	Close() error
}
