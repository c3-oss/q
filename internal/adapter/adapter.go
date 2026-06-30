// Package adapter defines the contract every database backend implements and
// the registry that maps a connection-string scheme to a backend factory.
package adapter

import "context"

// Family selects the default output format and the read-only strategy class.
type Family int

const (
	// Relational covers Postgres, MySQL, and SQLite. Default format: CSV.
	Relational Family = iota
	// Document covers MongoDB. Default format: JSON.
	Document
	// KeyValue covers Redis. Default format: JSON.
	KeyValue
	// WideColumn covers DynamoDB. Default format: JSON.
	WideColumn
)

// Factory builds a connected, read-only Adapter from a normalized DSN.
type Factory interface {
	// Schemes lists the connection-string schemes this factory handles.
	Schemes() []string
	// Family reports the engine class, which selects the default format.
	Family() Family
	// Open connects and returns a live, read-only Adapter.
	Open(ctx context.Context, dsn string) (Adapter, error)
}

// Adapter is a live, read-only connection to one database.
type Adapter interface {
	// Ping verifies connectivity and authentication.
	Ping(ctx context.Context) error
	// Query rejects any mutating or DDL operation, then streams the result.
	Query(ctx context.Context, query string) (Result, error)
	// Close releases the underlying connection.
	Close() error
}

// Result is a forward-only, streaming cursor.
type Result interface {
	// Next yields the next record; ok=false signals the end of the stream.
	Next(ctx context.Context) (rec Record, ok bool, err error)
	// Close releases cursor resources.
	Close() error
}

// Describer is an optional interface an Adapter may implement to report a
// human-readable endpoint and server version for test-connection.
type Describer interface {
	// Describe returns a short detail string, e.g. "host:5432 (PostgreSQL 16.2)".
	Describe(ctx context.Context) string
}

// Record is an ordered list of named fields. Order is significant for CSV and
// Table, and the field set of the first record establishes the header.
type Record []Field

// Field is one named value within a Record. Value is a scalar, or
// map[string]any / []any / json.RawMessage / json.Number for richer types.
type Field struct {
	Name  string
	Value any
}
