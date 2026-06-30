package redis

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/c3-oss/q/internal/readonly"
)

func TestTokenize(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"HGETALL user:1", []string{"HGETALL", "user:1"}},
		{"LRANGE q 0 -1", []string{"LRANGE", "q", "0", "-1"}},
		{"SCAN 0 MATCH 'user:*' COUNT 100", []string{"SCAN", "0", "MATCH", "user:*", "COUNT", "100"}},
		{`GET "a b"`, []string{"GET", "a b"}},
		{`SET k ""`, []string{"SET", "k", ""}},
		{"  PING  ", []string{"PING"}},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := tokenize(tc.in)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestTokenizeUnbalancedQuote(t *testing.T) {
	_, err := tokenize(`GET "unterminated`)
	require.Error(t, err)
}

func TestGuardDenylist(t *testing.T) {
	// Denylisted commands are rejected before any server round-trip, so a nil
	// client is fine here.
	c := &conn{}
	for _, name := range []string{"FLUSHALL", "FLUSHDB", "CONFIG", "EVAL", "FUNCTION", "SHUTDOWN"} {
		err := c.guard(context.Background(), name)
		require.Error(t, err, "should deny %s", name)
		var v *readonly.Violation
		require.True(t, errors.As(err, &v))
	}
}

func TestConvertReply(t *testing.T) {
	require.Equal(t, "hello", convertReply([]byte("hello")))
	require.Equal(t, []any{"a", int64(1)}, convertReply([]any{[]byte("a"), int64(1)}))
	require.Equal(t, map[string]any{"k": "v"}, convertReply(map[any]any{"k": []byte("v")}))
}
