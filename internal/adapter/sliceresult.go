package adapter

import "context"

// SliceResult is a Result backed by an in-memory slice of records. Adapters
// whose reply is small and fully materialized (a count, a distinct list, a
// single command reply) use it instead of a live cursor.
type SliceResult struct {
	Records []Record
	i       int
}

// Next yields the next buffered record.
func (r *SliceResult) Next(context.Context) (Record, bool, error) {
	if r.i >= len(r.Records) {
		return nil, false, nil
	}
	rec := r.Records[r.i]
	r.i++
	return rec, true, nil
}

// Close is a no-op for buffered results.
func (r *SliceResult) Close() error { return nil }
