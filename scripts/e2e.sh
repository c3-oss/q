#!/bin/sh
# e2e.sh — end-to-end tests of the compiled `q` binary against real databases.
#
# Spins up Postgres, MySQL, MongoDB, and Redis via docker compose (seeded from
# testdata/e2e), then drives the actual CLI: it pipes a connection string on
# stdin and asserts that a read query streams the expected output and that a
# mutation is refused with exit code 5. SQLite runs from a temp file, no
# container. Requires Docker (compose v2) and Go.

set -eu

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"
Q="$ROOT/bin/q"
COMPOSE="docker compose"
SERVICES="postgres mysql mongo redis"

cleanup() {
    echo "── tearing down compose ──"
    $COMPOSE down -v --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

echo "── building q ──"
CGO_ENABLED=0 go build -o "$Q" ./cmd/q

echo "── starting databases (this pulls images on first run) ──"
# shellcheck disable=SC2086 # SERVICES must word-split into separate service args
$COMPOSE up -d --wait $SERVICES

echo "── seeding redis ──"
$COMPOSE exec -T redis redis-cli SET greeting hello >/dev/null
$COMPOSE exec -T redis redis-cli HSET user:1 name ada role admin >/dev/null

fail() {
    echo "FAIL [$1]: $2" >&2
    exit 1
}

# expect_select <name> <conn> <query> <needle>
expect_select() {
    printf '→ [%s] read query streams output\n' "$1"
    if ! out=$(printf '%s' "$2" | "$Q" "$3" 2>/dev/null); then
        fail "$1" "q exited non-zero on a read query"
    fi
    case "$out" in
        *"$4"*) printf '  ok: output contains %s\n' "$4" ;;
        *) fail "$1" "output missing '$4'; got: $out" ;;
    esac
}

# expect_refused <name> <conn> <mutation>
expect_refused() {
    printf '→ [%s] mutation is refused (exit 5)\n' "$1"
    set +e
    printf '%s' "$2" | "$Q" "$3" >/dev/null 2>&1
    rc=$?
    set -e
    [ "$rc" -eq 5 ] || fail "$1" "expected exit 5, got $rc"
    printf '  ok: exit 5\n'
}

PG="postgres://q:q@localhost:5432/app?sslmode=disable"
expect_select postgres "$PG" "select id, email from users order by id" "ada@example.com"
expect_refused postgres "$PG" "delete from users"

MYSQL="mysql://q:q@localhost:3306/app"
expect_select mysql "$MYSQL" "select sku, price from products order by sku" "150"
expect_refused mysql "$MYSQL" "update products set price = 0"

MONGO="mongodb://localhost:27017/app"
expect_select mongodb "$MONGO" 'events.find({"type":"signup"})' "ada"
expect_refused mongodb "$MONGO" 'events.insertOne({"type":"x"})'

REDIS="redis://localhost:6379/0"
expect_select redis "$REDIS" "GET greeting" "hello"
expect_refused redis "$REDIS" "SET greeting bye"

if command -v python3 >/dev/null 2>&1; then
    SQLITE_DIR=$(mktemp -d)
    SQLITE_DB="$SQLITE_DIR/e2e.db"
    python3 - "$SQLITE_DB" <<'PY'
import sqlite3, sys
c = sqlite3.connect(sys.argv[1])
c.execute("create table gauges (name text, value real)")
c.executemany("insert into gauges values (?, ?)", [("cpu", 0.5), ("mem", 0.8)])
c.commit(); c.close()
PY
    expect_select sqlite "sqlite://$SQLITE_DB" "select name, value from gauges order by name" "cpu"
    expect_refused sqlite "sqlite://$SQLITE_DB" "delete from gauges"
    rm -rf "$SQLITE_DIR"
else
    echo "→ [sqlite] skipped (python3 not available to seed)"
fi

echo "── all e2e checks passed ──"
