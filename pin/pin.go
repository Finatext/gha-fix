package pin

import (
	"context"
	"regexp"
	"slices"
	"strings"

	"github.com/cockroachdb/errors"
	gogithub "github.com/google/go-github/v72/github"

	"github.com/Finatext/gha-fix/internal/pin"
)

type resolver interface {
	ResolveVersion(ctx context.Context, def pin.ActionDef) (pin.ResolvedVersion, error)
}

type Pin struct {
	resolver            resolver
	ignoreOwners        []string
	ignoreRepos         []string
	strictPinning202508 bool
}

// NewPin creates a pin command with primary GitHub client and optional fallback GitHub.com client.
func NewPin(primaryClient *gogithub.Client, fallbackClient *gogithub.Client, ignoreOwners, ignoreRepos []string, strictPinning202508 bool) Pin {
	var fallbackRepos *gogithub.RepositoriesService
	if fallbackClient != nil {
		fallbackRepos = fallbackClient.Repositories
	}
	resolver := pin.NewVersionResolver(primaryClient.Repositories, fallbackRepos)
	return Pin{
		resolver:            &resolver,
		ignoreOwners:        ignoreOwners,
		ignoreRepos:         ignoreRepos,
		strictPinning202508: strictPinning202508,
	}
}

// Apply replaces input YAML content then returns the modified content, a boolean indicating if any replacements were
// made, and an error if any occurred.
func (p *Pin) Apply(ctx context.Context, input string) (string, bool, error) {
	lines := strings.Split(input, "\n")

	changed := false
	resultLines := make([]string, 0, len(lines))
	for _, line := range lines {
		modifiedLine, lineChanged, err := p.replaceLine(ctx, line)
		if err != nil {
			return "", false, err
		}

		if lineChanged {
			changed = true
			line = modifiedLine
		}
		resultLines = append(resultLines, line)
	}

	// Join lines back into a single string using strings.Join (more efficient than concatenation)
	output := strings.Join(resultLines, "\n")

	return output, changed, nil
}

func (p *Pin) replaceLine(ctx context.Context, line string) (string, bool, error) {
	parsed, ok := parseLine(line)
	if !ok {
		return line, false, nil // No action definition found, return the line unchanged
	}
	def := parsed.def

	// Apply ignore owners check (skip for composite actions when strict pinning is enabled)
	if !p.strictPinning202508 || def.IsReusableWorkflow() {
		if slices.Contains(p.ignoreOwners, def.Owner) {
			return line, false, nil
		}
	}

	repoKey := def.Owner + "/" + def.Repo
	if slices.Contains(p.ignoreRepos, repoKey) {
		return line, false, nil
	}

	if def.HasCommitSHA() {
		return line, false, nil
	}

	resolved, err := p.resolver.ResolveVersion(ctx, def)
	if err != nil {
		if errors.Is(err, pin.AlreadyResolvedError) {
			return line, false, nil
		}
		return "", false, errors.Wrapf(err, "failed to resolve version for %s/%s@%s", def.Owner, def.Repo, def.RefOrSHA)
	}

	newComment := " # " + resolved.RefComment
	if parsed.comment != "" {
		newComment += " " + parsed.comment
	}

	// Reconstruct the path part if necessary
	repoPath := def.Repo
	if def.Path != "" {
		repoPath = def.Repo + "/" + def.Path
	}

	// Construct the new line using the original quotes
	newRef := def.Owner + "/" + repoPath + "@" + resolved.CommitSHA
	newLine := parsed.prefix + parsed.openQuote + newRef + parsed.closeQuote + newComment

	return newLine, true, nil
}

type parsedLine struct {
	def        pin.ActionDef
	prefix     string
	openQuote  string // Opening quote if any (e.g., '"' or ''')
	closeQuote string // Closing quote if any (should match openQuote)
	comment    string // Comment part of the line (if any)
}

// regexp to match and extract the action definition, see testdata/pin.yml for examples:
//
//   - uses: actions/checkout@v4 # Some comment
//   - uses: actions/setup-go@v5.4
//   - uses: "actions/checkout@v4"
//   - uses: 'actions/checkout@v4'
//     uses: golangci/golangci-lint-action@1481404843c368bc19ca9406f87d6e0fc97bdcfd # v7.0.0
//     uses: Finatext/workflows-public/.github/workflows/gha-lint.yml@main
var usesPattern = regexp.MustCompile(`^([-\s]*(?:["']?uses["']?:\s+))(["']?)([^/"']+)/([^/"']+)(/[^@"']+)?(@)([^\s#"']+)(["']?)(.*)`)

// Group indices:
// 1: prefix (e.g., "- uses: ", "   uses: ", or "   "uses": ")
// 2: opening quote (if any)
// 3: owner (e.g., "actions")
// 4: repo (e.g., "checkout")
// 5: path (e.g., "/diff" - optional)
// 6: @ symbol
// 7: refOrSHA (e.g., "v4", "main", commit SHA)
// 8: closing quote (if any)
// 9: suffix (comments, etc.)

func parseLine(line string) (parsedLine, bool) {
	// Check for leading comments
	trimmed := strings.TrimSpace(line)
	if len(trimmed) > 0 && trimmed[0] == '#' {
		return parsedLine{}, false
	}

	matches := usesPattern.FindStringSubmatch(line)
	if matches == nil {
		return parsedLine{}, false
	}

	// Capture components from regex match
	prefix := matches[1]    // "- uses: " or "uses: "
	openQuote := matches[2] // Opening quote if any
	owner := matches[3]     // e.g., "actions"
	repo := matches[4]      // e.g., "checkout" or "oasdiff-action"
	path := ""
	if matches[5] != "" {
		path = matches[5][1:] // Remove leading slash, e.g., "diff" from "/diff"
	}
	refOrSHA := matches[7]   // e.g., "v4", "main", or commit SHA
	closeQuote := matches[8] // Closing quote if any
	suffix := matches[9]     // Any trailing comment or whitespace

	comment := ""
	if commentIdx := strings.Index(suffix, "#"); commentIdx >= 0 {
		comment = strings.TrimSpace(suffix[commentIdx:])
	}

	def := pin.ActionDef{
		Owner:    owner,
		Repo:     repo,
		Path:     path,
		RefOrSHA: refOrSHA,
	}

	return parsedLine{
		def:        def,
		prefix:     prefix,
		openQuote:  openQuote,
		closeQuote: closeQuote,
		comment:    comment,
	}, true
}
