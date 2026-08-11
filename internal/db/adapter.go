package db

import "context"

// Kind records what a cell's display string in QueryResult.Rows stands for.
//
// Rows is stringified at scan time (see dbutil.StringifyValue), which is the
// only point where the driver's typed value still exists. Kind captures just
// enough of that value — its storage class, plus which strings are display
// sentinels rather than real text — for a consumer to reconstruct the SQL
// value behind a display string. It is deliberately not a type system: there
// is no notion of width, signedness, timestamps or booleans, because nothing
// downstream needs them.
type Kind uint8

const (
	// KindText means the display string is the value verbatim. It is the zero
	// value, so a QueryResult carrying no Kinds behaves as all-text.
	KindText Kind = iota
	// KindNull means SQL NULL. The display string is the "NULL" sentinel.
	KindNull
	// KindEmpty means an empty string. The display string is the `""` sentinel.
	KindEmpty
	// KindInt means an integer. The display string parses with strconv.ParseInt.
	KindInt
	// KindFloat means a floating-point number. The display string parses with
	// strconv.ParseFloat and round-trips exactly.
	KindFloat
	// KindBlob means binary data that was not valid UTF-8. The display string
	// is its lowercase hex encoding and decodes with hex.DecodeString.
	KindBlob
)

type QueryResult struct {
	Columns     []string
	ColumnTypes []string // e.g. "INTEGER", "TEXT", "VARCHAR". nil if unavailable.
	Rows        [][]string
	// Kinds is parallel to Rows: Kinds[r][c] describes Rows[r][c]. It is nil
	// when the producer did not record kinds, in which case every cell must be
	// treated as KindText.
	//
	// INVARIANT: any transformation that reorders, filters or replaces Rows
	// must apply the same transformation to Kinds or clear Kinds entirely.
	// Display-only sorting (see ui.sortedRows) does not violate this because it
	// writes to the model's displayRows, never back into lastResult.Rows.
	Kinds     [][]Kind
	Message   string
	Truncated bool // true when rows were capped at the scan limit
}

// KindAt returns the kind of the cell at (row, col), or KindText when this
// result carries no kind information for it.
func (r QueryResult) KindAt(row, col int) Kind {
	if row < 0 || row >= len(r.Kinds) {
		return KindText
	}
	if col < 0 || col >= len(r.Kinds[row]) {
		return KindText
	}
	return r.Kinds[row][col]
}

// HasKinds reports whether this result carries kind information. Consumers that
// need to fall back to string-sentinel heuristics should branch on this once
// rather than per cell.
func (r QueryResult) HasKinds() bool {
	return len(r.Kinds) > 0
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
