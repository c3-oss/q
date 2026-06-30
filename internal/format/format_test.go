package format

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/c3-oss/q/internal/adapter"
)

func rec(pairs ...any) adapter.Record {
	r := make(adapter.Record, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		r = append(r, adapter.Field{Name: pairs[i].(string), Value: pairs[i+1]})
	}
	return r
}

func render(t *testing.T, name string, fam adapter.Family, recs ...adapter.Record) (string, string) {
	t.Helper()
	var out, warn bytes.Buffer
	f, err := New(name, fam, &out, &warn)
	require.NoError(t, err)
	for _, r := range recs {
		require.NoError(t, f.Write(r))
	}
	require.NoError(t, f.Close())
	return out.String(), warn.String()
}

func TestCSVBasic(t *testing.T) {
	out, _ := render(t, "csv", adapter.Relational,
		rec("id", 1, "email", "ada@example.com"),
		rec("id", 2, "email", "alan@example.com"),
	)
	require.Equal(t, "id,email\n1,ada@example.com\n2,alan@example.com\n", out)
}

func TestCSVNestedValueIsEmbeddedJSON(t *testing.T) {
	out, _ := render(t, "csv", adapter.Relational,
		rec("id", 1, "prefs", map[string]any{"theme": "dark"}),
	)
	require.Equal(t, "id,prefs\n1,\"{\"\"theme\"\":\"\"dark\"\"}\"\n", out)
}

func TestCSVFirstRecordFixesHeader(t *testing.T) {
	out, warn := render(t, "csv", adapter.Relational,
		rec("a", 1, "b", 2),
		rec("a", 3, "c", 99), // c is extra → dropped; b missing → empty
		rec("a", 5, "b", 6),
	)
	require.Equal(t, "a,b\n1,2\n3,\n5,6\n", out)
	require.Contains(t, warn, "warning")
	require.Equal(t, 1, strings.Count(warn, "warning"), "warning should appear once")
}

func TestJSONPreservesOrderAndNesting(t *testing.T) {
	out, _ := render(t, "json", adapter.Document,
		rec("id", 1, "email", "ada@example.com", "prefs", map[string]any{"theme": "dark"}),
		rec("id", 2, "email", "alan@example.com", "prefs", map[string]any{"theme": "light"}),
	)
	want := `[{"id":1,"email":"ada@example.com","prefs":{"theme":"dark"}},` + "\n" +
		`{"id":2,"email":"alan@example.com","prefs":{"theme":"light"}}]` + "\n"
	require.Equal(t, want, out)
}

func TestJSONEmptyResult(t *testing.T) {
	out, _ := render(t, "json", adapter.Document)
	require.Equal(t, "[]\n", out)
}

func TestJSONRawMessagePassesThrough(t *testing.T) {
	out, _ := render(t, "json", adapter.Relational,
		rec("doc", json.RawMessage(`{"a": 1, "b": [2,3]}`)),
	)
	require.Equal(t, `[{"doc":{"a":1,"b":[2,3]}}]`+"\n", out)
}

func TestJSONNumberKeepsPrecision(t *testing.T) {
	out, _ := render(t, "json", adapter.WideColumn,
		rec("big", json.Number("12345678901234567890")),
	)
	require.Equal(t, `[{"big":12345678901234567890}]`+"\n", out)
}

func TestJSONByteSliceIsText(t *testing.T) {
	out, _ := render(t, "json", adapter.Relational,
		rec("name", []byte("hello")),
	)
	require.Equal(t, `[{"name":"hello"}]`+"\n", out)
}

func TestJSONNoHTMLEscaping(t *testing.T) {
	out, _ := render(t, "json", adapter.Relational,
		rec("html", "a<b>&c"),
	)
	require.Equal(t, `[{"html":"a<b>&c"}]`+"\n", out)
}

func TestTableBasic(t *testing.T) {
	out, _ := render(t, "table", adapter.Relational,
		rec("id", 1, "email", "ada@example.com"),
		rec("id", 2, "email", "alan@example.com"),
	)
	require.Contains(t, out, "id")
	require.Contains(t, out, "email")
	require.Contains(t, out, "ada@example.com")
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	require.Len(t, lines, 3) // header + 2 rows
}

func TestDefaultFormatByFamily(t *testing.T) {
	// Relational default is CSV.
	out, _ := render(t, "", adapter.Relational, rec("n", 1))
	require.Equal(t, "n\n1\n", out)
	// Document default is JSON.
	out, _ = render(t, "", adapter.Document, rec("n", 1))
	require.Equal(t, "[{\"n\":1}]\n", out)
}

func TestUnknownFormat(t *testing.T) {
	var out, warn bytes.Buffer
	_, err := New("yaml", adapter.Relational, &out, &warn)
	require.Error(t, err)
}
