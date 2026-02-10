package pin

import (
	"context"
	"testing"

	"github.com/Masterminds/semver/v3"
	gogithub "github.com/google/go-github/v72/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

func TestVersionResolver_ResolveVersion(t *testing.T) {
	t.Run("Cache hit after first call for branch reference", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockRepo := NewMockRepositoryService(ctrl)

		// Setup the mock to expect only ONE call
		mockRepo.EXPECT().
			GetCommitSHA1(gomock.Any(), "actions", "checkout", "main", "").
			Return("11bd71901bbe5b1630ceea73d27597364c9af683", &gogithub.Response{}, nil).Times(1)

		resolver := NewVersionResolver(mockRepo, nil)

		// First call should hit the API
		def := ActionDef{
			Owner:    "actions",
			Repo:     "checkout",
			RefOrSHA: "main",
		}

		result1, err := resolver.ResolveVersion(context.Background(), def)
		require.NoError(t, err)
		assert.Equal(t, "11bd71901bbe5b1630ceea73d27597364c9af683", result1.CommitSHA)

		// Second call should use cache (mock won't be called again)
		result2, err := resolver.ResolveVersion(context.Background(), def)
		require.NoError(t, err)
		assert.Equal(t, result1.CommitSHA, result2.CommitSHA)
		assert.Equal(t, result1.RefComment, result2.RefComment)
	})

	t.Run("Cache hit after first call for version tag", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockRepo := NewMockRepositoryService(ctrl)

		// Mock list tags response (should be called only once)
		tags := []*gogithub.RepositoryTag{
			createTag("v4.0.0", "sha1"),
			createTag("v4.1.0", "sha2"),
			createTag("v4.1.1", "sha3"),
			createTag("v5.0.0", "sha4"),
		}
		mockRepo.EXPECT().
			ListTags(gomock.Any(), "actions", "checkout", gomock.Any()).
			Return(tags, &gogithub.Response{NextPage: 0}, nil).Times(1)

		resolver := NewVersionResolver(mockRepo, nil)

		def := ActionDef{
			Owner:    "actions",
			Repo:     "checkout",
			RefOrSHA: "v4",
		}

		// First call should hit the API
		result1, err := resolver.ResolveVersion(context.Background(), def)
		require.NoError(t, err)
		assert.Equal(t, "sha3", result1.CommitSHA) // v4.1.1 is the latest v4 tag
		assert.Equal(t, "v4.1.1", result1.RefComment)

		// Second call should use cache (mock won't be called again)
		result2, err := resolver.ResolveVersion(context.Background(), def)
		require.NoError(t, err)
		assert.Equal(t, result1.CommitSHA, result2.CommitSHA)
		assert.Equal(t, result1.RefComment, result2.RefComment)
	})

	tests := []struct {
		name      string
		actionDef ActionDef
		mockSetup func(*MockRepositoryService)
		expected  ResolvedVersion
	}{
		{
			name: "Resolve commit SHA for branch reference",
			actionDef: ActionDef{
				Owner:    "actions",
				Repo:     "checkout",
				RefOrSHA: "main",
			},
			mockSetup: func(mock *MockRepositoryService) {
				mock.EXPECT().
					GetCommitSHA1(gomock.Any(), "actions", "checkout", "main", "").
					Return("11bd71901bbe5b1630ceea73d27597364c9af683", &gogithub.Response{}, nil)
			},
			expected: ResolvedVersion{
				CommitSHA:  "11bd71901bbe5b1630ceea73d27597364c9af683",
				RefComment: "main",
			},
		},
		{
			name: "Resolve commit SHA for semver tag",
			actionDef: ActionDef{
				Owner:    "actions",
				Repo:     "checkout",
				RefOrSHA: "v4",
			},
			mockSetup: func(mock *MockRepositoryService) {
				// Mock list tags response
				tags := []*gogithub.RepositoryTag{
					createTag("v4.0.0", "sha1"),
					createTag("v4.1.0", "sha2"),
					createTag("v4.1.1", "sha3"),
					createTag("v5.0.0", "sha4"),
				}
				mock.EXPECT().
					ListTags(gomock.Any(), "actions", "checkout", gomock.Any()).
					Return(tags, &gogithub.Response{NextPage: 0}, nil)
			},
			expected: ResolvedVersion{
				CommitSHA:  "sha3", // v4.1.1 is the latest v4 tag
				RefComment: "v4.1.1",
			},
		},
		{
			name: "Resolve commit SHA for non-v-prefixed semver tag",
			actionDef: ActionDef{
				Owner:    "actions",
				Repo:     "checkout",
				RefOrSHA: "4",
			},
			mockSetup: func(mock *MockRepositoryService) {
				// Mock list tags response with a mix of v-prefixed and non-v-prefixed tags
				tags := []*gogithub.RepositoryTag{
					createTag("v4.0.0", "sha1"),
					createTag("4.0.0", "sha2"),
					createTag("4.1.0", "sha3"),
					createTag("v4.1.1", "sha4"),
					createTag("4.2.0", "sha5"),
					createTag("v5.0.0", "sha6"),
				}
				mock.EXPECT().
					ListTags(gomock.Any(), "actions", "checkout", gomock.Any()).
					Return(tags, &gogithub.Response{NextPage: 0}, nil)
			},
			expected: ResolvedVersion{
				CommitSHA:  "sha5", // 4.2.0 is the latest 4.x.x tag
				RefComment: "4.2.0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRepo := NewMockRepositoryService(ctrl)
			if tt.mockSetup != nil {
				tt.mockSetup(mockRepo)
			}

			resolver := NewVersionResolver(mockRepo, nil)

			result, err := resolver.ResolveVersion(context.Background(), tt.actionDef)
			require.NoError(t, err)
			assert.Equal(t, tt.expected.CommitSHA, result.CommitSHA)
			assert.Equal(t, tt.expected.RefComment, result.RefComment)
		})
	}
}

func TestVersionResolver_listSemverTagsAll(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := NewMockRepositoryService(ctrl)

	// Test with multiple pages of results
	mockRepo.EXPECT().
		ListTags(gomock.Any(), "owner", "repo", gomock.Any()).
		Return([]*gogithub.RepositoryTag{
			createTag("v1.0.0", "sha1"),
			createTag("v1.1.0", "sha2"),
		}, &gogithub.Response{NextPage: 2}, nil)

	mockRepo.EXPECT().
		ListTags(gomock.Any(), "owner", "repo", gomock.Any()).
		Return([]*gogithub.RepositoryTag{
			createTag("v2.0.0", "sha3"),
			createTag("not-semver", "sha4"), // This should be filtered out
		}, &gogithub.Response{NextPage: 0}, nil)

	resolver := NewVersionResolver(mockRepo, nil)

	tags, err := resolver.listSemverTagsAll(context.Background(), "owner", "repo")

	require.NoError(t, err)
	assert.Len(t, tags, 3) // only semver tags should be included

	// Verify the tags are correctly parsed
	assert.Equal(t, "1.0.0", tags[0].version.String())
	assert.Equal(t, "1.1.0", tags[1].version.String())
	assert.Equal(t, "2.0.0", tags[2].version.String())
}

func TestFindLatestTag(t *testing.T) {
	tests := []struct {
		name          string
		version       string
		tags          []string
		expectedTag   string
		expectedError bool
	}{
		{
			name:        "Find latest v4 tag",
			version:     "v4",
			tags:        []string{"v3.0.0", "v4.0.0", "v4.1.0", "v4.1.1", "v5.0.0"},
			expectedTag: "v4.1.1",
		},
		{
			name:        "Find latest v4.1 tag",
			version:     "v4.1",
			tags:        []string{"v4.0.0", "v4.1.0", "v4.1.1", "v4.2.0"},
			expectedTag: "v4.1.1",
		},
		{
			name:        "Find exact v4.1.0 tag",
			version:     "v4.1.0",
			tags:        []string{"v4.0.0", "v4.1.0", "v4.1.1", "v4.2.0"},
			expectedTag: "v4.1.0",
		},
		{
			name:          "No tags",
			version:       "v4",
			tags:          []string{},
			expectedError: true,
		},
		{
			name:          "No matching tags",
			version:       "v6",
			tags:          []string{"v3.0.0", "v4.0.0", "v5.0.0"},
			expectedError: true,
		},
		{
			name:        "Prerelease tags",
			version:     "v4",
			tags:        []string{"v4.0.0-alpha", "v4.0.0-beta", "v4.0.0", "v4.1.0-rc1"},
			expectedTag: "v4.0.0",
		},
		{
			name:        "Mixed version formats",
			version:     "v3",
			tags:        []string{"v3.0.0", "3.1.0", "v3.2.0", "3.2.1"},
			expectedTag: "3.2.1",
		},
		{
			name:        "Multiple prereleases",
			version:     "v2.1.0",
			tags:        []string{"v2.1.0-alpha.1", "v2.1.0-beta.1", "v2.1.0-rc.1", "v2.1.0-rc.2", "v2.1.0"},
			expectedTag: "v2.1.0",
		},
		{
			name:          "Only prereleases available",
			version:       "v1.0.0",
			tags:          []string{"v1.0.0-alpha.1", "v1.0.0-beta.1", "v1.0.0-rc.1"},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the version
			version, err := semver.NewVersion(tt.version)
			require.NoError(t, err)

			// Create semver tags
			var tags []semverTag
			for _, tagStr := range tt.tags {
				semver, _ := semver.NewVersion(tagStr)
				if semver != nil {
					tags = append(tags, semverTag{
						gogithubTag: gogithub.RepositoryTag{Name: &tagStr},
						version:     *semver,
					})
				}
			}

			// Find latest tag
			result, err := findLatestTag(*version, tags)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedTag, *result.gogithubTag.Name)
		})
	}
}

func TestActionDef_IsReusableWorkflow(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "Composite action - no path",
			path:     "",
			expected: false,
		},
		{
			name:     "Composite action - simple path",
			path:     "diff",
			expected: false,
		},
		{
			name:     "Composite action - deep path",
			path:     "tools/diff",
			expected: false,
		},
		{
			name:     "Composite action - path with subdirectories",
			path:     "path/to/action",
			expected: false,
		},
		{
			name:     "Reusable workflow - yml extension",
			path:     ".github/workflows/build.yml",
			expected: true,
		},
		{
			name:     "Reusable workflow - yaml extension",
			path:     ".github/workflows/test.yaml",
			expected: true,
		},
		{
			name:     "Reusable workflow - other extension",
			path:     "scripts/deploy.sh",
			expected: true,
		},
		{
			name:     "Reusable workflow - deep path with extension",
			path:     "path/to/workflow.yml",
			expected: true,
		},
		{
			name:     "Reusable workflow - multiple extensions",
			path:     "file.tar.gz",
			expected: true,
		},
		{
			name:     "Reusable workflow - extension in directory name",
			path:     "dir.ext/workflow.yml",
			expected: true,
		},
		{
			name:     "Composite action - extension only in directory name",
			path:     "dir.ext/action",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actionDef := ActionDef{
				Owner:    "test",
				Repo:     "repo",
				Path:     tt.path,
				RefOrSHA: "v1.0.0",
			}

			result := actionDef.IsReusableWorkflow()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Helper function to create a tag
func createTag(name, sha string) *gogithub.RepositoryTag {
	return &gogithub.RepositoryTag{
		Name: &name,
		Commit: &gogithub.Commit{
			SHA: &sha,
		},
	}
}
