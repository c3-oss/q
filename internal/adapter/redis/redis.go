// Package redis implements the read-only Redis adapter using go-redis. Read-only
// is enforced by a hard denylist of administrative commands plus a COMMAND INFO
// write-flag check (layer 1) — with a static read allowlist fallback.
package redis

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/redis/go-redis/v9"

	"github.com/c3-oss/q/internal/adapter"
	"github.com/c3-oss/q/internal/readonly"
)

func init() { adapter.Register(Factory{}) }

// denylist holds administrative or scripting commands rejected regardless of
// their flags.
var denylist = map[string]bool{
	"FLUSHALL": true, "FLUSHDB": true, "CONFIG": true, "SHUTDOWN": true,
	"DEBUG": true, "SAVE": true, "BGSAVE": true, "BGREWRITEAOF": true,
	"SCRIPT": true, "EVAL": true, "EVALSHA": true, "FUNCTION": true, "FCALL": true,
}

// readAllowlist backs the fallback when COMMAND INFO is unavailable.
var readAllowlist = map[string]bool{
	"GET": true, "MGET": true, "STRLEN": true, "GETRANGE": true, "GETBIT": true,
	"EXISTS": true, "TYPE": true, "TTL": true, "PTTL": true, "KEYS": true, "SCAN": true,
	"DBSIZE": true, "RANDOMKEY": true, "DUMP": true, "OBJECT": true, "MEMORY": true,
	"HGET": true, "HGETALL": true, "HMGET": true, "HKEYS": true, "HVALS": true,
	"HLEN": true, "HEXISTS": true, "HSCAN": true, "HSTRLEN": true,
	"LRANGE": true, "LLEN": true, "LINDEX": true, "LPOS": true,
	"SMEMBERS": true, "SCARD": true, "SISMEMBER": true, "SMISMEMBER": true,
	"SSCAN": true, "SRANDMEMBER": true, "SINTER": true, "SUNION": true, "SDIFF": true,
	"ZRANGE": true, "ZREVRANGE": true, "ZRANGEBYSCORE": true, "ZRANGEBYLEX": true,
	"ZCARD": true, "ZSCORE": true, "ZMSCORE": true, "ZSCAN": true, "ZCOUNT": true,
	"ZRANK": true, "ZREVRANK": true, "BITCOUNT": true, "BITPOS": true,
	"PING": true, "INFO": true, "GETEX": true,
}

// Factory opens read-only Redis connections.
type Factory struct{}

func (Factory) Schemes() []string      { return []string{"redis", "rediss"} }
func (Factory) Family() adapter.Family { return adapter.KeyValue }

func (Factory) Open(ctx context.Context, uri string) (adapter.Adapter, error) {
	opts, err := redis.ParseURL(uri)
	if err != nil {
		return nil, fmt.Errorf("redis: parse connection string: %w", err)
	}
	opts.Protocol = 2 // stable RESP2 reply shapes
	client := redis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return &conn{client: client}, nil
}

type conn struct{ client *redis.Client }

func (c *conn) Ping(ctx context.Context) error { return c.client.Ping(ctx).Err() }
func (c *conn) Close() error                   { return c.client.Close() }

func (c *conn) Query(ctx context.Context, command string) (adapter.Result, error) {
	toks, err := tokenize(command)
	if err != nil {
		return nil, err
	}
	if len(toks) == 0 {
		return nil, fmt.Errorf("empty command")
	}
	name := strings.ToUpper(toks[0])
	if err := c.guard(ctx, name); err != nil {
		return nil, err
	}

	args := make([]any, len(toks))
	for i, t := range toks {
		args[i] = t
	}
	reply, err := c.client.Do(ctx, args...).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			reply = nil
		} else {
			return nil, err
		}
	}
	rec := adapter.Record{{Name: "result", Value: convertReply(reply)}}
	return &adapter.SliceResult{Records: []adapter.Record{rec}}, nil
}

// guard rejects administrative commands and any command whose server metadata
// marks it as a writer, falling back to a static read allowlist.
func (c *conn) guard(ctx context.Context, name string) error {
	if denylist[name] {
		return readonly.Deny("'" + name + "' is not a read-only operation")
	}
	infos, err := c.client.Command(ctx).Result()
	if err != nil {
		if readAllowlist[name] {
			return nil
		}
		return readonly.Deny("'" + name + "' is not in the read-only allowlist")
	}
	info, ok := infos[strings.ToLower(name)]
	if !ok {
		return readonly.Deny("unknown command '" + name + "'")
	}
	if slices.Contains(info.Flags, "write") || !info.ReadOnly {
		return readonly.Deny("'" + name + "' is not a read-only operation")
	}
	return nil
}

func convertReply(v any) any {
	switch x := v.(type) {
	case []byte:
		return string(x)
	case []any:
		a := make([]any, len(x))
		for i, e := range x {
			a[i] = convertReply(e)
		}
		return a
	case map[any]any:
		m := make(map[string]any, len(x))
		for k, val := range x {
			m[fmt.Sprint(k)] = convertReply(val)
		}
		return m
	default:
		return v
	}
}

// tokenize splits a command into arguments, honoring single and double quotes.
func tokenize(s string) ([]string, error) {
	var toks []string
	var cur strings.Builder
	var quote rune
	started := false
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
			started = true
		case unicode.IsSpace(r):
			if started || cur.Len() > 0 {
				toks = append(toks, cur.String())
				cur.Reset()
				started = false
			}
		default:
			cur.WriteRune(r)
			started = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unbalanced quote in command")
	}
	if started || cur.Len() > 0 {
		toks = append(toks, cur.String())
	}
	return toks, nil
}
