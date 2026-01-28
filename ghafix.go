package ghafix

import (
	"context"

	gogithub "github.com/google/go-github/v72/github"

	"github.com/Finatext/gha-fix/internal/rewrite"
	"github.com/Finatext/gha-fix/pin"
	"github.com/Finatext/gha-fix/timeout"
)

// Result represents the result of a auto-fix operation.
type Result = rewrite.RewriteResult

// PinOptions defines options for the pin command.
type PinOptions struct {
	IgnoreOwners []string
	IgnoreRepos  []string
	IgnoreDirs   []string
	// Strict SHA pinning for new GitHub's SHA pinning enforcement policy. See README for details.
	StrictPinning202508 bool
}

// PinCommand is a command to pin GitHub Actions in workflow files to specific commit SHAs.
type PinCommand struct {
	pin     pin.Pin
	options PinOptions
}

// NewPinCommand creates a new PinCommand with the provided GitHub clients and options.
// primaryClient is required. fallbackClient (GitHub.com) is optional and used for tag resolution fallback.
func NewPinCommand(primaryClient *gogithub.Client, fallbackClient *gogithub.Client, opts PinOptions) PinCommand {
	return PinCommand{
		pin:     pin.NewPin(primaryClient, fallbackClient, opts.IgnoreOwners, opts.IgnoreRepos, opts.StrictPinning202508),
		options: opts,
	}
}

// Run executes the pin command with the provided context and file paths.
//
// If filePaths is specified, pin the specified workflow files. Accepts both absolute and relative paths.
// If filePaths is emtpy, list all workflow files (.yml or .yaml) in the current directory and subdirectories.
//
// When re-write YAML files, use temporary files then rename them to the original file names to do atomic updates.
func (p *PinCommand) Run(ctx context.Context, filePaths []string) (Result, error) {
	return rewrite.Rewrite(ctx, filePaths, p.options.IgnoreDirs, p.pin.Apply)
}

// TimeoutOptions defines options for the timeout command.
type TimeoutOptions struct {
	IgnoreDirs     []string
	TimeoutMinutes uint64
}

// TimeoutCommand is a command to insert timeout-minutes to GitHub Actions jobs in workflow files.
type TimeoutCommand struct {
	opts TimeoutOptions
}

// NewTimeoutCommand creates a new TimeoutCommand with the provided options.
func NewTimeoutCommand(opts TimeoutOptions) TimeoutCommand {
	return TimeoutCommand{
		opts: opts,
	}
}

// Run executes the timeout command with the provided context and file paths.
// See PinCommand.Run for details on file handling.
func (t TimeoutCommand) Run(ctx context.Context, filePaths []string) (Result, error) {
	tt := timeout.NewTimeout(t.opts.TimeoutMinutes)
	return rewrite.Rewrite(ctx, filePaths, t.opts.IgnoreDirs, tt.Insert)
}
