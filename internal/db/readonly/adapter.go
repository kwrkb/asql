package readonly

import (
	"context"

	"github.com/kwrkb/asql/internal/db"
)

// Adapter wraps a db.DBAdapter and refuses writing statements passed to Query.
//
// Query is the single point every user-supplied statement reaches — the SQL
// editor, saved snippets, query history, the sidebar's generated SELECTs and
// the SQL the AI assistant writes into the editor all execute through it.
// Wrapping here therefore covers all of them without any of them knowing that
// readonly exists.
//
// Tables, Columns and Schema pass through unchecked: their statements are
// built inside the adapters, never by the user.
type Adapter struct {
	inner db.DBAdapter
}

// Wrap returns adapter guarded against writes. Wrapping an already-wrapped
// adapter returns it unchanged.
func Wrap(adapter db.DBAdapter) db.DBAdapter {
	if adapter == nil {
		return nil
	}
	if _, ok := adapter.(*Adapter); ok {
		return adapter
	}
	return &Adapter{inner: adapter}
}

// IsWrapped reports whether adapter is guarded.
func IsWrapped(adapter db.DBAdapter) bool {
	_, ok := adapter.(*Adapter)
	return ok
}

func (a *Adapter) Type() string { return a.inner.Type() }

func (a *Adapter) Query(ctx context.Context, query string) (db.QueryResult, error) {
	if err := Check(query); err != nil {
		return db.QueryResult{}, err
	}
	return a.inner.Query(ctx, query)
}

func (a *Adapter) Tables(ctx context.Context) ([]string, error) {
	return a.inner.Tables(ctx)
}

func (a *Adapter) Columns(ctx context.Context, tableName string) ([]string, error) {
	return a.inner.Columns(ctx, tableName)
}

func (a *Adapter) Schema(ctx context.Context) (string, error) {
	return a.inner.Schema(ctx)
}

func (a *Adapter) QuoteIdentifier(name string) string {
	return a.inner.QuoteIdentifier(name)
}

func (a *Adapter) Close() error { return a.inner.Close() }
