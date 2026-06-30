// Package postgres implements the read-only Postgres adapter using pgx/v5.
// Read-only is enforced by a read-only transaction (layer 1) plus the SQL
// classifier (layer 2).
package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/c3-oss/q/internal/adapter"
	"github.com/c3-oss/q/internal/readonly"
)

func init() { adapter.Register(Factory{}) }

// Factory opens read-only Postgres connections.
type Factory struct{}

func (Factory) Schemes() []string      { return []string{"postgres", "postgresql"} }
func (Factory) Family() adapter.Family { return adapter.Relational }

func (Factory) Open(ctx context.Context, dsn string) (adapter.Adapter, error) {
	pg, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pg.Ping(ctx); err != nil {
		_ = pg.Close(ctx)
		return nil, err
	}
	return &conn{pg: pg}, nil
}

type conn struct{ pg *pgx.Conn }

func (c *conn) Ping(ctx context.Context) error { return c.pg.Ping(ctx) }
func (c *conn) Close() error                   { return c.pg.Close(context.Background()) }

func (c *conn) Describe(ctx context.Context) string {
	var v string
	if err := c.pg.QueryRow(ctx, "SHOW server_version").Scan(&v); err != nil {
		return ""
	}
	return "PostgreSQL " + v
}

func (c *conn) Query(ctx context.Context, query string) (adapter.Result, error) {
	if err := readonly.CheckSQL(query); err != nil {
		return nil, err
	}
	tx, err := c.pg.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, query)
	if err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}
	return &result{rows: rows, tx: tx}, nil
}

type result struct {
	rows pgx.Rows
	tx   pgx.Tx
}

func (r *result) Next(_ context.Context) (adapter.Record, bool, error) {
	if !r.rows.Next() {
		return nil, false, r.rows.Err()
	}
	vals, err := r.rows.Values()
	if err != nil {
		return nil, false, err
	}
	fds := r.rows.FieldDescriptions()
	rec := make(adapter.Record, len(fds))
	for i, fd := range fds {
		rec[i] = adapter.Field{Name: fd.Name, Value: convert(vals[i], fd.DataTypeOID)}
	}
	return rec, true, nil
}

func (r *result) Close() error {
	r.rows.Close()
	return r.tx.Rollback(context.Background())
}

// convert wraps json/jsonb bytes as native JSON; other values pass through.
func convert(v any, oid uint32) any {
	if b, ok := v.([]byte); ok && (oid == pgtype.JSONOID || oid == pgtype.JSONBOID) {
		return json.RawMessage(b)
	}
	return v
}
