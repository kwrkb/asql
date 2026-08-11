package opener

import (
	"github.com/kwrkb/asql/internal/db"
	"github.com/kwrkb/asql/internal/db/mysql"
	"github.com/kwrkb/asql/internal/db/postgres"
	"github.com/kwrkb/asql/internal/db/readonly"
	"github.com/kwrkb/asql/internal/db/sqlite"
)

// Open creates a DBAdapter from a DSN string by detecting the database type.
func Open(dsn string) (db.DBAdapter, error) {
	switch db.DetectType(dsn) {
	case "mysql":
		return mysql.Open(dsn)
	case "postgres":
		return postgres.Open(dsn)
	default:
		return sqlite.Open(dsn)
	}
}

// OpenReadonly creates a DBAdapter that refuses to write.
//
// Every type gets the statement guard, which is the layer asql relies on.
// SQLite additionally opens the file in the driver's read-only mode; MySQL and
// PostgreSQL have no verified connection-level equivalent here, so for them the
// guard is the only layer. That is why the guard refuses data-modifying CTEs
// and EXPLAIN ANALYZE outright instead of treating them as PostgreSQL trivia.
func OpenReadonly(dsn string) (db.DBAdapter, error) {
	var (
		adapter db.DBAdapter
		err     error
	)
	switch db.DetectType(dsn) {
	case "mysql":
		adapter, err = mysql.Open(dsn)
	case "postgres":
		adapter, err = postgres.Open(dsn)
	default:
		adapter, err = sqlite.OpenReadonly(dsn)
	}
	if err != nil {
		return nil, err
	}
	return readonly.Wrap(adapter), nil
}
