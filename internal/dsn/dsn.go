// Package dsn detects a database engine from a connection-string scheme and
// normalizes the string into a form each adapter can consume.
package dsn

import (
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
)

// Info is the result of detecting and normalizing a connection string.
type Info struct {
	// Engine is the canonical engine name and the key used for adapter lookup
	// (postgres, mysql, sqlite, mongodb, redis, dynamodb).
	Engine string
	// Normalized is the connection string in the form the adapter expects.
	Normalized string
	// Host is a credential-free endpoint shown by test-connection.
	Host string
}

// schemeEngine maps every accepted scheme alias to its canonical engine.
var schemeEngine = map[string]string{
	"postgres":    "postgres",
	"postgresql":  "postgres",
	"mysql":       "mysql",
	"sqlite":      "sqlite",
	"sqlite3":     "sqlite",
	"file":        "sqlite",
	"mongodb":     "mongodb",
	"mongodb+srv": "mongodb",
	"mongo":       "mongodb",
	"redis":       "redis",
	"rediss":      "redis",
	"dynamodb":    "dynamodb",
	"dynamo":      "dynamodb",
	"ddb":         "dynamodb",
}

// Detect resolves the engine and normalizes the connection string. An unknown
// or unsupported scheme is a usage error.
func Detect(raw string) (Info, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return Info{}, fmt.Errorf("empty connection string")
	}

	scheme, hasScheme := schemeOf(s)
	if !hasScheme {
		if isBareSQLitePath(s) {
			return sqliteInfo(s)
		}
		return Info{}, fmt.Errorf(
			"cannot detect engine from %q: no scheme and not a SQLite path; supported schemes: %s",
			s, strings.Join(supportedSchemes(), ", "))
	}

	engine, ok := schemeEngine[scheme]
	if !ok {
		return Info{}, fmt.Errorf("unsupported scheme %q; supported schemes: %s",
			scheme, strings.Join(supportedSchemes(), ", "))
	}

	switch engine {
	case "sqlite":
		return sqliteInfo(s)
	case "dynamodb":
		return dynamoInfo(s)
	default:
		return urlInfo(engine, s)
	}
}

// schemeOf returns the lowercased scheme of a URL-style DSN.
func schemeOf(s string) (string, bool) {
	i := strings.Index(s, "://")
	if i > 0 {
		return strings.ToLower(s[:i]), true
	}
	// Opaque forms such as "file:rel.db" or "sqlite:app.db".
	if j := strings.Index(s, ":"); j > 0 {
		head := strings.ToLower(s[:j])
		if _, ok := schemeEngine[head]; ok {
			return head, true
		}
	}
	return "", false
}

func isBareSQLitePath(s string) bool {
	lower := strings.ToLower(s)
	for _, ext := range []string{".db", ".sqlite", ".sqlite3"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// sqliteInfo normalizes any SQLite input into a read-only file: URI.
func sqliteInfo(s string) (Info, error) {
	path := sqlitePath(s)
	if path == "" {
		return Info{}, fmt.Errorf("sqlite: empty database path in %q", s)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return Info{}, fmt.Errorf("sqlite: resolve path %q: %w", path, err)
	}
	normalized := "file:" + abs + "?mode=ro&_pragma=query_only(1)"
	return Info{Engine: "sqlite", Normalized: normalized, Host: abs}, nil
}

// sqlitePath strips any sqlite/file scheme prefix and returns the file path.
func sqlitePath(s string) string {
	for _, p := range []string{"sqlite3://", "sqlite://", "file://", "sqlite3:", "sqlite:", "file:"} {
		if rest, ok := strings.CutPrefix(s, p); ok {
			path, _, _ := strings.Cut(rest, "?")
			return path
		}
	}
	path, _, _ := strings.Cut(s, "?")
	return path
}

// urlInfo handles engines whose driver accepts the URL more or less directly.
func urlInfo(engine, s string) (Info, error) {
	u, err := url.Parse(s)
	if err != nil {
		return Info{}, fmt.Errorf("%s: parse connection string: %w", engine, err)
	}
	return Info{Engine: engine, Normalized: s, Host: u.Host}, nil
}

// dynamoInfo extracts the region and optional custom endpoint. Credentials
// never travel in the URL; they resolve through the AWS credential chain.
func dynamoInfo(s string) (Info, error) {
	u, err := url.Parse(s)
	if err != nil {
		return Info{}, fmt.Errorf("dynamodb: parse connection string: %w", err)
	}
	host := u.Host
	if host == "" {
		host = "dynamodb." + u.Query().Get("region") + ".amazonaws.com"
	}
	return Info{Engine: "dynamodb", Normalized: s, Host: host}, nil
}

func supportedSchemes() []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(schemeEngine))
	for s := range schemeEngine {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
