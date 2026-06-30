package mongo

import (
	"fmt"
	"strings"
)

// query is a parsed shell-like MongoDB command:
//
//	<collection>.<method>(<arg>[, <arg>...])
type query struct {
	Collection string
	Method     string
	Args       []string // raw JSON argument substrings, in order
}

// parse splits a shell-like Mongo command into its collection, method, and raw
// JSON arguments. It does not interpret the arguments.
func parse(s string) (query, error) {
	s = strings.TrimSpace(s)
	dot := strings.IndexByte(s, '.')
	if dot <= 0 {
		return query{}, fmt.Errorf("expected <collection>.<method>(...), got %q", s)
	}
	coll := strings.TrimSpace(s[:dot])
	rest := s[dot+1:]

	open := strings.IndexByte(rest, '(')
	if open < 0 {
		return query{}, fmt.Errorf("missing '(' in %q", s)
	}
	method := strings.TrimSpace(rest[:open])
	if method == "" {
		return query{}, fmt.Errorf("missing method in %q", s)
	}

	inner, err := parenBody(rest[open:])
	if err != nil {
		return query{}, err
	}
	return query{Collection: coll, Method: method, Args: splitArgs(inner)}, nil
}

// parenBody returns the content between the opening paren at s[0] and its
// matching close, respecting JSON strings.
func parenBody(s string) (string, error) {
	depth := 0
	inStr := false
	esc := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return s[1:i], nil
			}
		}
	}
	return "", fmt.Errorf("unbalanced parentheses")
}

// splitArgs splits top-level comma-separated arguments, respecting JSON
// brackets and strings. Empty input yields no arguments.
func splitArgs(inner string) []string {
	inner = strings.TrimSpace(inner)
	if inner == "" {
		return nil
	}
	var args []string
	depth := 0
	inStr := false
	esc := false
	start := 0
	for i := 0; i < len(inner); i++ {
		c := inner[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{', '[':
			depth++
		case '}', ']':
			depth--
		case ',':
			if depth == 0 {
				args = append(args, strings.TrimSpace(inner[start:i]))
				start = i + 1
			}
		}
	}
	if last := strings.TrimSpace(inner[start:]); last != "" {
		args = append(args, last)
	}
	return args
}
