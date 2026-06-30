package readonly

import (
	"slices"
	"strings"
	"unicode"
)

// allowed is the set of leading keywords permitted for relational engines.
var allowed = map[string]bool{
	"SELECT":   true,
	"WITH":     true,
	"TABLE":    true,
	"VALUES":   true,
	"SHOW":     true,
	"EXPLAIN":  true,
	"DESCRIBE": true,
	"DESC":     true,
}

// writeKeywords are data-modifying statements that must never appear inside a
// WITH clause (Postgres allows data-modifying CTEs).
var writeKeywords = map[string]bool{
	"INSERT": true,
	"UPDATE": true,
	"DELETE": true,
	"MERGE":  true,
}

// CheckSQL classifies a SQL statement and returns a *Violation if it is not a
// read-only operation. It is a lexer-level guard, not a full parser: it strips
// comments and quoted literals, rejects multiple statements, requires an
// allow-listed leading keyword, and blocks the known read-only evasions.
func CheckSQL(query string) error {
	stripped := strip(query)

	stmt, n := singleStatement(stripped)
	if n == 0 {
		return Deny("empty query is not a read-only operation")
	}
	if n > 1 {
		return Deny("multiple statements are not allowed")
	}

	ws := words(stmt)
	if len(ws) == 0 {
		return Deny("empty query is not a read-only operation")
	}
	kw := ws[0]
	if !allowed[kw] {
		return Deny("'" + kw + "' is not a read-only operation")
	}

	switch kw {
	case "WITH":
		for _, w := range ws {
			if writeKeywords[w] {
				return Deny("a data-modifying '" + w + "' inside WITH is not a read-only operation")
			}
		}
	case "EXPLAIN":
		if slices.Contains(ws, "ANALYZE") {
			return Deny("EXPLAIN ANALYZE executes the statement and is not a read-only operation")
		}
	}

	if kw == "SELECT" || kw == "WITH" || kw == "TABLE" || kw == "VALUES" {
		if slices.Contains(ws, "INTO") {
			return Deny("'... INTO' writes data and is not a read-only operation")
		}
	}

	return nil
}

// singleStatement splits on real statement separators and returns the first
// non-empty statement and the count of non-empty statements.
func singleStatement(stripped string) (string, int) {
	var first string
	count := 0
	for p := range strings.SplitSeq(stripped, ";") {
		if strings.TrimSpace(p) != "" {
			count++
			if count == 1 {
				first = p
			}
		}
	}
	return first, count
}

// words extracts maximal identifier-like runs, uppercased, from sanitized SQL.
func words(s string) []string {
	var out []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for _, r := range s {
		if r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur.WriteRune(unicode.ToUpper(r))
		} else {
			flush()
		}
	}
	flush()
	return out
}

// strip replaces comments and quoted literals with spaces so that token and
// statement scanning cannot be fooled by their contents.
func strip(s string) string {
	r := []rune(s)
	n := len(r)
	var b strings.Builder
	b.Grow(n)
	for i := 0; i < n; {
		c := r[i]
		switch {
		case c == '-' && i+1 < n && r[i+1] == '-':
			for i < n && r[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < n && r[i+1] == '*':
			depth := 1
			i += 2
			for i < n && depth > 0 {
				switch {
				case r[i] == '/' && i+1 < n && r[i+1] == '*':
					depth++
					i += 2
				case r[i] == '*' && i+1 < n && r[i+1] == '/':
					depth--
					i += 2
				default:
					i++
				}
			}
		case c == '\'' || c == '"' || c == '`':
			b.WriteByte(' ')
			i = skipQuoted(r, i, c)
		case c == '$':
			if tag, ok := dollarTag(r, i); ok {
				b.WriteByte(' ')
				i = skipDollar(r, i+len(tag), tag)
			} else {
				b.WriteRune(c)
				i++
			}
		default:
			b.WriteRune(c)
			i++
		}
	}
	return b.String()
}

// skipQuoted advances past a quoted region opened at index i with quote q,
// honoring doubled-quote escapes, and returns the index after the close.
func skipQuoted(r []rune, i int, q rune) int {
	n := len(r)
	i++ // opening quote
	for i < n {
		if r[i] == q {
			if i+1 < n && r[i+1] == q {
				i += 2
				continue
			}
			return i + 1
		}
		i++
	}
	return n
}

// dollarTag returns the dollar-quote delimiter (e.g. "$$" or "$tag$") starting
// at index i, if present.
func dollarTag(r []rune, i int) ([]rune, bool) {
	n := len(r)
	j := i + 1
	for j < n && (r[j] == '_' || unicode.IsLetter(r[j]) || unicode.IsDigit(r[j])) {
		j++
	}
	if j < n && r[j] == '$' {
		return r[i : j+1], true
	}
	return nil, false
}

// skipDollar advances past a dollar-quoted body opened with tag, returning the
// index after the closing tag.
func skipDollar(r []rune, i int, tag []rune) int {
	n := len(r)
	for i < n {
		if matchAt(r, i, tag) {
			return i + len(tag)
		}
		i++
	}
	return n
}

func matchAt(r []rune, i int, tag []rune) bool {
	if i+len(tag) > len(r) {
		return false
	}
	for k, t := range tag {
		if r[i+k] != t {
			return false
		}
	}
	return true
}
