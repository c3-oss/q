// Command q is a read-only, multi-database query CLI. It detects the engine
// from the connection-string scheme, runs one read-only query, and streams the
// result as CSV, JSON, or a table.
package main

import (
	"fmt"
	"os"

	"github.com/c3-oss/q/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "q: "+err.Error())
		os.Exit(cli.Code(err))
	}
}
