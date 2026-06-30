// Package cli wires the Cobra command tree for q.
package cli

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/c3-oss/q/internal/format"
	"github.com/c3-oss/q/internal/logging"
)

func newRootCmd() *cobra.Command {
	var (
		logLevel   string
		formatName string
		timeout    time.Duration
	)

	cmd := &cobra.Command{
		Use:   "q [flags] <query>",
		Short: "q — read-only multi-database query CLI",
		Long: "q connects to a database, auto-detected from the connection-string scheme,\n" +
			"runs a single read-only query, and streams the result as CSV, JSON, or a table.\n\n" +
			"The connection string is read from stdin (when piped) or the\n" +
			"Q_CONNECTION_STRING environment variable — never from the command line.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return exitErr(ExitUsage, errors.New("requires exactly one <query> argument"))
			}
			return nil
		},
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			return logging.Configure(logLevel)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQuery(cmd, args[0], formatName, timeout)
		},
	}

	cmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")
	cmd.Flags().StringVarP(&formatName, "format", "f", "",
		"output format: csv, json, or table (default: engine-dependent)")
	cmd.Flags().DurationVar(&timeout, "timeout", 0,
		"overall time budget, e.g. 30s or 2m (0 = no limit)")
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return exitErr(ExitUsage, err)
	})

	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newTestConnectionCmd())

	return cmd
}

func runQuery(cmd *cobra.Command, query, formatName string, timeout time.Duration) error {
	ctx, cancel := withTimeout(cmd.Context(), timeout)
	defer cancel()

	info, factory, err := resolveTarget()
	if err != nil {
		return err
	}

	fmtr, err := format.New(formatName, factory.Family(), cmd.OutOrStdout(), cmd.ErrOrStderr())
	if err != nil {
		return exitErr(ExitUsage, err)
	}

	ad, err := factory.Open(ctx, info.Normalized)
	if err != nil {
		return exitErr(ExitConnection, err)
	}
	defer func() { _ = ad.Close() }()

	res, err := ad.Query(ctx, query)
	if err != nil {
		return classifyQueryErr(err)
	}
	defer func() { _ = res.Close() }()

	for {
		rec, ok, err := res.Next(ctx)
		if err != nil {
			return classifyQueryErr(err)
		}
		if !ok {
			break
		}
		if err := fmtr.Write(rec); err != nil {
			return exitErr(ExitInternal, err)
		}
	}
	if err := fmtr.Close(); err != nil {
		return exitErr(ExitInternal, err)
	}
	return nil
}

// Execute runs the root command, cancelling on interrupt, and returns any error.
func Execute() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return newRootCmd().ExecuteContext(ctx)
}
