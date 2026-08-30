package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	gomysql "github.com/go-sql-driver/mysql"

	"github.com/kwrkb/asql/internal/db"
	"github.com/kwrkb/asql/internal/db/dbutil"
)

type Adapter struct {
	conn *sql.DB
}

// Open connects to a MySQL database using the given DSN.
// Accepts both mysql:// URL format and go-sql-driver's own DSN format.
func Open(dsn string) (*Adapter, error) {
	cfg, err := buildConfig(dsn)
	if err != nil {
		return nil, err
	}

	connector, err := gomysql.NewConnector(cfg)
	if err != nil {
		return nil, err
	}
	conn := sql.OpenDB(connector)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.PingContext(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}

	conn.SetMaxOpenConns(5)
	conn.SetMaxIdleConns(2)
	conn.SetConnMaxLifetime(5 * time.Minute)

	return &Adapter{conn: conn}, nil
}

func (a *Adapter) Type() string { return "mysql" }

func (a *Adapter) QuoteIdentifier(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

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

func (a *Adapter) Columns(ctx context.Context, tableName string) ([]string, error) {
	quoted := a.QuoteIdentifier(tableName)
	rows, err := a.conn.QueryContext(ctx, "SHOW COLUMNS FROM "+quoted)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var field, colType, null, key string
		var dflt *string
		var extra string
		if err := rows.Scan(&field, &colType, &null, &key, &dflt, &extra); err != nil {
			return nil, err
		}
		cols = append(cols, field)
	}
	return cols, rows.Err()
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

// buildConfig turns a DSN — either a mysql:// URL or go-sql-driver's own
// format — into a driver config.
//
// The credentials are set as struct fields rather than written into a DSN
// string, because the string form cannot express all of them. go-sql-driver
// splits `user:password` on the *first* colon and has no escape for one inside
// a username, so `mysql://first%3Alast:secret@host/db` would come back as user
// "first" with password "last:secret" — a silent swap to different, sometimes
// valid, credentials. Everything else still goes through the driver's own
// parser, so parameter handling (parseTime, loc, tls, ...) stays its business.
func buildConfig(dsn string) (*gomysql.Config, error) {
	if !strings.HasPrefix(dsn, "mysql://") {
		return gomysql.ParseDSN(dsn)
	}

	u, err := url.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("parsing MySQL URL: %w", err)
	}

	host := u.Host
	if host == "" {
		host = "127.0.0.1:3306"
	}

	params := u.Query()
	if params.Get("parseTime") == "" {
		params.Set("parseTime", "true")
	}

	// The dbname keeps its escaped form: ParseDSN runs url.PathUnescape on it,
	// so handing over the already-decoded path would decode it twice.
	cfg, err := gomysql.ParseDSN(fmt.Sprintf("tcp(%s)/%s?%s",
		host, strings.TrimPrefix(u.EscapedPath(), "/"), params.Encode()))
	if err != nil {
		return nil, err
	}

	// Decoded, and past the parser: url.Userinfo.String would re-encode these
	// for URL use and the driver does not percent-decode them.
	if u.User != nil {
		cfg.User = u.User.Username()
		cfg.Passwd, _ = u.User.Password()
	}
	return cfg, nil
}
