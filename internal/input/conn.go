// Package input resolves the connection string from stdin or the environment.
// The connection string is a secret and is never a positional argument or flag.
package input

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// EnvVar is the environment variable consulted when stdin is a TTY.
const EnvVar = "Q_CONNECTION_STRING"

// Resolver locates the connection string. Its fields are injectable so the
// resolution order can be unit-tested without a real terminal.
type Resolver struct {
	Stdin      io.Reader
	StdinIsTTY bool
	Getenv     func(string) string
}

// Default builds a Resolver bound to the process stdin and environment.
func Default() Resolver {
	return Resolver{
		Stdin:      os.Stdin,
		StdinIsTTY: stdinIsTTY(),
		Getenv:     os.Getenv,
	}
}

// Resolve returns the connection string. Precedence: piped stdin, then the
// environment variable, otherwise a usage error. When stdin is piped, it wins
// over the environment.
func (r Resolver) Resolve() (string, error) {
	if !r.StdinIsTTY && r.Stdin != nil {
		b, err := io.ReadAll(r.Stdin)
		if err != nil {
			return "", fmt.Errorf("read connection string from stdin: %w", err)
		}
		if s := strings.TrimSpace(string(b)); s != "" {
			return s, nil
		}
	}
	if r.Getenv != nil {
		if s := strings.TrimSpace(r.Getenv(EnvVar)); s != "" {
			return s, nil
		}
	}
	return "", fmt.Errorf("no connection string: pipe it on stdin or set %s", EnvVar)
}

func stdinIsTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
