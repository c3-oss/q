package mongo

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	cases := []struct {
		in     string
		coll   string
		method string
		args   []string
	}{
		{
			`events.aggregate([{"$match":{"type":"signup"}},{"$count":"n"}])`,
			"events", "aggregate",
			[]string{`[{"$match":{"type":"signup"}},{"$count":"n"}]`},
		},
		{
			`users.find({"age":{"$gt":21}}, {"name":1})`,
			"users", "find",
			[]string{`{"age":{"$gt":21}}`, `{"name":1}`},
		},
		{`c.distinct("type")`, "c", "distinct", []string{`"type"`}},
		{`c.find()`, "c", "find", nil},
		{`  orders.countDocuments( {"paid": true} ) `, "orders", "countDocuments", []string{`{"paid": true}`}},
		{`coll.find({"s":"a, b"})`, "coll", "find", []string{`{"s":"a, b"}`}}, // comma inside string
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			q, err := parse(tc.in)
			require.NoError(t, err)
			require.Equal(t, tc.coll, q.Collection)
			require.Equal(t, tc.method, q.Method)
			require.Equal(t, tc.args, q.Args)
		})
	}
}

func TestParseErrors(t *testing.T) {
	for _, in := range []string{"noparens", "events.find(", ".find()", "events.()"} {
		_, err := parse(in)
		require.Error(t, err, "expected parse error for %q", in)
	}
}
