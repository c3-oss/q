package dsn

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetectEngine(t *testing.T) {
	cases := []struct {
		name   string
		raw    string
		engine string
	}{
		{"postgres", "postgres://u:p@h:5432/db?sslmode=require", "postgres"},
		{"postgresql alias", "postgresql://u:p@h:5432/db", "postgres"},
		{"mysql", "mysql://u:p@h:3306/db?tls=true", "mysql"},
		{"mongodb", "mongodb://u:p@h:27017/db?authSource=admin", "mongodb"},
		{"mongodb+srv", "mongodb+srv://u:p@h/db", "mongodb"},
		{"mongo alias", "mongo://h:27017/db", "mongodb"},
		{"redis", "redis://:p@h:6379/0", "redis"},
		{"rediss tls", "rediss://:p@h:6379/0", "redis"},
		{"dynamodb", "dynamodb://?region=us-east-1", "dynamodb"},
		{"dynamo alias", "dynamo://localhost:8000?region=us-east-1", "dynamodb"},
		{"ddb alias", "ddb://?region=eu-west-1", "dynamodb"},
		{"sqlite scheme", "sqlite:///var/lib/app.db", "sqlite"},
		{"sqlite3 scheme", "sqlite3:///tmp/x.sqlite", "sqlite"},
		{"file scheme", "file:///tmp/data.db", "sqlite"},
		{"bare .db path", "/var/lib/metrics.db", "sqlite"},
		{"bare .sqlite path", "./local.sqlite", "sqlite"},
		{"bare .sqlite3 path", "data.sqlite3", "sqlite"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info, err := Detect(tc.raw)
			require.NoError(t, err)
			require.Equal(t, tc.engine, info.Engine)
		})
	}
}

func TestDetectUnknownScheme(t *testing.T) {
	for _, raw := range []string{
		"oracle://u:p@h/db",
		"cassandra://h:9042",
		"http://example.com",
		"",
		"   ",
		"just-some-text",
	} {
		_, err := Detect(raw)
		require.Error(t, err, "expected error for %q", raw)
	}
}

func TestSQLiteNormalization(t *testing.T) {
	info, err := Detect("sqlite:///var/lib/app.db")
	require.NoError(t, err)
	require.Equal(t, "sqlite", info.Engine)
	require.True(t, strings.HasPrefix(info.Normalized, "file:/var/lib/app.db"),
		"got %q", info.Normalized)
	require.Contains(t, info.Normalized, "mode=ro")
	require.Contains(t, info.Normalized, "_pragma=query_only(1)")
	require.Equal(t, "/var/lib/app.db", info.Host)
}

func TestSQLiteStripsQueryParams(t *testing.T) {
	info, err := Detect("sqlite:///tmp/app.db?cache=shared")
	require.NoError(t, err)
	require.Equal(t, "/tmp/app.db", info.Host)
	require.Contains(t, info.Normalized, "mode=ro")
}

func TestSQLiteBarePathIsAbsolute(t *testing.T) {
	info, err := Detect("./relative.db")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(info.Host, "/"), "expected absolute path, got %q", info.Host)
}

func TestURLEngineHostRedaction(t *testing.T) {
	info, err := Detect("postgres://user:secret@db.internal:5432/app")
	require.NoError(t, err)
	require.Equal(t, "db.internal:5432", info.Host)
	require.Equal(t, "postgres://user:secret@db.internal:5432/app", info.Normalized)
}

func TestDynamoHost(t *testing.T) {
	info, err := Detect("dynamodb://?region=us-east-1")
	require.NoError(t, err)
	require.Equal(t, "dynamodb.us-east-1.amazonaws.com", info.Host)

	local, err := Detect("dynamodb://localhost:8000?region=us-east-1")
	require.NoError(t, err)
	require.Equal(t, "localhost:8000", local.Host)
}
