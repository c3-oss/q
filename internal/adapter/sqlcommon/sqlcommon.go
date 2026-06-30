// Package sqlcommon holds the database/sql row-streaming logic shared by the
// MySQL and SQLite adapters: it converts dynamic columns into ordered records,
// turning byte slices into text and JSON columns into native JSON.
package sqlcommon

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/c3-oss/q/internal/adapter"
)

// Result streams *sql.Rows as adapter records. cleanup runs on Close (used to
// roll back the read-only transaction).
type Result struct {
	rows    *sql.Rows
	cols    []string
	types   []*sql.ColumnType
	cleanup func() error
}

// NewResult builds a Result from open rows, capturing the column schema.
func NewResult(rows *sql.Rows, cleanup func() error) (*Result, error) {
	cols, err := rows.Columns()
	if err != nil {
		_ = rows.Close()
		return nil, err
	}
	types, _ := rows.ColumnTypes()
	return &Result{rows: rows, cols: cols, types: types, cleanup: cleanup}, nil
}

// Next yields the next row as an ordered Record.
func (r *Result) Next(_ context.Context) (adapter.Record, bool, error) {
	if !r.rows.Next() {
		return nil, false, r.rows.Err()
	}
	dest := make([]any, len(r.cols))
	ptrs := make([]any, len(r.cols))
	for i := range dest {
		ptrs[i] = &dest[i]
	}
	if err := r.rows.Scan(ptrs...); err != nil {
		return nil, false, err
	}
	rec := make(adapter.Record, len(r.cols))
	for i, name := range r.cols {
		rec[i] = adapter.Field{Name: name, Value: convert(dest[i], r.colType(i))}
	}
	return rec, true, nil
}

// Close closes the rows and runs cleanup, preferring the first error.
func (r *Result) Close() error {
	err := r.rows.Close()
	if r.cleanup != nil {
		if cerr := r.cleanup(); cerr != nil && err == nil {
			err = cerr
		}
	}
	return err
}

func (r *Result) colType(i int) *sql.ColumnType {
	if i < len(r.types) {
		return r.types[i]
	}
	return nil
}

// convert turns driver-native byte slices into text, or into native JSON for
// JSON-typed columns. database/sql clones the bytes when scanning into *any, so
// retaining them across rows is safe.
func convert(v any, ct *sql.ColumnType) any {
	b, ok := v.([]byte)
	if !ok {
		return v
	}
	if ct != nil {
		switch ct.DatabaseTypeName() {
		case "JSON", "JSONB":
			return json.RawMessage(b)
		}
	}
	return string(b)
}
