package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/c3-oss/q/internal/adapter"
)

func newTestConnectionCmd() *cobra.Command {
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "test-connection",
		Short: "Resolve the connection string, connect, and report success or failure",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := withTimeout(cmd.Context(), timeout)
			defer cancel()

			info, factory, err := resolveTarget()
			if err != nil {
				return err
			}

			ad, err := factory.Open(ctx, info.Normalized)
			if err != nil {
				return exitErr(ExitConnection, err)
			}
			defer func() { _ = ad.Close() }()

			if err := ad.Ping(ctx); err != nil {
				return exitErr(ExitConnection, err)
			}

			detail := info.Host
			if d, ok := ad.(adapter.Describer); ok {
				if s := d.Describe(ctx); s != "" {
					detail = s
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "ok: %s — connected to %s\n", info.Engine, detail)
			return nil
		},
	}

	cmd.Flags().DurationVar(&timeout, "timeout", 0, "connection time budget, e.g. 5s (0 = no limit)")
	return cmd
}
