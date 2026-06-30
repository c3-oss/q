// Package cli wires the Cobra command tree for q.
package cli

import (
	"github.com/spf13/cobra"

	"github.com/c3-oss/q/internal/logging"
)

func newRootCmd() *cobra.Command {
	var logLevel string

	cmd := &cobra.Command{
		Use:           "q",
		Short:         "q — replace with a one-line summary of your application",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			return logging.Configure(logLevel)
		},
	}

	cmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")

	cmd.AddCommand(newVersionCmd())

	return cmd
}

// Execute runs the root command and returns any error encountered.
func Execute() error {
	return newRootCmd().Execute()
}
