//go:build integration

package itest

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestPostgresIntegration(t *testing.T) {
	ctx := context.Background()
	ctr, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("app"),
		postgres.WithUsername("ro"),
		postgres.WithPassword("secret"),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	defer func() { _ = testcontainers.TerminateContainer(ctr) }()

	conn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pg, err := pgx.Connect(ctx, conn)
	require.NoError(t, err)
	_, err = pg.Exec(ctx, `create table users (id int primary key, email text, prefs jsonb)`)
	require.NoError(t, err)
	_, err = pg.Exec(ctx, `insert into users values (1, 'ada@example.com', '{"theme":"dark"}')`)
	require.NoError(t, err)
	_ = pg.Close(ctx)

	ad := open(t, ctx, conn)

	recs := readAll(t, ctx, ad, "select id, email, prefs from users order by id")
	require.Len(t, recs, 1)
	require.Equal(t, "id", recs[0][0].Name)
	require.EqualValues(t, 1, recs[0][0].Value)
	email, _ := fieldNamed(recs[0], "email")
	require.Equal(t, "ada@example.com", email)
	// pgx decodes jsonb into a native Go map, so it renders as native JSON.
	prefs, _ := fieldNamed(recs[0], "prefs")
	m, ok := prefs.(map[string]any)
	require.True(t, ok, "jsonb should be native, got %T", prefs)
	require.Equal(t, "dark", m["theme"])

	assertRejected(t, ctx, ad, "delete from users")
	assertRejected(t, ctx, ad, "with x as (insert into users values (2,'b','{}') returning *) select * from x")
}
