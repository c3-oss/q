# `q`

[![CI](https://github.com/c3-oss/q/actions/workflows/ci.yml/badge.svg)](https://github.com/c3-oss/q/actions/workflows/ci.yml)
[![Release](https://github.com/c3-oss/q/actions/workflows/release.yml/badge.svg)](https://github.com/c3-oss/q/actions/workflows/release.yml)
[![License: CC0 1.0](https://img.shields.io/badge/license-CC0%201.0-lightgrey.svg)](LICENSE)

> a read-only, multi-database query CLI

`q` connects to a database — auto-detected from the connection-string scheme —
runs a single query in the engine's native language, and streams the result to
stdout as CSV, JSON, or an aligned table.

```sh
echo "$CONN_STRING" | q -f json 'select id, email from users limit 10'
```

`q` is read-only. It never mutates data: it runs inside an engine read-only
session where the driver supports one, **and** classifies every statement and
rejects any mutation or DDL before execution.

## Install

```sh
go install github.com/c3-oss/q/cmd/q@latest
```

Or build from source: `just build` produces `bin/q`.

## Supported engines

The engine is detected from the connection-string scheme (aliases accepted).
Engine-specific options travel as URL query parameters and are passed to the
driver.

| Engine | Schemes | Example connection string |
| --- | --- | --- |
| Postgres | `postgres`, `postgresql` | `postgres://user:pass@host:5432/db?sslmode=require` |
| MySQL | `mysql` | `mysql://user:pass@host:3306/db?tls=true` |
| SQLite | `sqlite`, `sqlite3`, `file`, or a path ending in `.db`/`.sqlite`/`.sqlite3` | `sqlite:///var/lib/app.db` |
| MongoDB | `mongodb`, `mongodb+srv`, `mongo` | `mongodb://user:pass@host:27017/db?authSource=admin` |
| Redis | `redis`, `rediss` | `redis://:pass@host:6379/0` |
| DynamoDB | `dynamodb`, `dynamo`, `ddb` | `dynamodb://?region=us-east-1` |

Query syntax is native to each engine: SQL for Postgres/MySQL/SQLite, the
MongoDB query language for MongoDB, native commands for Redis, and PartiQL for
DynamoDB.

## Connection string and credentials

The connection string is a secret, so it is **never** a positional argument or
flag (argv leaks into shell history and `ps`). `q` resolves it in this order:

1. **stdin**, when piped or redirected (it wins when also set in the environment);
2. the **`Q_CONNECTION_STRING`** environment variable;
3. otherwise a usage error.

DynamoDB credentials are never in the URL — only the region and an optional
custom endpoint are. Credentials resolve through the standard AWS credential
chain (environment, `~/.aws`, SSO, IAM role).

## Output formats

Select a format with `-f`/`--format`. The default depends on the engine:
relational engines default to **CSV**, document/key-value/wide-column engines
default to **JSON**.

| Format | Behavior |
| --- | --- |
| `csv` | RFC 4180. The first record fixes the header. Non-scalar values embed as compact JSON. |
| `json` | A streamed array of objects with field order preserved; nested structures stay native. |
| `table` | Columns aligned for reading; non-scalar values embed as compact JSON. |

The first record fixes the header for CSV and table: fields absent from a later
record render empty, and extra fields are dropped with a single warning to
stderr. Use `-f json` for heterogeneous document results.

```sh
$ echo "$PG" | q -f csv 'select id, email, prefs from users limit 2'
id,email,prefs
1,ada@example.com,"{""theme"":""dark""}"
2,alan@example.com,"{""theme"":""light""}"

$ echo "$PG" | q -f json 'select id, email, prefs from users limit 2'
[{"id":1,"email":"ada@example.com","prefs":{"theme":"dark"}},
{"id":2,"email":"alan@example.com","prefs":{"theme":"light"}}]
```

## Read-only guarantee

Every adapter enforces read-only in two independent layers:

- **Engine layer**: a read-only transaction (Postgres, MySQL), a read-only
  connection (`mode=ro` + `query_only` for SQLite), an operation allowlist
  (MongoDB), a command write-flag check (Redis), or PartiQL `SELECT`-only
  (DynamoDB).
- **Classification layer**: each query is classified and any mutation or DDL is
  rejected before execution with a clear message, e.g.
  `q: refused: 'DELETE' is not a read-only operation`.

MongoDB has no read-only session; run `q` as a read-only database user for a
defense-in-depth engine layer.

## Commands

| Command | Purpose |
| --- | --- |
| `q [flags] <query>` | Detect the engine, connect read-only, run `<query>`, stream the result. |
| `q test-connection` | Resolve the connection string, connect, ping, and report success or failure. |
| `q version` | Print build metadata. |
| `q completion <shell>` | Generate a shell completion script. |

Flags: `-f`/`--format` (`csv`\|`json`\|`table`), `--timeout` (Go duration, e.g.
`30s`; `0` means no limit), and `--log-level` (logs go to stderr).

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | Success. |
| `1` | Unexpected or internal error. |
| `2` | Usage error (bad flags/args, no connection string, unknown scheme). |
| `3` | Connection or authentication failure. |
| `4` | Query execution error. |
| `5` | Read-only violation — the query was rejected as mutating or DDL. |

## Examples

```sh
# Postgres (default CSV), credentials via stdin
echo 'postgres://ro:secret@db.internal:5432/app?sslmode=require' \
  | q 'select id, email, created_at from users order by created_at desc limit 50'

# MySQL, JSON output, credentials via env
export Q_CONNECTION_STRING='mysql://ro:secret@db:3306/shop?tls=true'
q -f json 'select sku, price from products where price > 100'

# SQLite, table output, read-only file
echo 'sqlite:///var/lib/metrics.db' | q -f table 'select name, value from gauges'

# MongoDB — native query syntax, default JSON
echo 'mongodb://ro:secret@mongo:27017/analytics?authSource=admin' \
  | q 'events.aggregate([{"$match":{"type":"signup"}},{"$count":"n"}])'

# Redis — native command, default JSON
echo 'redis://:secret@cache:6379/0' | q 'HGETALL session:abc123'

# DynamoDB — PartiQL SELECT; region in URL, credentials via the AWS chain
echo 'dynamodb://?region=us-east-1' \
  | q -f json "SELECT id, email FROM \"Users\" WHERE id = 'u-42'"

# Connectivity / auth check (no query)
echo "$Q_CONNECTION_STRING" | q test-connection

# A mutation is always refused (exit 5), on every engine
echo "$PG" | q 'delete from users'   # → q: refused: 'DELETE' is not a read-only operation
```

## Build from source

This repo uses [devbox](https://www.jetify.com/devbox) for a pinned toolchain
and [`just`](https://github.com/casey/just) as the task runner.

```sh
devbox shell        # enter the toolchain (Go, golangci-lint, gosec, …)
just build          # build bin/q
just test           # unit tests (fast, no containers)
just test-integration   # testcontainers suite (requires Docker)
just ci             # the full local PR pipeline
```

Integration tests run each engine in an ephemeral container via
[testcontainers-go](https://golang.testcontainers.org/) behind the
`integration` build tag; SQLite uses a temporary file.

## License

`q` is released into the public domain under [CC0 1.0](LICENSE).

— Caian Ertl &lt;<https://github.com/upsetbit>&gt;

```text
              ___
       ^C ^V /   \
```
