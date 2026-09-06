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
// Every type gets the statement guard, which is the layer asql relies on. Each
// also gets a connection-level second layer, but the three are not equally
// strong: SQLite's mode=ro cannot be lifted, while the MySQL and PostgreSQL
// session variables can be turned off by the session itself (measured — see
// docs/readonly-design.md). What refuses that SET is the guard, and a server
// too old for the variable falls back to a plain connection, so the guard is
// still the layer that has to hold. That is why it refuses data-modifying CTEs
// and EXPLAIN ANALYZE outright instead of treating them as PostgreSQL trivia.
func OpenReadonly(dsn string) (db.DBAdapter, error) {
	var (
		adapter db.DBAdapter
		err     error
	)
	switch db.DetectType(dsn) {
	case "mysql":
		adapter, err = mysql.OpenReadonly(dsn)
	case "postgres":
		adapter, err = postgres.OpenReadonly(dsn)
	default:
		adapter, err = sqlite.OpenReadonly(dsn)
	}
	if err != nil {
		return nil, err
	}
	return readonly.Wrap(adapter), nil
}
