package cmd

import (
	"context"
	"log/slog"
	"os"

	"github.com/Finatext/gha-fix/pkg/pin"
	"github.com/google/go-github/v72/github"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var pinCmd = &cobra.Command{
	Use:   "pin",
	Short: "Pin GitHub Actions to specific commit SHAs",
	Long: `Pin GitHub Actions used in workflow files (.yml or .yaml) to specific commit SHAs.

This command scans GitHub Actions in workflow files and replaces references like 'owner/repo@v1'
with specific commit SHAs like 'owner/repo@8843d7f53bd34e3b78f2acee556ba5d53feae7c4'.

Usage:
  pin [file1 file2 ...]

If no files are specified, all workflow files (.yml or .yaml) in the current directory
and subdirectories will be processed.

You can customize the behavior with the following options:
  --ignore-owners: Skip actions from specific owners (e.g., "actions,github")
  --ignore-repos: Skip specific repositories (e.g., "actions/checkout,docker/login-action")

Global options:
  --ignore-dirs: Skip specific directories when searching for workflow files (e.g., "node_modules,dist")

Note: GITHUB_TOKEN environment variable is required to fetch tags and commit SHAs from GitHub.`,

	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		githubToken := viper.GetString("pin.github-token")
		if githubToken == "" {
			slog.Error("GitHub token is required. Use --github-token flag, GITHUB_TOKEN env var, or pin.github-token in config file.")
			os.Exit(1)
		}

		githubClient := github.NewClient(nil).WithAuthToken(githubToken)

		// Get values from viper which can come from flags, config file, or environment variables
		ignoreOwners := viper.GetStringSlice("pin.ignore-owners")
		ignoreRepos := viper.GetStringSlice("pin.ignore-repos")
		ignoreDirs := viper.GetStringSlice("ignore-dirs") // Use common ignore-dirs configuration

		pinner := pin.NewPinner(githubClient, pin.Options{
			IgnoreOwners: ignoreOwners,
			IgnoreRepos:  ignoreRepos,
			IgnoreDirs:   ignoreDirs,
		})

		result, err := pinner.Pin(ctx, args)
		if err != nil {
			slog.Error("failed to pin actions", "error", err)
			os.Exit(1)
		}

		if !result.Changed {
			slog.Info("no changes needed. all GitHub Actions are already pinned or no actions found.")
		} else {
			slog.Info("successfully pinned GitHub Actions to specific commit SHAs", slog.Int("changed", result.FileCount))
		}
	},
}

var (
	ghToken string
)

func init() {
	rootCmd.AddCommand(pinCmd)

	// Configure GitHub token options specifically for the pin command
	pinCmd.Flags().StringVarP(&ghToken, "github-token", "", "", "GitHub token for accessing GitHub API (can also be set via GITHUB_TOKEN env var or pin.github-token in config)")
	cobra.CheckErr(viper.BindPFlag("pin.github-token", pinCmd.Flags().Lookup("github-token")))
	// Bind GITHUB_TOKEN environment variable directly to pin.github-token
	// This avoids the prefix from viper.SetEnvPrefix
	cobra.CheckErr(viper.BindEnv("pin.github-token", "GITHUB_TOKEN"))

	pinCmd.Flags().StringSlice("ignore-owners", []string{}, "Comma-separated list of owners to ignore")
	pinCmd.Flags().StringSlice("ignore-repos", []string{}, "Comma-separated list of repos to ignore in format owner/repo")

	cobra.CheckErr(viper.BindPFlag("pin.ignore-owners", pinCmd.Flags().Lookup("ignore-owners")))
	cobra.CheckErr(viper.BindPFlag("pin.ignore-repos", pinCmd.Flags().Lookup("ignore-repos")))
}
