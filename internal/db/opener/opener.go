package opener

import (
	"github.com/kwrkb/asql/internal/db"
	"github.com/kwrkb/asql/internal/db/mysql"
	"github.com/kwrkb/asql/internal/db/postgres"
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
