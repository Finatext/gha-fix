package main

import (
	"context"
	"log/slog"
	"os"

	ghafix "github.com/Finatext/gha-fix"
	"github.com/Finatext/gha-fix/internal/githubclient"
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
  --strict-pinning-202508: Enable strict SHA pinning for composite actions (GitHub's SHA pinning enforcement policy)
  --api-server: Full GitHub API base URL (e.g., https://github.enterprise.company.com/api/v3/)

The --strict-pinning-202508 option implements support for GitHub's SHA pinning enforcement policy
announced in August 2025. When enabled:
  - Composite actions (e.g., actions/checkout@v4) will be pinned to SHAs even if owner is in ignore-owners
  - Reusable workflows (e.g., org/repo/.github/workflows/build.yml@main) still respect ignore-owners

This helps organizations comply with GitHub's security policies while maintaining flexibility
for reusable workflows. See: https://github.blog/changelog/2025-08-15-github-actions-policy-now-supports-blocking-and-sha-pinning-actions/

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

		// API server resolution priority:
		// 1) pin.api-server (flag/config)
		// 2) GITHUB_API_URL env var
		// 3) default https://api.github.com/
		apiServer := viper.GetString("pin.api-server")
		if apiServer == "" {
			apiServer = os.Getenv("GITHUB_API_URL")
		}

		githubClient, err := githubclient.NewClient(githubToken, apiServer)
		if err != nil {
			slog.Error("failed to create GitHub client", "error", err)
			os.Exit(1)
		}

		// Get values from viper which can come from flags, config file, or environment variables
		ignoreOwners := viper.GetStringSlice("pin.ignore-owners")
		ignoreRepos := viper.GetStringSlice("pin.ignore-repos")
		ignoreDirs := viper.GetStringSlice("ignore-dirs") // Use common ignore-dirs configuration
		strictPinning202508 := viper.GetBool("pin.strict-pinning-202508")

		pinCmd := ghafix.NewPinCommand(githubClient, ghafix.PinOptions{
			IgnoreOwners:        ignoreOwners,
			IgnoreRepos:         ignoreRepos,
			IgnoreDirs:          ignoreDirs,
			StrictPinning202508: strictPinning202508,
		})

		result, err := pinCmd.Run(ctx, args)
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
	pinCmd.Flags().Bool("strict-pinning-202508", false, "Enable strict SHA pinning for composite actions (GitHub's SHA pinning enforcement policy)")

	// Full GitHub API base URL (GHES support)
	pinCmd.Flags().String("api-server", "", "Full GitHub API base URL (e.g., https://github.enterprise.company.com/api/v3/)")
	cobra.CheckErr(viper.BindPFlag("pin.api-server", pinCmd.Flags().Lookup("api-server")))

	cobra.CheckErr(viper.BindPFlag("pin.ignore-owners", pinCmd.Flags().Lookup("ignore-owners")))
	cobra.CheckErr(viper.BindPFlag("pin.ignore-repos", pinCmd.Flags().Lookup("ignore-repos")))
	cobra.CheckErr(viper.BindPFlag("pin.strict-pinning-202508", pinCmd.Flags().Lookup("strict-pinning-202508")))
}

