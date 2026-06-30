package input

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestResolveFromStdin(t *testing.T) {
	r := Resolver{
		Stdin:      strings.NewReader("postgres://h/db\n"),
		StdinIsTTY: false,
		Getenv:     env(nil),
	}
	got, err := r.Resolve()
	require.NoError(t, err)
	require.Equal(t, "postgres://h/db", got)
}

func TestStdinWinsOverEnv(t *testing.T) {
	r := Resolver{
		Stdin:      strings.NewReader("  mysql://h/db  "),
		StdinIsTTY: false,
		Getenv:     env(map[string]string{EnvVar: "redis://h/0"}),
	}
	got, err := r.Resolve()
	require.NoError(t, err)
	require.Equal(t, "mysql://h/db", got)
}

func TestEnvUsedWhenStdinIsTTY(t *testing.T) {
	r := Resolver{
		Stdin:      strings.NewReader("ignored-because-tty"),
		StdinIsTTY: true,
		Getenv:     env(map[string]string{EnvVar: "redis://h/0"}),
	}
	got, err := r.Resolve()
	require.NoError(t, err)
	require.Equal(t, "redis://h/0", got)
}

func TestEnvUsedWhenStdinEmpty(t *testing.T) {
	r := Resolver{
		Stdin:      strings.NewReader("   \n"),
		StdinIsTTY: false,
		Getenv:     env(map[string]string{EnvVar: "redis://h/0"}),
	}
	got, err := r.Resolve()
	require.NoError(t, err)
	require.Equal(t, "redis://h/0", got)
}

func TestErrorWhenNothingProvided(t *testing.T) {
	r := Resolver{
		Stdin:      strings.NewReader(""),
		StdinIsTTY: false,
		Getenv:     env(nil),
	}
	_, err := r.Resolve()
	require.Error(t, err)
}
