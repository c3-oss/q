// Package mongo implements the read-only MongoDB adapter. MongoDB has no
// read-only session, so layer 1 is an operation allowlist (only find,
// aggregate, countDocuments, distinct; $out/$merge rejected) and a read-only
// database user is the recommended deployment guard.
package mongo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"

	"github.com/c3-oss/q/internal/adapter"
	"github.com/c3-oss/q/internal/readonly"
)

func init() { adapter.Register(Factory{}) }

// Factory opens read-only MongoDB connections.
type Factory struct{}

func (Factory) Schemes() []string      { return []string{"mongodb", "mongodb+srv", "mongo"} }
func (Factory) Family() adapter.Family { return adapter.Document }

func (Factory) Open(ctx context.Context, uri string) (adapter.Adapter, error) {
	db, err := databaseName(uri)
	if err != nil {
		return nil, err
	}
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, err
	}
	return &conn{client: client, db: db}, nil
}

func databaseName(uri string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", fmt.Errorf("mongo: parse connection string: %w", err)
	}
	db := strings.TrimPrefix(u.Path, "/")
	if db == "" {
		return "", fmt.Errorf("mongo: no database in connection string path")
	}
	return db, nil
}

type conn struct {
	client *mongo.Client
	db     string
}

func (c *conn) Ping(ctx context.Context) error { return c.client.Ping(ctx, readpref.Primary()) }
func (c *conn) Close() error                   { return c.client.Disconnect(context.Background()) }

func (c *conn) Query(ctx context.Context, q string) (adapter.Result, error) {
	pq, err := parse(q)
	if err != nil {
		return nil, err
	}
	coll := c.client.Database(c.db).Collection(pq.Collection)

	switch pq.Method {
	case "find":
		return c.find(ctx, coll, pq.Args)
	case "aggregate":
		return c.aggregate(ctx, coll, pq.Args)
	case "countDocuments":
		return c.count(ctx, coll, pq.Args)
	case "distinct":
		return c.distinct(ctx, coll, pq.Args)
	default:
		return nil, readonly.Deny("'" + pq.Method + "' is not a read-only operation")
	}
}

func (c *conn) find(ctx context.Context, coll *mongo.Collection, args []string) (adapter.Result, error) {
	filter, err := extJSON(args, 0, "find filter")
	if err != nil {
		return nil, err
	}
	opts := options.Find()
	if len(args) >= 2 && args[1] != "" {
		var proj bson.D
		if err := bson.UnmarshalExtJSON([]byte(args[1]), false, &proj); err != nil {
			return nil, fmt.Errorf("find projection: %w", err)
		}
		opts.SetProjection(proj)
	}
	cur, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	return &cursorResult{cur: cur}, nil
}

func (c *conn) aggregate(ctx context.Context, coll *mongo.Collection, args []string) (adapter.Result, error) {
	if len(args) == 0 || args[0] == "" {
		return nil, fmt.Errorf("aggregate requires a pipeline")
	}
	var pipeline []bson.D
	if err := bson.UnmarshalExtJSON([]byte(args[0]), false, &pipeline); err != nil {
		return nil, fmt.Errorf("aggregate pipeline: %w", err)
	}
	for _, stage := range pipeline {
		if len(stage) > 0 {
			if op := stage[0].Key; op == "$out" || op == "$merge" {
				return nil, readonly.Deny("aggregation stage '" + op + "' writes and is not a read-only operation")
			}
		}
	}
	cur, err := coll.Aggregate(ctx, mongo.Pipeline(pipeline))
	if err != nil {
		return nil, err
	}
	return &cursorResult{cur: cur}, nil
}

func (c *conn) count(ctx context.Context, coll *mongo.Collection, args []string) (adapter.Result, error) {
	filter, err := extJSON(args, 0, "countDocuments filter")
	if err != nil {
		return nil, err
	}
	n, err := coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, err
	}
	return &adapter.SliceResult{Records: []adapter.Record{{{Name: "count", Value: n}}}}, nil
}

func (c *conn) distinct(ctx context.Context, coll *mongo.Collection, args []string) (adapter.Result, error) {
	if len(args) == 0 || args[0] == "" {
		return nil, fmt.Errorf("distinct requires a field name")
	}
	var field string
	if err := json.Unmarshal([]byte(args[0]), &field); err != nil {
		return nil, fmt.Errorf("distinct field: %w", err)
	}
	filter, err := extJSON(args, 1, "distinct filter")
	if err != nil {
		return nil, err
	}
	var vals []any
	if err := coll.Distinct(ctx, field, filter).Decode(&vals); err != nil {
		return nil, err
	}
	recs := make([]adapter.Record, len(vals))
	for i, v := range vals {
		recs[i] = adapter.Record{{Name: field, Value: convertBSON(v)}}
	}
	return &adapter.SliceResult{Records: recs}, nil
}

// extJSON parses the argument at index i as an Extended JSON document, defaulting
// to an empty filter when the argument is absent.
func extJSON(args []string, i int, what string) (bson.D, error) {
	doc := bson.D{}
	if len(args) > i && args[i] != "" {
		if err := bson.UnmarshalExtJSON([]byte(args[i]), false, &doc); err != nil {
			return nil, fmt.Errorf("%s: %w", what, err)
		}
	}
	return doc, nil
}

type cursorResult struct{ cur *mongo.Cursor }

func (r *cursorResult) Next(ctx context.Context) (adapter.Record, bool, error) {
	if !r.cur.Next(ctx) {
		return nil, false, r.cur.Err()
	}
	var doc bson.D
	if err := r.cur.Decode(&doc); err != nil {
		return nil, false, err
	}
	return docToRecord(doc), true, nil
}

func (r *cursorResult) Close() error { return r.cur.Close(context.Background()) }

func docToRecord(d bson.D) adapter.Record {
	rec := make(adapter.Record, len(d))
	for i, e := range d {
		rec[i] = adapter.Field{Name: e.Key, Value: convertBSON(e.Value)}
	}
	return rec
}

// convertBSON turns BSON values into JSON-friendly Go values. Nested documents
// become maps (top-level field order is preserved by the Record).
func convertBSON(v any) any {
	switch x := v.(type) {
	case bson.D:
		m := make(map[string]any, len(x))
		for _, e := range x {
			m[e.Key] = convertBSON(e.Value)
		}
		return m
	case bson.M:
		m := make(map[string]any, len(x))
		for k, val := range x {
			m[k] = convertBSON(val)
		}
		return m
	case bson.A:
		a := make([]any, len(x))
		for i, e := range x {
			a[i] = convertBSON(e)
		}
		return a
	case bson.ObjectID:
		return x.Hex()
	case bson.DateTime:
		return x.Time().UTC().Format(time.RFC3339Nano)
	case bson.Decimal128:
		return x.String()
	case bson.Binary:
		return x.Data
	default:
		return v
	}
}
