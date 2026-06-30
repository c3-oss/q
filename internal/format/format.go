// Package format renders streamed records as CSV, JSON, or an aligned table.
// Records are written as they arrive so memory stays flat regardless of size.
package format

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/c3-oss/q/internal/adapter"
)

// Formatter renders records to an output stream.
type Formatter interface {
	// Write renders one record.
	Write(rec adapter.Record) error
	// Close flushes any buffered output and finalizes the stream.
	Close() error
}

// Names lists the selectable format names.
var Names = []string{"csv", "json", "table"}

// New builds a Formatter for the given format name. An empty name selects the
// engine's default (Relational → csv, everything else → json). out receives the
// rendered output; warn receives the single dropped-fields warning.
func New(name string, fam adapter.Family, out, warn io.Writer) (Formatter, error) {
	if name == "" {
		name = defaultFor(fam)
	}
	switch name {
	case "csv":
		return newCSV(out, warn), nil
	case "json":
		return newJSON(out), nil
	case "table":
		return newTable(out, warn), nil
	default:
		return nil, fmt.Errorf("unknown format %q (want csv, json, or table)", name)
	}
}

func defaultFor(fam adapter.Family) string {
	if fam == adapter.Relational {
		return "csv"
	}
	return "json"
}

// cell renders a value as a single CSV/Table cell. Scalars render directly;
// everything else becomes compact JSON.
func cell(v any) (string, error) {
	switch x := v.(type) {
	case nil:
		return "", nil
	case string:
		return x, nil
	case []byte:
		return string(x), nil
	case bool:
		return strconv.FormatBool(x), nil
	case time.Time:
		return x.Format(time.RFC3339Nano), nil
	case json.Number:
		return x.String(), nil
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64:
		return fmt.Sprint(x), nil
	case json.RawMessage:
		var buf bytes.Buffer
		if err := json.Compact(&buf, x); err != nil {
			return "", err
		}
		return buf.String(), nil
	default:
		b, err := jsonValue(v)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
}

// jsonValue marshals a value for JSON output, preserving native nested
// structures, avoiding HTML escaping, and rendering byte slices as text.
func jsonValue(v any) ([]byte, error) {
	switch x := v.(type) {
	case json.RawMessage:
		var buf bytes.Buffer
		if err := json.Compact(&buf, x); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	case []byte:
		return marshalNoHTML(string(x))
	default:
		return marshalNoHTML(v)
	}
}

// marshalNoHTML marshals without HTML escaping and without the trailing newline
// that json.Encoder appends.
func marshalNoHTML(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
