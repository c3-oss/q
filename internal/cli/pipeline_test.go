package cli

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/c3-oss/q/internal/adapter"
	"github.com/c3-oss/q/internal/readonly"
)

// The fake backend registers under the sqlite engine. No real adapters are
// compiled into the cli test binary, so there is no registration conflict.
func init() { adapter.Register(fakeFactory{}) }

var (
	fakeOpenErr error
	fakePingErr error
	fakeRecords []adapter.Record
)

type fakeFactory struct{}

func (fakeFactory) Schemes() []string      { return []string{"sqlite"} }
func (fakeFactory) Family() adapter.Family { return adapter.Relational }
func (fakeFactory) Open(_ context.Context, _ string) (adapter.Adapter, error) {
	if fakeOpenErr != nil {
		return nil, fakeOpenErr
	}
	return &fakeAdapter{}, nil
}

type fakeAdapter struct{}

func (fakeAdapter) Ping(context.Context) error { return fakePingErr }
func (fakeAdapter) Close() error               { return nil }
func (fakeAdapter) Query(_ context.Context, query string) (adapter.Result, error) {
	if err := readonly.CheckSQL(query); err != nil {
		return nil, err
	}
	return &fakeResult{recs: fakeRecords}, nil
}

func (fakeAdapter) Describe(context.Context) string { return "fake:0 (test)" }

type fakeResult struct {
	recs []adapter.Record
	i    int
}

func (r *fakeResult) Next(context.Context) (adapter.Record, bool, error) {
	if r.i >= len(r.recs) {
		return nil, false, nil
	}
	rec := r.recs[r.i]
	r.i++
	return rec, true, nil
}
func (r *fakeResult) Close() error { return nil }

// runCLI executes the root command with the given args and a fixed connection
// string, returning stdout, stderr, and the mapped exit code.
func runCLI(t *testing.T, conn string, args ...string) (string, string, int) {
	t.Helper()
	prev := resolveConn
	resolveConn = func() (string, error) {
		if conn == "" {
			return "", errors.New("no connection string")
		}
		return conn, nil
	}
	t.Cleanup(func() {
		resolveConn = prev
		fakeOpenErr, fakePingErr, fakeRecords = nil, nil, nil
	})

	cmd := newRootCmd()
	cmd.SetArgs(args)
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	err := cmd.ExecuteContext(context.Background())
	return out.String(), errb.String(), Code(err)
}

func TestQueryStreamsCSV(t *testing.T) {
	fakeRecords = []adapter.Record{
		{{Name: "id", Value: 1}, {Name: "email", Value: "ada@example.com"}},
		{{Name: "id", Value: 2}, {Name: "email", Value: "alan@example.com"}},
	}
	out, _, code := runCLI(t, "sqlite:///tmp/x.db", "select id, email from users")
	require.Equal(t, ExitOK, code)
	require.Equal(t, "id,email\n1,ada@example.com\n2,alan@example.com\n", out)
}

func TestQueryJSONFormat(t *testing.T) {
	fakeRecords = []adapter.Record{{{Name: "n", Value: 1}}}
	out, _, code := runCLI(t, "sqlite:///tmp/x.db", "-f", "json", "select 1")
	require.Equal(t, ExitOK, code)
	require.Equal(t, "[{\"n\":1}]\n", out)
}

func TestReadOnlyViolationExit5(t *testing.T) {
	_, _, code := runCLI(t, "sqlite:///tmp/x.db", "delete from users")
	require.Equal(t, ExitReadOnly, code)
}

func TestNoConnStringExit2(t *testing.T) {
	_, _, code := runCLI(t, "", "select 1")
	require.Equal(t, ExitUsage, code)
}

func TestUnknownSchemeExit2(t *testing.T) {
	_, _, code := runCLI(t, "oracle://h/db", "select 1")
	require.Equal(t, ExitUsage, code)
}

func TestBadFormatExit2(t *testing.T) {
	_, _, code := runCLI(t, "sqlite:///tmp/x.db", "-f", "yaml", "select 1")
	require.Equal(t, ExitUsage, code)
}

func TestMissingQueryArgExit2(t *testing.T) {
	_, _, code := runCLI(t, "sqlite:///tmp/x.db")
	require.Equal(t, ExitUsage, code)
}

func TestConnectionFailureExit3(t *testing.T) {
	fakeOpenErr = errors.New("dial tcp: connection refused")
	_, _, code := runCLI(t, "sqlite:///tmp/x.db", "select 1")
	require.Equal(t, ExitConnection, code)
}

func TestTestConnectionOK(t *testing.T) {
	out, _, code := runCLI(t, "sqlite:///tmp/x.db", "test-connection")
	require.Equal(t, ExitOK, code)
	require.Contains(t, out, "ok: sqlite")
	require.Contains(t, out, "fake:0 (test)")
}

func TestTestConnectionPingFailureExit3(t *testing.T) {
	fakePingErr = errors.New("ping timeout")
	_, _, code := runCLI(t, "sqlite:///tmp/x.db", "test-connection")
	require.Equal(t, ExitConnection, code)
}
