// Package sqlite implements the read-only SQLite adapter using the pure-Go
// modernc.org/sqlite driver. Read-only is enforced by the mode=ro file URI and
// the query_only pragma (layer 1) plus the SQL classifier (layer 2).
package sqlite

import (
	"context"
	"database/sql"

	_ "modernc.org/sqlite" // registers the "sqlite" driver

	"github.com/c3-oss/q/internal/adapter"
	"github.com/c3-oss/q/internal/adapter/sqlcommon"
	"github.com/c3-oss/q/internal/readonly"
)

func init() { adapter.Register(Factory{}) }

// Factory opens read-only SQLite connections.
type Factory struct{}

func (Factory) Schemes() []string      { return []string{"sqlite", "sqlite3", "file"} }
func (Factory) Family() adapter.Family { return adapter.Relational }

func (Factory) Open(ctx context.Context, dsn string) (adapter.Adapter, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &conn{db: db}, nil
}

type conn struct{ db *sql.DB }

func (c *conn) Ping(ctx context.Context) error { return c.db.PingContext(ctx) }
func (c *conn) Close() error                   { return c.db.Close() }

func (c *conn) Query(ctx context.Context, query string) (adapter.Result, error) {
	if err := readonly.CheckSQL(query); err != nil {
		return nil, err
	}
	rows, err := c.db.QueryContext(ctx, query) //nolint:rowserrcheck // closed via Result.Close
	if err != nil {
		return nil, err
	}
	return sqlcommon.NewResult(rows, nil)
}
