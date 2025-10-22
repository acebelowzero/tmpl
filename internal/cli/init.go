package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var chartName string

	cmd := &cobra.Command{
		Use:   "init [DIRECTORY]",
		Short: "Create a new tmpl chart skeleton",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := "."
			if len(args) == 1 {
				target = args[0]
			}
			return runInit(cmd, target, chartName)
		},
	}

	cmd.Flags().StringVar(&chartName, "name", "example", "Chart name")

	return cmd
}

func runInit(cmd *cobra.Command, dir, chartName string) error {
	dir = filepath.Clean(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create chart directory: %w", err)
	}

	chartPath := filepath.Join(dir, "Chart.yaml")
	if _, err := os.Stat(chartPath); err == nil {
		return errors.New("chart already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check chart existence: %w", err)
	}

	files := map[string]string{
		"Chart.yaml": `apiVersion: v1
name: ` + chartName + `
description: A tmpl chart
version: 0.1.0
appVersion: "1.0.0"
`,
		"values.yaml": `# Default values for ` + chartName + `.
# This is a YAML-formatted file.
`,
		"values.schema.json": `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "additionalProperties": false,
  "properties": {}
}
`,
		filepath.Join("templates", "stack.yaml.tmpl"): `version: "3.9"
services:
  example:
    image: nginx:latest
`,
		"_helpers.tpl": `{{- define "chart.name" -}}
` + chartName + `
{{- end -}}
`,
	}

	for relPath, contents := range files {
		fullPath := filepath.Join(dir, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return fmt.Errorf("create directories for %s: %w", relPath, err)
		}
		if err := os.WriteFile(fullPath, []byte(contents), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", relPath, err)
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Created chart skeleton in %s\n", dir)
	return nil
}
