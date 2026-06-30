//go:build integration

package itest

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestSQLiteIntegration(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "metrics.db")

	db, err := sql.Open("sqlite", path) // writable for seeding
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `create table gauges (name text, value real)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `insert into gauges values ('cpu', 0.5), ('mem', 0.8)`)
	require.NoError(t, err)
	_ = db.Close()

	ad := open(t, ctx, "sqlite://"+path)

	recs := readAll(t, ctx, ad, "select name, value from gauges order by name")
	require.Len(t, recs, 2)
	name, _ := fieldNamed(recs[0], "name")
	require.Equal(t, "cpu", name)

	assertRejected(t, ctx, ad, "delete from gauges")
	assertRejected(t, ctx, ad, "drop table gauges")
}
