package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/acebelowzero/tmpl/internal/version"
)

type versionFormat string

const (
	versionFormatText versionFormat = "text"
	versionFormatJSON versionFormat = "json"
)

func newVersionCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the tmpl version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			switch versionFormat(format) {
			case versionFormatJSON:
				return runVersionJSON(cmd)
			case versionFormatText:
				fallthrough
			default:
				return runVersionText(cmd)
			}
		},
	}

	cmd.Flags().StringVarP(&format, "output", "o", "text", "Output format: text or json")

	return cmd
}

func runVersionText(cmd *cobra.Command) error {
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "tmpl version %s\n", version.Version)
	return err
}

func runVersionJSON(cmd *cobra.Command) error {
	payload := map[string]string{
		"version":   version.Version,
		"gitCommit": version.GitCommit,
		"buildDate": version.BuildDate,
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}
