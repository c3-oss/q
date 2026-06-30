// Package readonly enforces the second defense layer: it classifies a query and
// rejects any mutating or DDL operation before execution. Every engine's guard
// reports rejection with the shared Violation type so the CLI can map it to a
// single exit code.
package readonly

// Violation marks a query rejected as not read-only. The CLI maps it to exit 5.
type Violation struct {
	// Reason is a human-readable explanation without the program-name prefix.
	Reason string
}

func (v *Violation) Error() string { return "refused: " + v.Reason }

// Deny builds a Violation with the given reason.
func Deny(reason string) *Violation { return &Violation{Reason: reason} }
