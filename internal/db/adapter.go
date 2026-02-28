package db

import "context"

type QueryResult struct {
	Columns []string
	Rows    [][]string
	Message string
}

type DBAdapter interface {
	Type() string
	Query(context.Context, string) (QueryResult, error)
	Tables(context.Context) ([]string, error)
	Schema(context.Context) (string, error)
	Close() error
}
