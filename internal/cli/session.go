package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/c3-oss/q/internal/adapter"
	"github.com/c3-oss/q/internal/dsn"
	"github.com/c3-oss/q/internal/input"
	"github.com/c3-oss/q/internal/readonly"
)

// resolveConn is the connection-string source. It is a package variable so
// tests can supply a fixed value instead of reading the real stdin/env.
var resolveConn = func() (string, error) { return input.Default().Resolve() }

// resolveTarget resolves the connection string, detects the engine, and looks
// up its factory. Failures are typed with the right exit code.
func resolveTarget() (dsn.Info, adapter.Factory, error) {
	conn, err := resolveConn()
	if err != nil {
		return dsn.Info{}, nil, exitErr(ExitUsage, err)
	}
	info, err := dsn.Detect(conn)
	if err != nil {
		return dsn.Info{}, nil, exitErr(ExitUsage, err)
	}
	factory, ok := adapter.Lookup(info.Engine)
	if !ok {
		return dsn.Info{}, nil, exitErr(ExitUsage,
			fmt.Errorf("no adapter registered for engine %q (registered: %s)",
				info.Engine, strings.Join(adapter.Schemes(), ", ")))
	}
	return info, factory, nil
}

// withTimeout derives a context bounded by d, or merely cancelable when d <= 0.
func withTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, d)
}

// classifyQueryErr maps a query error to a read-only violation (exit 5) or a
// generic query error (exit 4).
func classifyQueryErr(err error) error {
	var v *readonly.Violation
	if errors.As(err, &v) {
		return exitErr(ExitReadOnly, err)
	}
	return exitErr(ExitQuery, err)
}
