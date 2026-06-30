// Package dynamodb implements the read-only DynamoDB adapter via PartiQL.
// Read-only is enforced by requiring a leading SELECT (layer 1 + 2 combined:
// the API only executes the given statement). Region and an optional custom
// endpoint come from the DSN; credentials come from the AWS credential chain.
package dynamodb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/c3-oss/q/internal/adapter"
	"github.com/c3-oss/q/internal/readonly"
)

func init() { adapter.Register(Factory{}) }

// Factory opens read-only DynamoDB clients.
type Factory struct{}

func (Factory) Schemes() []string      { return []string{"dynamodb", "dynamo", "ddb"} }
func (Factory) Family() adapter.Family { return adapter.WideColumn }

func (Factory) Open(ctx context.Context, raw string) (adapter.Adapter, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("dynamodb: parse connection string: %w", err)
	}
	region := u.Query().Get("region")
	local := u.Host != ""

	var loadOpts []func(*config.LoadOptions) error
	if region == "" && local {
		region = "us-east-1"
	}
	if region != "" {
		loadOpts = append(loadOpts, config.WithRegion(region))
	}
	if local {
		// DynamoDB Local still signs requests; supply placeholder credentials.
		loadOpts = append(loadOpts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider("dummy", "dummy", "")))
	}

	cfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, err
	}

	var clientOpts []func(*dynamodb.Options)
	if local {
		endpoint := "http://" + u.Host
		clientOpts = append(clientOpts, func(o *dynamodb.Options) {
			o.BaseEndpoint = aws.String(endpoint)
		})
	}
	return &conn{client: dynamodb.NewFromConfig(cfg, clientOpts...)}, nil
}

type conn struct{ client *dynamodb.Client }

func (c *conn) Ping(ctx context.Context) error {
	_, err := c.client.ListTables(ctx, &dynamodb.ListTablesInput{Limit: aws.Int32(1)})
	return err
}

func (c *conn) Close() error { return nil }

func (c *conn) Query(ctx context.Context, query string) (adapter.Result, error) {
	if err := checkSelect(query); err != nil {
		return nil, err
	}
	return &result{client: c.client, stmt: query}, nil
}

// checkSelect requires the PartiQL statement to begin with SELECT.
func checkSelect(query string) error {
	t := strings.TrimSpace(query)
	word := t
	if i := strings.IndexFunc(t, func(r rune) bool { return r == ' ' || r == '\t' || r == '\n' }); i >= 0 {
		word = t[:i]
	}
	if !strings.EqualFold(word, "SELECT") {
		if word == "" {
			word = "empty statement"
		}
		return readonly.Deny("'" + strings.ToUpper(word) + "' is not a read-only operation")
	}
	return nil
}

// result streams PartiQL pages lazily, following NextToken.
type result struct {
	client *dynamodb.Client
	stmt   string
	token  *string
	page   []map[string]types.AttributeValue
	idx    int
	done   bool
}

func (r *result) Next(ctx context.Context) (adapter.Record, bool, error) {
	for r.idx >= len(r.page) {
		if r.done {
			return nil, false, nil
		}
		out, err := r.client.ExecuteStatement(ctx, &dynamodb.ExecuteStatementInput{
			Statement: aws.String(r.stmt),
			NextToken: r.token,
		})
		if err != nil {
			return nil, false, err
		}
		r.page, r.idx = out.Items, 0
		if out.NextToken == nil {
			r.done = true
		} else {
			r.token = out.NextToken
		}
	}
	item := r.page[r.idx]
	r.idx++
	return itemToRecord(item), true, nil
}

func (r *result) Close() error { return nil }

// itemToRecord converts an item to a record with attributes sorted by name, so
// the header stays stable across pages despite DynamoDB's unordered attributes.
func itemToRecord(item map[string]types.AttributeValue) adapter.Record {
	keys := make([]string, 0, len(item))
	for k := range item {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	rec := make(adapter.Record, len(keys))
	for i, k := range keys {
		rec[i] = adapter.Field{Name: k, Value: convertAV(item[k])}
	}
	return rec
}

// convertAV walks an AttributeValue, emitting numbers as json.Number to keep
// full precision.
func convertAV(av types.AttributeValue) any {
	switch v := av.(type) {
	case *types.AttributeValueMemberS:
		return v.Value
	case *types.AttributeValueMemberN:
		return json.Number(v.Value)
	case *types.AttributeValueMemberBOOL:
		return v.Value
	case *types.AttributeValueMemberNULL:
		return nil
	case *types.AttributeValueMemberB:
		return v.Value
	case *types.AttributeValueMemberM:
		m := make(map[string]any, len(v.Value))
		for k, val := range v.Value {
			m[k] = convertAV(val)
		}
		return m
	case *types.AttributeValueMemberL:
		a := make([]any, len(v.Value))
		for i, val := range v.Value {
			a[i] = convertAV(val)
		}
		return a
	case *types.AttributeValueMemberSS:
		return v.Value
	case *types.AttributeValueMemberNS:
		a := make([]any, len(v.Value))
		for i, s := range v.Value {
			a[i] = json.Number(s)
		}
		return a
	case *types.AttributeValueMemberBS:
		return v.Value
	default:
		return nil
	}
}
