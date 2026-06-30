//go:build integration

package itest

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

func TestRedisIntegration(t *testing.T) {
	ctx := context.Background()
	ctr, err := tcredis.Run(ctx, "redis:7-alpine")
	require.NoError(t, err)
	defer func() { _ = testcontainers.TerminateContainer(ctr) }()

	conn, err := ctr.ConnectionString(ctx)
	require.NoError(t, err)

	opts, err := redis.ParseURL(conn)
	require.NoError(t, err)
	cli := redis.NewClient(opts)
	require.NoError(t, cli.Set(ctx, "greeting", "hello", 0).Err())
	require.NoError(t, cli.HSet(ctx, "user:1", "name", "ada", "role", "admin").Err())
	_ = cli.Close()

	ad := open(t, ctx, conn)

	recs := readAll(t, ctx, ad, "GET greeting")
	require.Len(t, recs, 1)
	require.Equal(t, "hello", recs[0][0].Value)

	hash := readAll(t, ctx, ad, "HGETALL user:1")
	require.Len(t, hash, 1)

	assertRejected(t, ctx, ad, "SET greeting bye")
	assertRejected(t, ctx, ad, "FLUSHALL")
}
