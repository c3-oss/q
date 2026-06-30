package cli

import "errors"

// Process exit codes. main maps a returned error to one of these.
const (
	ExitOK         = 0
	ExitInternal   = 1
	ExitUsage      = 2
	ExitConnection = 3
	ExitQuery      = 4
	ExitReadOnly   = 5
)

// ExitError carries a process exit code alongside the underlying error.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string { return e.Err.Error() }
func (e *ExitError) Unwrap() error { return e.Err }

func exitErr(code int, err error) *ExitError {
	return &ExitError{Code: code, Err: err}
}

// Code returns the exit code for an error: the ExitError code if present,
// ExitOK for nil, otherwise ExitInternal.
func Code(err error) int {
	if err == nil {
		return ExitOK
	}
	var ee *ExitError
	if errors.As(err, &ee) {
		return ee.Code
	}
	return ExitInternal
}
