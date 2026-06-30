//go:build integration

package itest

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestMongoIntegration(t *testing.T) {
	ctx := context.Background()
	ctr, err := mongodb.Run(ctx, "mongo:7")
	require.NoError(t, err)
	defer func() { _ = testcontainers.TerminateContainer(ctr) }()

	base, err := ctr.ConnectionString(ctx)
	require.NoError(t, err)
	base = strings.TrimRight(base, "/")

	cli, err := mongo.Connect(options.Client().ApplyURI(base))
	require.NoError(t, err)
	_, err = cli.Database("itest").Collection("events").InsertMany(ctx, []any{
		bson.D{{Key: "type", Value: "signup"}, {Key: "user", Value: "ada"}},
		bson.D{{Key: "type", Value: "login"}, {Key: "user", Value: "alan"}},
		bson.D{{Key: "type", Value: "signup"}, {Key: "user", Value: "grace"}},
	})
	require.NoError(t, err)
	_ = cli.Disconnect(ctx)

	ad := open(t, ctx, base+"/itest")

	recs := readAll(t, ctx, ad, `events.find({"type":"signup"})`)
	require.Len(t, recs, 2)

	agg := readAll(t, ctx, ad, `events.aggregate([{"$match":{"type":"signup"}},{"$count":"n"}])`)
	require.Len(t, agg, 1)
	n, _ := fieldNamed(agg[0], "n")
	require.EqualValues(t, 2, n)

	assertRejected(t, ctx, ad, `events.insertOne({"type":"x"})`)
	assertRejected(t, ctx, ad, `events.aggregate([{"$out":"copy"}])`)
}
