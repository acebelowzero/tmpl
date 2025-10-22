package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/acebelowzero/tmpl/internal/logx"
)

// Options hold global CLI flags propagated to sub-commands.
type Options struct {
	LogLevel string
}

// NewRootCmd constructs the root command, wiring in all sub-commands.
func NewRootCmd(ctx context.Context, opts *Options) *cobra.Command {
	if opts == nil {
		opts = &Options{}
	}

	cmd := &cobra.Command{
		Use:          "tmpl",
		Short:        "Render and manage Docker Swarm stacks from Helm-like charts",
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			level := strings.TrimSpace(opts.LogLevel)
			if level == "" {
				level = os.Getenv("TMPL_LOG_LEVEL")
			}
			if level == "" {
				level = "info"
			}
			logger := logx.New(level)
			cmd.SetContext(logx.WithContext(cmd.Context(), logger))
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("no command specified; run 'tmpl --help' for usage")
		},
	}

	cmd.PersistentFlags().StringVar(&opts.LogLevel, "log-level", "info", "Log level (debug, info, warn, error)")

	// Register sub-commands
	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newTemplateCmd())
	cmd.AddCommand(newLintCmd())
	cmd.AddCommand(newPlanCmd())
	cmd.AddCommand(newApplyCmd())
	cmd.AddCommand(newRollbackCmd())
	cmd.AddCommand(newHistoryCmd())
	cmd.AddCommand(newGetCmd())
	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newDoctorCmd())

	return cmd
}
