package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	_ "github.com/go-sql-driver/mysql"

	"github.com/kwrkb/asql/internal/db"
	"github.com/kwrkb/asql/internal/db/dbutil"
)

type Adapter struct {
	conn *sql.DB
}

// Open connects to a MySQL database using the given DSN.
// Accepts mysql:// URL format and converts it to go-sql-driver format.
func Open(dsn string) (*Adapter, error) {
	driverDSN := convertDSN(dsn)

	conn, err := sql.Open("mysql", driverDSN)
	if err != nil {
		return nil, err
	}

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, err
	}

	return &Adapter{conn: conn}, nil
}

func (a *Adapter) Type() string { return "mysql" }

func (a *Adapter) Close() error {
	return a.conn.Close()
}

func (a *Adapter) Tables(ctx context.Context) ([]string, error) {
	rows, err := a.conn.QueryContext(ctx, "SHOW TABLES")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

func (a *Adapter) Schema(ctx context.Context) (string, error) {
	tables, err := a.Tables(ctx)
	if err != nil {
		return "", err
	}

	var stmts []string
	for _, t := range tables {
		var tableName, ddl string
		quoted := "`" + strings.ReplaceAll(t, "`", "``") + "`"
		err := a.conn.QueryRowContext(ctx, "SHOW CREATE TABLE "+quoted).Scan(&tableName, &ddl)
		if err != nil {
			return "", fmt.Errorf("SHOW CREATE TABLE %s: %w", t, err)
		}
		stmts = append(stmts, ddl+";")
	}
	return strings.Join(stmts, "\n\n"), nil
}

func (a *Adapter) Query(ctx context.Context, query string) (db.QueryResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return db.QueryResult{}, fmt.Errorf("query is empty")
	}

	if returnsRows(query) {
		rows, err := a.conn.QueryContext(ctx, query)
		if err != nil {
			return db.QueryResult{}, err
		}
		defer rows.Close()
		return dbutil.ScanRows(rows)
	}

	res, err := a.conn.ExecContext(ctx, query)
	if err != nil {
		return db.QueryResult{}, err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return db.QueryResult{Message: "statement executed (rows affected unknown)"}, nil
	}

	return db.QueryResult{
		Message: fmt.Sprintf("%d row(s) affected", rowsAffected),
	}, nil
}

// returnsRows determines whether a SQL statement returns a result set.
// MySQL does not support RETURNING clause.
func returnsRows(query string) bool {
	keyword := dbutil.LeadingKeyword(query)
	switch keyword {
	case "select", "show", "describe", "desc", "explain", "values", "table":
		return true
	case "with":
		body := dbutil.CteBodyKeyword(query)
		switch body {
		case "select", "values", "table", "show", "describe", "desc", "explain":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

// convertDSN converts a mysql:// URL to go-sql-driver DSN format.
// Input:  mysql://user:pass@host:port/dbname?charset=utf8mb4
// Output: user:pass@tcp(host:port)/dbname?charset=utf8mb4&parseTime=true
func convertDSN(dsn string) string {
	if !strings.HasPrefix(dsn, "mysql://") {
		return dsn
	}

	u, err := url.Parse(dsn)
	if err != nil {
		return dsn
	}

	var userInfo string
	if u.User != nil {
		userInfo = u.User.String()
	}

	host := u.Host
	if host == "" {
		host = "127.0.0.1:3306"
	}

	path := strings.TrimPrefix(u.Path, "/")

	params := u.Query()
	if params.Get("parseTime") == "" {
		params.Set("parseTime", "true")
	}

	return fmt.Sprintf("%s@tcp(%s)/%s?%s", userInfo, host, path, params.Encode())
}
