package main

import (
	"context"
	"log/slog"
	"os"

	ghafix "github.com/Finatext/gha-fix"
	"github.com/Finatext/gha-fix/internal/githubclient"
	"github.com/google/go-github/v72/github"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var pinCmd = &cobra.Command{
	Use:   "pin [file1 file2 ...]",
	Short: "Pin GitHub Actions in workflow files to specific commit SHAs",
	Long: `Pin GitHub Actions in workflow files to specific commit SHAs to enhance security and reliability.

This command scans workflow files for GitHub Actions and replaces version references with specific commit SHAs.
It supports various options to customize its behavior. For example, it can replace 'owner/repo@v1' with a specific commit SHA like 'owner/repo@8843d7f53bd34e3b78f2acee556ba5d53feae7c4'.
Usage:
  pin [file1 file2 ...]
If no files are specified, all workflow files (.yml or .yaml) in the current directory
and subdirectories will be processed.

	You can customize the behavior with the following options:
  --github-token: GitHub token for accessing GitHub API (can also be set via GITHUB_TOKEN env var or pin.github-token in config)
  --ghes-github-token: GitHub token for GitHub Enterprise Server (can also be set via GHES_GITHUB_TOKEN env var or pin.ghes-github-token in config)
  --ignore-owners: Skip actions from specific owners (e.g., "actions,github")
  --ignore-repos: Skip specific repositories (e.g., "actions/checkout,docker/login-action")
  --strict-pinning-202508: Enable strict SHA pinning for composite actions (GitHub's SHA pinning enforcement policy)
  --api-server: Full GitHub API base URL (defaults to https://api.github.com/ when not specified, e.g., https://github.enterprise.company.com/api/v3)

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

		// Resolve API base
		apiServer := viper.GetString("pin.api-server")
		if apiServer == "" {
			apiServer = os.Getenv("GITHUB_API_URL")
		}
		apiServer, err := githubclient.NormalizeAPIBaseURL(apiServer)
		if err != nil {
			slog.Error("invalid api-server", "error", err)
			os.Exit(1)
		}
		if apiServer == "" {
			apiServer = githubclient.DefaultAPIBaseURL
		}
		isDefaultAPI := apiServer == githubclient.DefaultAPIBaseURL

		// Tokens
		var primaryToken string
		var fallbackToken string

		if isDefaultAPI {
			primaryToken = viper.GetString("pin.github-token") // bound to GITHUB_TOKEN or flag/config
			if primaryToken == "" {
				slog.Error("GITHUB_TOKEN is required for GitHub.com API calls. Use --github-token flag, GITHUB_TOKEN env var, or pin.github-token in config file.")
				os.Exit(1)
			}
		} else {
			primaryToken = viper.GetString("pin.ghes-github-token")
			if primaryToken == "" {
				slog.Error("GHES_GITHUB_TOKEN is required when api-server is not https://api.github.com/. Set GHES_GITHUB_TOKEN or use --ghes-github-token flag or pin.ghes-github-token in config.")
				os.Exit(1)
			}
			fallbackToken = viper.GetString("pin.github-token") // GITHUB_TOKEN
			if fallbackToken == "" {
				slog.Error("GITHUB_TOKEN is required for GitHub.com fallback when api-server is not https://api.github.com/. Set GITHUB_TOKEN to enable fallback tag resolution.")
				os.Exit(1)
			}
		}

		primaryClient, err := githubclient.NewClient(primaryToken, apiServer)
		if err != nil {
			slog.Error("failed to create primary GitHub client", "error", err)
			os.Exit(1)
		}

		var fallbackClient *github.Client
		if !isDefaultAPI {
			fallbackClient, err = githubclient.NewClient(fallbackToken, githubclient.DefaultAPIBaseURL)
			if err != nil {
				slog.Error("failed to create fallback GitHub.com client", "error", err)
				os.Exit(1)
			}
		}

		// Get values from viper which can come from flags, config file, or environment variables
		ignoreOwners := viper.GetStringSlice("pin.ignore-owners")
		ignoreRepos := viper.GetStringSlice("pin.ignore-repos")
		ignoreDirs := viper.GetStringSlice("ignore-dirs") // Use common ignore-dirs configuration
		strictPinning202508 := viper.GetBool("pin.strict-pinning-202508")

		pinCmd := ghafix.NewPinCommand(primaryClient, fallbackClient, ghafix.PinOptions{
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

	// GHES token (used when api-server is not https://api.github.com/)
	pinCmd.Flags().String("ghes-github-token", "", "GitHub token for GHES API calls (can also be set via GHES_GITHUB_TOKEN env var or pin.ghes-github-token in config)")
	cobra.CheckErr(viper.BindPFlag("pin.ghes-github-token", pinCmd.Flags().Lookup("ghes-github-token")))
	cobra.CheckErr(viper.BindEnv("pin.ghes-github-token", "GHES_GITHUB_TOKEN"))

	pinCmd.Flags().StringSlice("ignore-owners", []string{}, "Comma-separated list of owners to ignore")
	pinCmd.Flags().StringSlice("ignore-repos", []string{}, "Comma-separated list of repos to ignore in format owner/repo")
	pinCmd.Flags().Bool("strict-pinning-202508", false, "Enable strict SHA pinning for composite actions (GitHub's SHA pinning enforcement policy)")

	// Full GitHub API base URL (GHES support)
	pinCmd.Flags().String("api-server", "", "Full GitHub API base URL (e.g., https://github.enterprise.company.com/api/v3/)")
	cobra.CheckErr(viper.BindPFlag("pin.api-server", pinCmd.Flags().Lookup("api-server")))
}
