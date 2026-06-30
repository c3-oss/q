// Package mysql implements the read-only MySQL adapter. Read-only is enforced
// by a read-only transaction (layer 1) plus the SQL classifier (layer 2).
package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	"github.com/go-sql-driver/mysql"

	"github.com/c3-oss/q/internal/adapter"
	"github.com/c3-oss/q/internal/adapter/sqlcommon"
	"github.com/c3-oss/q/internal/readonly"
)

func init() { adapter.Register(Factory{}) }

// Factory opens read-only MySQL connections.
type Factory struct{}

func (Factory) Schemes() []string      { return []string{"mysql"} }
func (Factory) Family() adapter.Family { return adapter.Relational }

func (Factory) Open(ctx context.Context, raw string) (adapter.Adapter, error) {
	driverDSN, err := dsnFromURL(raw)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("mysql", driverDSN)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &conn{db: db}, nil
}

// dsnFromURL converts a mysql:// URL into the go-sql-driver DSN format
// (user:pass@tcp(host:port)/db?params), which the driver requires.
func dsnFromURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("mysql: parse connection string: %w", err)
	}

	cfg := mysql.NewConfig()
	cfg.Net = "tcp"
	cfg.Addr = u.Host
	if cfg.Addr != "" && !strings.Contains(cfg.Addr, ":") {
		cfg.Addr += ":3306"
	}
	if u.User != nil {
		cfg.User = u.User.Username()
		if p, ok := u.User.Password(); ok {
			cfg.Passwd = p
		}
	}
	cfg.DBName = strings.TrimPrefix(u.Path, "/")
	cfg.ParseTime = true
	cfg.Params = map[string]string{}
	for k, vs := range u.Query() {
		if len(vs) == 0 {
			continue
		}
		switch strings.ToLower(k) {
		case "tls":
			cfg.TLSConfig = vs[0]
		case "parsetime":
			// forced on for native time.Time values
		default:
			cfg.Params[k] = vs[0]
		}
	}
	return cfg.FormatDSN(), nil
}

type conn struct{ db *sql.DB }

func (c *conn) Ping(ctx context.Context) error { return c.db.PingContext(ctx) }
func (c *conn) Close() error                   { return c.db.Close() }

func (c *conn) Describe(ctx context.Context) string {
	var v string
	if err := c.db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&v); err != nil {
		return ""
	}
	return "MySQL " + v
}

func (c *conn) Query(ctx context.Context, query string) (adapter.Result, error) {
	if err := readonly.CheckSQL(query); err != nil {
		return nil, err
	}
	tx, err := c.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, query) //nolint:rowserrcheck // closed via Result.Close
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	return sqlcommon.NewResult(rows, tx.Rollback)
}
