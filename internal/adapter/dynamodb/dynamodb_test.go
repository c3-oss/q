package dynamodb

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/c3-oss/q/internal/readonly"
)

func TestCheckSelect(t *testing.T) {
	for _, ok := range []string{
		`SELECT * FROM "Users"`,
		`select id from t where id = 'u-42'`,
		`  SELECT a, b FROM t`,
	} {
		require.NoError(t, checkSelect(ok), "should accept %q", ok)
	}

	for _, bad := range []string{
		`INSERT INTO t VALUE {'id': '1'}`,
		`UPDATE t SET x = 1`,
		`DELETE FROM t WHERE id = '1'`,
		``,
	} {
		err := checkSelect(bad)
		require.Error(t, err, "should reject %q", bad)
		var v *readonly.Violation
		require.True(t, errors.As(err, &v))
	}
}
