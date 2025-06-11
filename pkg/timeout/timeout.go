package timeout

import (
	"context"

	"github.com/Finatext/gha-fix/internal/rewrite"
	"github.com/Finatext/gha-fix/internal/timeout"
	"github.com/Finatext/gha-fix/pkg/result"
)

var ErrIndentNotCalculated = timeout.ErrIndentNotCalculated
var ErrFlowStyleNotSupported = timeout.ErrFlowStyleNotSupported
var ErrCompactJobSyntaxNotSupported = timeout.ErrCompactJobSyntaxNotSupported

type Options struct {
	IgnoreDirs     []string
	TimeoutMinutes uint64
}

type Timeout struct {
	opts Options
}

func NewTimeout(opts Options) Timeout {
	return Timeout{
		opts: opts,
	}
}

func (t Timeout) Fix(ctx context.Context, filePaths []string) (result.RewriteResult, error) {
	fixer := timeout.Fixer{TimeoutMinutes: t.opts.TimeoutMinutes}
	return rewrite.Rewrite(ctx, filePaths, t.opts.IgnoreDirs, fixer.Fix)
}
