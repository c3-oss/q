//go:build integration

package itest

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
)

func TestMySQLIntegration(t *testing.T) {
	ctx := context.Background()
	ctr, err := mysql.Run(ctx, "mysql:8.0",
		mysql.WithDatabase("shop"),
		mysql.WithUsername("ro"),
		mysql.WithPassword("secret"),
	)
	require.NoError(t, err)
	defer func() { _ = testcontainers.TerminateContainer(ctr) }()

	driverDSN, err := ctr.ConnectionString(ctx)
	require.NoError(t, err)
	host, err := ctr.Host(ctx)
	require.NoError(t, err)
	port, err := ctr.MappedPort(ctx, "3306/tcp")
	require.NoError(t, err)
	qURL := fmt.Sprintf("mysql://ro:secret@%s:%s/shop", host, port.Port())

	db, err := sql.Open("mysql", driverDSN)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `create table products (sku varchar(32) primary key, price int, meta json)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `insert into products values ('a', 150, '{"x":1}')`)
	require.NoError(t, err)
	_ = db.Close()

	ad := open(t, ctx, qURL)

	recs := readAll(t, ctx, ad, "select sku, price, meta from products")
	require.Len(t, recs, 1)
	sku, _ := fieldNamed(recs[0], "sku")
	require.Equal(t, "a", sku)
	meta, _ := fieldNamed(recs[0], "meta")
	require.IsType(t, json.RawMessage{}, meta, "JSON column should stay native JSON")

	assertRejected(t, ctx, ad, "update products set price = 0")
}
