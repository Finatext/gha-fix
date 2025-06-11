package cmd

import (
	"context"
	"log/slog"
	"os"

	"github.com/Finatext/gha-fix/pkg/timeout"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var timeoutCmd = &cobra.Command{
	Use:   "timeout",
	Short: "Add timeout-minutes to GitHub Actions jobs",
	Long: `Add timeout-minutes to GitHub Actions jobs that don't have one.

This command scans GitHub Actions workflow files (.yml or .yaml) and adds a timeout-minutes
field to jobs that don't already have one. Jobs that use reusable workflows (have a 'uses' field)
are automatically skipped.

Usage:
  timeout [file1 file2 ...] [flags]

If no files are specified, all workflow files (.yml or .yaml) in the current directory
and subdirectories will be processed.

You can customize the behavior with the following options:
  --timeout-value, -t: The timeout value in minutes to add (default: 5)

Global options:
  --ignore-dirs: Skip specific directories when searching for workflow files

Example:
  # Add default 5-minute timeout to all jobs
  gha-fix timeout

  # Add 10-minute timeout to specific files
  gha-fix timeout -t 10 .github/workflows/build.yml

  # Process all files but ignore certain directories
  gha-fix --ignore-dirs node_modules,dist timeout --timeout-value 15`,

	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		// Get values from viper which can come from flags, config file, or environment variables
		timeoutValue := viper.GetUint64("timeout.timeout-value")
		ignoreDirs := viper.GetStringSlice("ignore-dirs") // Use common ignore-dirs configuration

		if timeoutValue == 0 {
			slog.Error("timeout value must be greater than 0")
			os.Exit(1)
		}

		t := timeout.NewTimeout(timeout.Options{
			IgnoreDirs:     ignoreDirs,
			TimeoutMinutes: timeoutValue,
		})

		result, err := t.Fix(ctx, args)
		if err != nil {
			slog.Error("failed to add timeouts", "error", err)
			os.Exit(1)
		}

		if !result.Changed {
			slog.Info("no changes needed. all jobs already have timeout-minutes or no jobs found.")
		} else {
			slog.Info("successfully added timeout-minutes to jobs", slog.Int("changed", result.FileCount), slog.Uint64("timeout-minutes", timeoutValue))
		}
	},
}

func init() {
	rootCmd.AddCommand(timeoutCmd)

	timeoutCmd.Flags().Uint64P("timeout-value", "t", 5, "Timeout value in minutes to add to jobs")

	cobra.CheckErr(viper.BindPFlag("timeout.timeout-value", timeoutCmd.Flags().Lookup("timeout-value")))
}