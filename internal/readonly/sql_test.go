package readonly

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckSQLAccepts(t *testing.T) {
	ok := []string{
		"SELECT 1",
		"select id, email from users limit 10",
		"  SELECT * FROM t  ",
		"SELECT 1;",
		"SELECT 1 ;  ",
		"WITH t AS (SELECT 1) SELECT * FROM t",
		"WITH RECURSIVE r AS (SELECT 1) SELECT * FROM r",
		"VALUES (1), (2)",
		"TABLE users",
		"SHOW TABLES",
		"EXPLAIN SELECT * FROM t",
		"EXPLAIN (VERBOSE, FORMAT JSON) SELECT * FROM t",
		"DESCRIBE users",
		"DESC users",
		"-- a comment\nSELECT 1",
		"/* block */ SELECT 1",
		"SELECT 'DELETE FROM t' AS literal",
		"SELECT \"delete\" FROM t",
		"SELECT delete_flag FROM accounts",
		"select * from deleted_users",
	}
	for _, q := range ok {
		t.Run(q, func(t *testing.T) {
			require.NoError(t, CheckSQL(q), "should accept: %s", q)
		})
	}
}

func TestCheckSQLRejects(t *testing.T) {
	bad := []string{
		"DELETE FROM users",
		"delete from users",
		"INSERT INTO t VALUES (1)",
		"UPDATE t SET x = 1",
		"MERGE INTO t USING s ON (t.id = s.id)",
		"DROP TABLE t",
		"TRUNCATE t",
		"ALTER TABLE t ADD COLUMN x int",
		"CREATE TABLE t (id int)",
		"GRANT ALL ON t TO u",
		"CALL do_something()",
		"COPY t FROM '/tmp/x'",
		"COPY t TO STDOUT",
		"SELECT 1; DROP TABLE t",
		"SELECT 1; SELECT 2",
		"EXPLAIN ANALYZE SELECT * FROM t",
		"EXPLAIN (ANALYZE, VERBOSE) SELECT * FROM t",
		"EXPLAIN ANALYZE DELETE FROM t",
		"WITH x AS (INSERT INTO t VALUES (1) RETURNING *) SELECT * FROM x",
		"WITH x AS (UPDATE t SET a=1 RETURNING *) SELECT * FROM x",
		"WITH x AS (DELETE FROM t RETURNING *) SELECT * FROM x",
		"SELECT * INTO newtable FROM t",
		"SELECT * FROM t INTO OUTFILE '/tmp/x'",
		"",
		"   ",
		"-- only a comment",
	}
	for _, q := range bad {
		t.Run(q, func(t *testing.T) {
			err := CheckSQL(q)
			require.Error(t, err, "should reject: %s", q)
			var v *Violation
			require.True(t, errors.As(err, &v), "expected *Violation for: %s", q)
		})
	}
}

func TestCheckSQLCommentEvasion(t *testing.T) {
	// A trailing comment must not hide a second statement.
	require.Error(t, CheckSQL("SELECT 1; /* sneaky */ DELETE FROM t"))
	// A semicolon inside a string literal is not a statement separator.
	require.NoError(t, CheckSQL("SELECT ';' AS s"))
	// A keyword inside a string is not a leading keyword.
	require.NoError(t, CheckSQL("SELECT 'DROP TABLE t'"))
}

func TestViolationMessage(t *testing.T) {
	err := CheckSQL("DELETE FROM users")
	require.Equal(t, "refused: 'DELETE' is not a read-only operation", err.Error())
}
