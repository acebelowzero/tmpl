package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/acebelowzero/tmpl/internal/render"
	"github.com/acebelowzero/tmpl/internal/values"
)

func newTemplateCmd() *cobra.Command {
	var valuesFiles []string
	var envFiles []string
	var output string

	cmd := &cobra.Command{
		Use:     "template [CHART]",
		Aliases: []string{"render"},
		Short:   "Render a stack from a tmpl chart",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			chart := "."
			if len(args) == 1 {
				chart = args[0]
			}
			if output == "" {
				output = filepath.Join(chart, "rendered-stack.yaml")
			}
			return runTemplate(cmd, chart, valuesFiles, envFiles, output)
		},
	}

	cmd.Flags().StringSliceVarP(&valuesFiles, "values", "f", nil, "Values files")
	cmd.Flags().StringSliceVar(&envFiles, "env-file", nil, "Environment files for value expansion")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file path")

	return cmd
}

func runTemplate(cmd *cobra.Command, chart string, valuesFiles, envFiles []string, output string) error {
	ctx := cmd.Context()
	loader, err := values.NewLoader(values.LoaderConfig{
		EnvFiles: envFiles,
	})
	if err != nil {
		return fmt.Errorf("setup values loader: %w", err)
	}
	mergedValues, err := loader.Load(ctx, chart, valuesFiles...)
	if err != nil {
		return fmt.Errorf("load values: %w", err)
	}

	renderer, err := render.New(render.Config{ChartPath: chart})
	if err != nil {
		return fmt.Errorf("setup renderer: %w", err)
	}

	result, err := renderer.Execute(ctx, mergedValues)
	if err != nil {
		return fmt.Errorf("render templates: %w", err)
	}

	if output == "-" {
		if _, err := cmd.OutOrStdout().Write(result); err != nil {
			return fmt.Errorf("write stdout: %w", err)
		}
		return nil
	}

	if err := writeFile(output, result); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Rendered stack written to %s\n", output)
	return nil
}

func writeFile(path string, data []byte) error {
	if path == "" {
		return errors.New("output path is empty")
	}
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

func ensureDir(dir string) error {
	if dir == "" || dir == "." {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("ensure directory %s: %w", dir, err)
	}
	return nil
}
