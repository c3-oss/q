//go:build integration

// Package itest holds end-to-end adapter tests that run real databases in
// ephemeral containers (testcontainers-go). Build and run them with:
//
//	go test -tags=integration -race ./...
package itest

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/c3-oss/q/internal/adapter"
	"github.com/c3-oss/q/internal/dsn"
	"github.com/c3-oss/q/internal/readonly"

	// Register every backend so adapter.Lookup resolves by scheme.
	_ "github.com/c3-oss/q/internal/adapter/dynamodb"
	_ "github.com/c3-oss/q/internal/adapter/mongo"
	_ "github.com/c3-oss/q/internal/adapter/mysql"
	_ "github.com/c3-oss/q/internal/adapter/postgres"
	_ "github.com/c3-oss/q/internal/adapter/redis"
	_ "github.com/c3-oss/q/internal/adapter/sqlite"
)

// open resolves a connection string and opens its adapter, registering cleanup.
func open(t *testing.T, ctx context.Context, conn string) adapter.Adapter {
	t.Helper()
	info, err := dsn.Detect(conn)
	require.NoError(t, err)
	f, ok := adapter.Lookup(info.Engine)
	require.True(t, ok, "no adapter for engine %q", info.Engine)
	ad, err := f.Open(ctx, info.Normalized)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ad.Close() })
	return ad
}

// readAll runs a read query and collects every streamed record.
func readAll(t *testing.T, ctx context.Context, ad adapter.Adapter, query string) []adapter.Record {
	t.Helper()
	res, err := ad.Query(ctx, query)
	require.NoError(t, err)
	defer func() { _ = res.Close() }()

	var recs []adapter.Record
	for {
		rec, ok, err := res.Next(ctx)
		require.NoError(t, err)
		if !ok {
			break
		}
		recs = append(recs, rec)
	}
	return recs
}

// assertRejected asserts a mutating query is refused as a read-only violation
// (which the CLI maps to exit 5).
func assertRejected(t *testing.T, ctx context.Context, ad adapter.Adapter, mutation string) {
	t.Helper()
	_, err := ad.Query(ctx, mutation)
	require.Error(t, err, "mutation should be rejected: %s", mutation)
	var v *readonly.Violation
	require.True(t, errors.As(err, &v), "expected read-only violation, got %v", err)
}

// fieldNamed returns the value of the named field in a record.
func fieldNamed(rec adapter.Record, name string) (any, bool) {
	for _, f := range rec {
		if f.Name == name {
			return f.Value, true
		}
	}
	return nil, false
}
