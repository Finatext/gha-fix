package pin

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/cockroachdb/errors"
	gogithub "github.com/google/go-github/v72/github"
)

type ActionDef struct {
	Owner    string
	Repo     string
	Path     string // Subdirectory path after the repository name (e.g., "diff" in "oasdiff-action/diff")
	RefOrSHA string
}

// Check the ref is a commit SHA.
func (a ActionDef) HasCommitSHA() bool {
	if len(a.RefOrSHA) != 40 {
		return false
	}

	for _, c := range a.RefOrSHA {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

// IsReusableWorkflow determines if this action is a reusable workflow.
// Reusable workflows have file extensions in their path (e.g., .yml, .yaml).
// Composite actions do not have extensions in their path.
// This is for GitHub's SHA pinning enforcement policy (strict-pinning-202508).
func (a ActionDef) IsReusableWorkflow() bool {
	if a.Path == "" {
		return false
	}

	// Check if the path has any file extension
	parts := strings.Split(a.Path, "/")
	lastPart := parts[len(parts)-1]
	return strings.Contains(lastPart, ".")
}

// Extract version representation from ref.
// Version like string = 2.3.4, v2.3.4, v2.3.4-beta, v2.3.4+build, 2.3.4-beta, 2.3.4+build
//
// If ref is not a version representation, returns nil.
func (a ActionDef) VersionTag() *semver.Version {
	v, _ := semver.NewVersion(a.RefOrSHA)
	return v
}

type ResolvedVersion struct {
	CommitSHA  string
	RefComment string
}

//go:generate mockgen -destination=./mock_repository_service.go -package=pin github.com/Finatext/gha-fix/internal/pin RepositoryService
type RepositoryService interface {
	ListTags(ctx context.Context, owner string, repo string, opts *gogithub.ListOptions) ([]*gogithub.RepositoryTag, *gogithub.Response, error)
	// https://docs.github.com/en/rest/git/refs?apiVersion=2022-11-28#get-a-reference
	// > The :ref in the URL must be formatted as heads/<branch name> for branches and tags/<tag name> for tags. If the :ref doesn't match an existing ref, a 404 is returned.
	//
	// Although the documentation states that the `:ref` must be prefixed with `tags/` or `heads/`,
	// the GitHub API currently accepts unprefixed tags and branch names (e.g., /repos/OWNER/REPO/commits/main).
	GetCommitSHA1(ctx context.Context, owner, repo, ref, lastSHA string) (string, *gogithub.Response, error)
}

// Cache key for storing resolved versions
type cacheKey struct {
	Owner    string
	Repo     string
	RefOrSHA string
}

type VersionResolver struct {
	repoService         RepositoryService
	fallbackRepoService RepositoryService
	cache               map[cacheKey]ResolvedVersion
}

func NewVersionResolver(repoService RepositoryService, fallbackRepoService RepositoryService) VersionResolver {
	return VersionResolver{
		repoService:         repoService,
		fallbackRepoService: fallbackRepoService,
		cache:               make(map[cacheKey]ResolvedVersion),
	}
}

var AlreadyResolvedError = errors.New("already resolved")

func (r *VersionResolver) ResolveVersion(ctx context.Context, def ActionDef) (ResolvedVersion, error) {
	if def.HasCommitSHA() {
		return ResolvedVersion{}, AlreadyResolvedError
	}

	key := cacheKey{
		Owner:    def.Owner,
		Repo:     def.Repo,
		RefOrSHA: def.RefOrSHA,
	}

	if cachedVersion, ok := r.cache[key]; ok {
		return cachedVersion, nil
	}

	version := def.VersionTag()

	// The ref is not a version tag, so treat it as a branch name.
	if version == nil {
		slog.Debug("fetching commit SHA for branch", "owner", def.Owner, "repo", def.Repo, "ref", def.RefOrSHA)
		sha, _, err := r.repoService.GetCommitSHA1(ctx, def.Owner, def.Repo, def.RefOrSHA, "")
		if err != nil {
			return ResolvedVersion{}, errors.Wrapf(err, "failed to get commit SHA for %s/%s@%s", def.Owner, def.Repo, def.RefOrSHA)
		}
		resolved := ResolvedVersion{CommitSHA: sha, RefComment: def.RefOrSHA}
		r.cache[key] = resolved
		return resolved, nil
	}

	tags, err := r.listSemverTagsAll(ctx, def.Owner, def.Repo)
	if err != nil {
		return ResolvedVersion{}, errors.Wrapf(err, "failed to resolve version %s for %s/%s", def.RefOrSHA, def.Owner, def.Repo)
	}

	latest, err := findLatestTag(*version, tags)
	if err != nil {
		return ResolvedVersion{}, errors.Wrapf(err, "failed to resolve version %s for %s/%s", def.RefOrSHA, def.Owner, def.Repo)
	}

	resolved := ResolvedVersion{
		CommitSHA:  latest.gogithubTag.GetCommit().GetSHA(),
		RefComment: latest.gogithubTag.GetName(),
	}
	r.cache[key] = resolved
	return resolved, nil
}

type semverTag struct {
	gogithubTag gogithub.RepositoryTag
	version     semver.Version
}

func (r *VersionResolver) listSemverTagsAll(ctx context.Context, owner, repo string) ([]semverTag, error) {
	tags, err := r.listTagsAll(ctx, owner, repo)
	if err != nil {
		return nil, err
	}
	semverTags := make([]semverTag, 0, len(tags))
	for _, tag := range tags {
		if v, err := semver.NewVersion(tag.GetName()); err == nil && v != nil {
			semverTags = append(semverTags, semverTag{
				gogithubTag: tag,
				version:     *v,
			})
		}
	}

	return semverTags, nil
}

func (r *VersionResolver) listTagsAll(ctx context.Context, owner, repo string) ([]gogithub.RepositoryTag, error) {
	fetchAll := func(svc RepositoryService) ([]gogithub.RepositoryTag, error) {
		opts := &gogithub.ListOptions{
			PerPage: 100,
		}
		var allTags []*gogithub.RepositoryTag

		for {
			slog.Debug("fetching tags for version resolution", "owner", owner, "repo", repo, "page", opts.Page)
			tags, resp, err := svc.ListTags(ctx, owner, repo, opts)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to list tags for %s/%s", owner, repo)
			}

			allTags = append(allTags, tags...)

			if resp.NextPage == 0 {
				break
			}
			opts.Page = resp.NextPage
		}

		result := make([]gogithub.RepositoryTag, len(allTags))
		for i, tag := range allTags {
			result[i] = *tag
		}

		return result, nil
	}

	tags, err := fetchAll(r.repoService)
	if err == nil {
		return tags, nil
	}

	if r.fallbackRepoService != nil && isNotFound(err) {
		slog.Debug("primary API returned 404; falling back to GitHub.com", "owner", owner, "repo", repo)
		return fetchAll(r.fallbackRepoService)
	}

	return nil, err
}

func isNotFound(err error) bool {
	var ghErr *gogithub.ErrorResponse
	if errors.As(err, &ghErr) {
		return ghErr.Response != nil && ghErr.Response.StatusCode == http.StatusNotFound
	}
	return false
}

var NoTagsFoundError = errors.New("repository has no tags")
var TagNotFoundError = errors.New("specified tag not found")

// Find the latest tag for the given version tag following semantic versioning rules.
//
// For example:
// - v4 converts to latest v4.x.y (e.g., v4.2.2)
// - v4.1 converts to latest v4.1.z (e.g., v4.1.2)
// - v4.1.2 converts to latest v4.1.2 (if not found, retuns an error)
//
// This ignores pre-release tags and build metadata.
func findLatestTag(definedVersion semver.Version, tags []semverTag) (semverTag, error) {
	if len(tags) == 0 {
		return semverTag{}, NoTagsFoundError
	}

	// Filter tags based on version requirements
	var matchingTags []semverTag
	var exactVersion bool

	// Check if definedVersion has all version components, indicating an exact version match is needed
	// We'll determine this based on the original string format, looking for x.y.z pattern
	parts := strings.Split(definedVersion.Original(), ".")
	if len(parts) >= 3 {
		exactVersion = true
	}

	for _, tag := range tags {
		// Skip prerelease tags
		if tag.version.Prerelease() != "" {
			continue
		}

		// Major version must match
		if tag.version.Major() != definedVersion.Major() {
			continue
		}

		// If minor version specified in definedVersion, it must match
		if definedVersion.Minor() != 0 && tag.version.Minor() != definedVersion.Minor() {
			continue
		}

		// If patch version specified in definedVersion, it must match exactly
		if exactVersion && tag.version.Patch() != definedVersion.Patch() {
			continue
		}

		matchingTags = append(matchingTags, tag)
	}

	if len(matchingTags) == 0 {
		return semverTag{}, errors.Newf("no matching tags found for version %s", definedVersion.String())
	}

	// Find the highest version tag
	highestTag := matchingTags[0]
	for _, tag := range matchingTags[1:] {
		if tag.version.GreaterThan(&highestTag.version) {
			highestTag = tag
		}
	}

	return highestTag, nil
}
