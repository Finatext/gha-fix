package pin

import (
	"context"
	"os"
	"testing"

	"github.com/Finatext/gha-fix/internal/pin"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Short-cuts for internal types
type ResolvedVersion = pin.ResolvedVersion
type ActionDef = pin.ActionDef

func TestReplace(t *testing.T) {
	inputBytes, err := os.ReadFile("../testdata/pin.yml")
	require.NoError(t, err)
	input := string(inputBytes)
	expectedBytes, err := os.ReadFile("../testdata/pin-after.yml")
	require.NoError(t, err)
	expected := string(expectedBytes)

	// Define mock resolver results
	resolveResults := map[string]ResolvedVersion{
		"actions/checkout@v4": {
			CommitSHA:  "11bd71901bbe5b1630ceea73d27597364c9af683",
			RefComment: "v4.2.2",
		},
		"actions/checkout@v3": {
			CommitSHA:  "f43a0e5ff2bd294095638e18286ca9a3d1956744",
			RefComment: "v3.6.0",
		},
		"actions/checkout@v4.2": {
			CommitSHA:  "11bd71901bbe5b1630ceea73d27597364c9af683",
			RefComment: "v4.2.2",
		},
		"actions/setup-go@v5.4": {
			CommitSHA:  "0aaccfd150d50ccaeb58ebd88d36e91967a5f35b",
			RefComment: "v5.4.0",
		},
		"oasdiff/oasdiff-action@v0": {
			CommitSHA:  "1c611ffb1253a72924624aa4fb662e302b3565d3",
			RefComment: "v0.0.21",
		},
	}

	mock := &mockResolver{
		resolveResult: resolveResults,
	}
	r := &Pin{
		resolver:     mock,
		ignoreOwners: []string{"Finatext"},
	}
	got, changed, err := r.Apply(context.Background(), input)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, expected, got)
}

func TestIgnoreOwner(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expected       string
		changed        bool
		wantErr        bool
		ignoreOwners   []string
		resolveResults map[string]ResolvedVersion
	}{
		{
			name:           "Ignore owner - actions",
			input:          "- uses: actions/checkout@v4",
			expected:       "- uses: actions/checkout@v4",
			changed:        false,
			ignoreOwners:   []string{"actions"},
			resolveResults: map[string]ResolvedVersion{},
		},
		{
			name:         "Process non-ignored owner",
			input:        "- uses: someone/repo@v1",
			expected:     "- uses: someone/repo@abcdef1234567890abcdef1234567890abcdef12 # v1.0.0",
			changed:      true,
			ignoreOwners: []string{"actions"},
			resolveResults: map[string]ResolvedVersion{
				"someone/repo@v1": {
					CommitSHA:  "abcdef1234567890abcdef1234567890abcdef12",
					RefComment: "v1.0.0",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockResolver{
				resolveResult: tt.resolveResults,
			}
			r := &Pin{
				resolver:     mock,
				ignoreOwners: tt.ignoreOwners,
			}

			got, changed, err := r.replaceLine(context.Background(), tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
			assert.Equal(t, tt.changed, changed)
		})
	}
}

func TestIgnoreRepo(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expected       string
		changed        bool
		wantErr        bool
		ignoreRepos    []string
		resolveResults map[string]ResolvedVersion
	}{
		{
			name:           "Ignore specific repo",
			input:          "- uses: actions/checkout@v4",
			expected:       "- uses: actions/checkout@v4",
			changed:        false,
			ignoreRepos:    []string{"actions/checkout"},
			resolveResults: map[string]ResolvedVersion{},
		},
		{
			name:        "Process non-ignored repo",
			input:       "- uses: actions/setup-go@v1",
			expected:    "- uses: actions/setup-go@abcdef1234567890abcdef1234567890abcdef12 # v1.0.0",
			changed:     true,
			ignoreRepos: []string{"actions/checkout", "actions/setup-node"},
			resolveResults: map[string]ResolvedVersion{
				"actions/setup-go@v1": {
					CommitSHA:  "abcdef1234567890abcdef1234567890abcdef12",
					RefComment: "v1.0.0",
				},
			},
		},
		{
			name:           "Multiple ignored repos",
			input:          "- uses: actions/setup-node@v3",
			expected:       "- uses: actions/setup-node@v3",
			changed:        false,
			ignoreRepos:    []string{"actions/checkout", "actions/setup-node"},
			resolveResults: map[string]ResolvedVersion{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockResolver{
				resolveResult: tt.resolveResults,
			}
			r := &Pin{
				resolver:    mock,
				ignoreRepos: tt.ignoreRepos,
			}

			got, changed, err := r.replaceLine(context.Background(), tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
			assert.Equal(t, tt.changed, changed)
		})
	}
}

func TestCombinedIgnores(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expected       string
		changed        bool
		wantErr        bool
		ignoreOwners   []string
		ignoreRepos    []string
		resolveResults map[string]ResolvedVersion
	}{
		{
			name:           "Ignore by owner",
			input:          "- uses: actions/checkout@v4",
			expected:       "- uses: actions/checkout@v4",
			changed:        false,
			ignoreOwners:   []string{"actions"},
			ignoreRepos:    []string{"someone/repo"},
			resolveResults: map[string]ResolvedVersion{},
		},
		{
			name:           "Ignore by repo",
			input:          "- uses: someone/checkout@v4",
			expected:       "- uses: someone/checkout@v4",
			changed:        false,
			ignoreOwners:   []string{"actions"},
			ignoreRepos:    []string{"someone/checkout"},
			resolveResults: map[string]ResolvedVersion{},
		},
		{
			name:         "Process non-ignored",
			input:        "- uses: other/tool@v1",
			expected:     "- uses: other/tool@abcdef1234567890abcdef1234567890abcdef12 # v1.0.0",
			changed:      true,
			ignoreOwners: []string{"actions"},
			ignoreRepos:  []string{"someone/checkout"},
			resolveResults: map[string]ResolvedVersion{
				"other/tool@v1": {
					CommitSHA:  "abcdef1234567890abcdef1234567890abcdef12",
					RefComment: "v1.0.0",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockResolver{
				resolveResult: tt.resolveResults,
			}
			r := &Pin{
				resolver:     mock,
				ignoreOwners: tt.ignoreOwners,
				ignoreRepos:  tt.ignoreRepos,
			}

			got, changed, err := r.replaceLine(context.Background(), tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
			assert.Equal(t, tt.changed, changed)
		})
	}
}

func TestParseLine(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantDef     ActionDef
		wantOk      bool
		wantPrefix  string
		wantComment string
	}{
		{
			name:  "Standard format",
			input: "- uses: actions/checkout@v4",
			wantDef: ActionDef{
				Owner:    "actions",
				Repo:     "checkout",
				Path:     "",
				RefOrSHA: "v4",
			},
			wantOk:      true,
			wantPrefix:  "- uses: ",
			wantComment: "",
		},
		{
			name:  "With comment",
			input: "- uses: actions/checkout@v4 # Some comment",
			wantDef: ActionDef{
				Owner:    "actions",
				Repo:     "checkout",
				Path:     "",
				RefOrSHA: "v4",
			},
			wantOk:      true,
			wantPrefix:  "- uses: ",
			wantComment: "# Some comment",
		},
		{
			name:  "With path",
			input: "- uses: oasdiff/oasdiff-action/diff@v0",
			wantDef: ActionDef{
				Owner:    "oasdiff",
				Repo:     "oasdiff-action",
				Path:     "diff",
				RefOrSHA: "v0",
			},
			wantOk:      true,
			wantPrefix:  "- uses: ",
			wantComment: "",
		},
		{
			name:  "With deep path",
			input: "- uses: oasdiff/oasdiff-action/tools/diff@v0",
			wantDef: ActionDef{
				Owner:    "oasdiff",
				Repo:     "oasdiff-action",
				Path:     "tools/diff",
				RefOrSHA: "v0",
			},
			wantOk:      true,
			wantPrefix:  "- uses: ",
			wantComment: "",
		},
		{
			name:  "With commit SHA",
			input: "- uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683",
			wantDef: ActionDef{
				Owner:    "actions",
				Repo:     "checkout",
				Path:     "",
				RefOrSHA: "11bd71901bbe5b1630ceea73d27597364c9af683",
			},
			wantOk:      true,
			wantPrefix:  "- uses: ",
			wantComment: "",
		},
		{
			name:  "No dash prefix",
			input: "uses: actions/checkout@v4",
			wantDef: ActionDef{
				Owner:    "actions",
				Repo:     "checkout",
				Path:     "",
				RefOrSHA: "v4",
			},
			wantOk:      true,
			wantPrefix:  "uses: ",
			wantComment: "",
		},
		{
			name:  "With quoted YAML string",
			input: "      uses: \"actions/checkout@v4\"",
			wantDef: ActionDef{
				Owner:    "actions",
				Repo:     "checkout",
				Path:     "",
				RefOrSHA: "v4",
			},
			wantOk:      true,
			wantPrefix:  "      uses: ",
			wantComment: "",
		},
		{
			name:  "With quoted uses keyword",
			input: "      \"uses\": \"actions/checkout@v4\"",
			wantDef: ActionDef{
				Owner:    "actions",
				Repo:     "checkout",
				Path:     "",
				RefOrSHA: "v4",
			},
			wantOk:      true,
			wantPrefix:  "      \"uses\": ",
			wantComment: "",
		},
		{
			name:  "With single quoted uses keyword",
			input: "      'uses': 'actions/checkout@v4'",
			wantDef: ActionDef{
				Owner:    "actions",
				Repo:     "checkout",
				Path:     "",
				RefOrSHA: "v4",
			},
			wantOk:      true,
			wantPrefix:  "      'uses': ",
			wantComment: "",
		},
		{
			name:        "Not a uses line",
			input:       "run: echo hello",
			wantDef:     ActionDef{},
			wantOk:      false,
			wantPrefix:  "",
			wantComment: "",
		},
		{
			name:        "Empty line",
			input:       "",
			wantDef:     ActionDef{},
			wantOk:      false,
			wantPrefix:  "",
			wantComment: "",
		},
		{
			name:  "With single quotes",
			input: "- uses: 'actions/checkout@v4'",
			wantDef: ActionDef{
				Owner:    "actions",
				Repo:     "checkout",
				Path:     "",
				RefOrSHA: "v4",
			},
			wantOk:      true,
			wantPrefix:  "- uses: ",
			wantComment: "",
		},
		{
			name:  "With comment and indentation",
			input: "    uses: actions/checkout@v4  # This is a comment with spaces",
			wantDef: ActionDef{
				Owner:    "actions",
				Repo:     "checkout",
				Path:     "",
				RefOrSHA: "v4",
			},
			wantOk:      true,
			wantPrefix:  "    uses: ",
			wantComment: "# This is a comment with spaces",
		},
		{
			name:        "With leading comment",
			input:       "  # This is a leading comment\n- uses: actions/checkout@v4",
			wantDef:     ActionDef{},
			wantOk:      false, // Leading comment should not be parsed as an action line
			wantPrefix:  "",
			wantComment: "",
		},
		{
			name:        "With uses in the middle of the line",
			input:       "Some text with uses: actions/checkout@v4 in the middle",
			wantDef:     ActionDef{},
			wantOk:      false, // Should not match "uses" if not at the start of the line
			wantPrefix:  "",
			wantComment: "",
		},
		{
			name:        "With uses in a comment",
			input:       "# This comment has uses: actions/checkout@v4",
			wantDef:     ActionDef{},
			wantOk:      false, // Should not match "uses" in a comment
			wantPrefix:  "",
			wantComment: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotParsed, gotOk := parseLine(tt.input)
			assert.Equal(t, tt.wantOk, gotOk)

			if gotOk {
				assert.Equal(t, tt.wantDef, gotParsed.def)
				assert.Equal(t, tt.wantPrefix, gotParsed.prefix)
				assert.Equal(t, tt.wantComment, gotParsed.comment)
			}
		})
	}
}

func TestReplaceLine(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expected       string
		changed        bool
		wantErr        bool
		resolveResults map[string]ResolvedVersion
	}{
		{
			name:     "Simple action with version",
			input:    "- uses: actions/checkout@v4",
			expected: "- uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2",
			changed:  true,
			resolveResults: map[string]ResolvedVersion{
				"actions/checkout@v4": {
					CommitSHA:  "11bd71901bbe5b1630ceea73d27597364c9af683",
					RefComment: "v4.2.2",
				},
			},
		},
		{
			name:     "Action with subdirectory path",
			input:    "uses: oasdiff/oasdiff-action/diff@v0",
			expected: "uses: oasdiff/oasdiff-action/diff@1c611ffb1253a72924624aa4fb662e302b3565d3 # v0.0.21",
			changed:  true,
			resolveResults: map[string]ResolvedVersion{
				"oasdiff/oasdiff-action@v0": {
					CommitSHA:  "1c611ffb1253a72924624aa4fb662e302b3565d3",
					RefComment: "v0.0.21",
				},
			},
		},
		{
			name:     "Action with deep subdirectory path",
			input:    "uses: oasdiff/oasdiff-action/tools/diff@v0",
			expected: "uses: oasdiff/oasdiff-action/tools/diff@1c611ffb1253a72924624aa4fb662e302b3565d3 # v0.0.21",
			changed:  true,
			resolveResults: map[string]ResolvedVersion{
				"oasdiff/oasdiff-action@v0": {
					CommitSHA:  "1c611ffb1253a72924624aa4fb662e302b3565d3",
					RefComment: "v0.0.21",
				},
			},
		},
		{
			name:     "Action with version and comment",
			input:    "uses: actions/checkout@v4 # Existing comment",
			expected: "uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2 # Existing comment",
			changed:  true,
			resolveResults: map[string]ResolvedVersion{
				"actions/checkout@v4": {
					CommitSHA:  "11bd71901bbe5b1630ceea73d27597364c9af683",
					RefComment: "v4.2.2",
				},
			},
		},
		{
			name:           "Already has SHA",
			input:          "uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4",
			expected:       "uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4",
			changed:        false,
			resolveResults: map[string]ResolvedVersion{},
		},
		{
			name:           "Not an action line",
			input:          "run: echo hello",
			expected:       "run: echo hello",
			changed:        false,
			resolveResults: map[string]ResolvedVersion{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockResolver{
				resolveResult: tt.resolveResults,
			}
			r := &Pin{
				resolver:     mock,
				ignoreOwners: []string{},
			}

			got, changed, err := r.replaceLine(context.Background(), tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
			assert.Equal(t, tt.changed, changed)
		})
	}
}

type mockResolver struct {
	resolveResult map[string]ResolvedVersion
}

func (m *mockResolver) ResolveVersion(ctx context.Context, def ActionDef) (ResolvedVersion, error) {
	if def.HasCommitSHA() {
		return ResolvedVersion{}, pin.AlreadyResolvedError
	}

	// For subdirectories, we need to look up the base repo
	key := def.Owner + "/" + def.Repo
	if def.Path != "" {
		// Include path in the lookup key
		fullKey := key + "/" + def.Path + "@" + def.RefOrSHA
		if result, ok := m.resolveResult[fullKey]; ok {
			return result, nil
		}

		// Try without path
		key = key + "@" + def.RefOrSHA
	} else {
		key = key + "@" + def.RefOrSHA
	}

	if result, ok := m.resolveResult[key]; ok {
		// Special case to trigger AlreadyResolvedError for testing
		if result.CommitSHA == "AlreadyResolvedError" {
			return ResolvedVersion{}, pin.AlreadyResolvedError
		}
		return result, nil
	}

	return ResolvedVersion{}, errors.Newf("no mock result for %s", key)
}

func TestStrictPinning202508(t *testing.T) {
	tests := []struct {
		name                string
		input               string
		expected            string
		changed             bool
		wantErr             bool
		ignoreOwners        []string
		strictPinning202508 bool
		resolveResults      map[string]ResolvedVersion
	}{
		{
			name:                "Composite action - ignore owners disabled with strict pinning",
			input:               "- uses: actions/checkout@v4",
			expected:            "- uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2",
			changed:             true,
			ignoreOwners:        []string{"actions"},
			strictPinning202508: true,
			resolveResults: map[string]ResolvedVersion{
				"actions/checkout@v4": {
					CommitSHA:  "11bd71901bbe5b1630ceea73d27597364c9af683",
					RefComment: "v4.2.2",
				},
			},
		},
		{
			name:                "Reusable workflow - ignore owners respected with strict pinning",
			input:               "- uses: org/repo/.github/workflows/build.yml@main",
			expected:            "- uses: org/repo/.github/workflows/build.yml@main",
			changed:             false,
			ignoreOwners:        []string{"org"},
			strictPinning202508: true,
			resolveResults:      map[string]ResolvedVersion{},
		},
		{
			name:                "Composite action - normal ignore owners without strict pinning",
			input:               "- uses: actions/checkout@v4",
			expected:            "- uses: actions/checkout@v4",
			changed:             false,
			ignoreOwners:        []string{"actions"},
			strictPinning202508: false,
			resolveResults:      map[string]ResolvedVersion{},
		},
		{
			name:                "Composite action with path - strict pinning overrides ignore owners",
			input:               "- uses: myorg/myrepo/subaction@v1",
			expected:            "- uses: myorg/myrepo/subaction@abcdef1234567890abcdef1234567890abcdef12 # v1.0.0",
			changed:             true,
			ignoreOwners:        []string{"myorg"},
			strictPinning202508: true,
			resolveResults: map[string]ResolvedVersion{
				"myorg/myrepo@v1": {
					CommitSHA:  "abcdef1234567890abcdef1234567890abcdef12",
					RefComment: "v1.0.0",
				},
			},
		},
		{
			name:                "Reusable workflow with yaml extension - ignore owners respected",
			input:               "- uses: myorg/workflows/.github/workflows/ci.yaml@main",
			expected:            "- uses: myorg/workflows/.github/workflows/ci.yaml@main",
			changed:             false,
			ignoreOwners:        []string{"myorg"},
			strictPinning202508: true,
			resolveResults:      map[string]ResolvedVersion{},
		},
		{
			name:                "Non-ignored owner with strict pinning - should process normally",
			input:               "- uses: other/repo@v2",
			expected:            "- uses: other/repo@fedcba0987654321fedcba0987654321fedcba09 # v2.1.0",
			changed:             true,
			ignoreOwners:        []string{"actions"},
			strictPinning202508: true,
			resolveResults: map[string]ResolvedVersion{
				"other/repo@v2": {
					CommitSHA:  "fedcba0987654321fedcba0987654321fedcba09",
					RefComment: "v2.1.0",
				},
			},
		},
		{
			name:                "Reusable workflow without strict pinning - normal ignore behavior",
			input:               "- uses: org/repo/.github/workflows/deploy.yml@v1",
			expected:            "- uses: org/repo/.github/workflows/deploy.yml@v1",
			changed:             false,
			ignoreOwners:        []string{"org"},
			strictPinning202508: false,
			resolveResults:      map[string]ResolvedVersion{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockResolver{
				resolveResult: tt.resolveResults,
			}
			r := &Pin{
				resolver:            mock,
				ignoreOwners:        tt.ignoreOwners,
				strictPinning202508: tt.strictPinning202508,
			}

			got, changed, err := r.replaceLine(context.Background(), tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
			assert.Equal(t, tt.changed, changed)
		})
	}
}
