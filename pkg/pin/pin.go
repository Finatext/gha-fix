package pin

import (
	"context"

	gogithub "github.com/google/go-github/v72/github"

	"github.com/Finatext/gha-fix/internal/pin"
	"github.com/Finatext/gha-fix/internal/rewrite"
	"github.com/Finatext/gha-fix/pkg/result"
)

type Options struct {
	IgnoreOwners []string
	IgnoreRepos  []string
	IgnoreDirs   []string
}

type Pinner struct {
	replacer *pin.Replacer
	options  Options
}

func NewPinner(client *gogithub.Client, opts Options) Pinner {
	resolver := pin.NewVersionResolver(client.Repositories)
	return Pinner{
		replacer: &pin.Replacer{
			Resolver:     &resolver,
			IgnoreOwners: opts.IgnoreOwners,
			IgnoreRepos:  opts.IgnoreRepos,
		},
		options: opts,
	}
}

// If filePaths is specified, pin the specified workflow files. Accepts both absolute and relative paths.
// If filePaths is emtpy, list all workflow files (.yml or .yaml) in the current directory and subdirectories.
//
// When re-write YAML files, use temporary files then rename them to the original file names to do atomic updates.
func (p *Pinner) Pin(ctx context.Context, filePaths []string) (result.RewriteResult, error) {
	return rewrite.Rewrite(ctx, filePaths, p.options.IgnoreDirs, p.replacer.Replace)
}
