package timeout

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFixer_Fix_Integration(t *testing.T) {
	input, err := os.ReadFile("../../testdata/timeout.yml")
	require.NoError(t, err)
	expected, err := os.ReadFile("../../testdata/timeout-after.yml")
	require.NoError(t, err)

	f := Fixer{TimeoutMinutes: 5}
	got, changed, err := f.Fix(context.Background(), string(input))
	require.NoError(t, err)

	assert.True(t, changed)
	assert.Equal(t, string(expected), got)
}

func TestFixer_Fix_EdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		timeoutMinutes uint64
		input          string
		expected       string
		wantChanged    bool
		wantErr        bool
	}{
		{
			name:           "empty file",
			timeoutMinutes: 5,
			input:          "",
			expected:       "",
			wantChanged:    false,
			wantErr:        false,
		},
		{
			name:           "no jobs section",
			timeoutMinutes: 5,
			input: `name: Test
on: push`,
			expected: `name: Test
on: push`,
			wantChanged: false,
			wantErr:     false,
		},
		{
			name:           "job with uses field should be skipped",
			timeoutMinutes: 5,
			input: `jobs:
  reusable:
    uses: owner/repo/.github/workflows/workflow.yml@main`,
			expected: `jobs:
  reusable:
    uses: owner/repo/.github/workflows/workflow.yml@main`,
			wantChanged: false,
			wantErr:     false,
		},
		{
			name:           "job already has timeout",
			timeoutMinutes: 5,
			input: `jobs:
  test:
    timeout-minutes: 10
    runs-on: ubuntu-latest`,
			expected: `jobs:
  test:
    timeout-minutes: 10
    runs-on: ubuntu-latest`,
			wantChanged: false,
			wantErr:     false,
		},
		{
			name:           "preserve indentation",
			timeoutMinutes: 5,
			input: `jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3`,
			expected: `jobs:
  test:
    timeout-minutes: 5
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3`,
			wantChanged: true,
			wantErr:     false,
		},
		{
			name:           "invalid yaml",
			timeoutMinutes: 5,
			input:          `jobs: [invalid`,
			expected:       `jobs: [invalid`,
			wantChanged:    false,
			wantErr:        false,
		},
		{
			name:           "non-github-workflow yaml with jobs and runs-on",
			timeoutMinutes: 5,
			input: `# This is a Kubernetes config file
kind: Deployment
apiVersion: apps/v1
metadata:
  name: example
jobs:
  example:
    runs-on: ubuntu-latest`,
			expected: `# This is a Kubernetes config file
kind: Deployment
apiVersion: apps/v1
metadata:
  name: example
jobs:
  example:
    runs-on: ubuntu-latest`,
			wantChanged: false,
			wantErr:     false,
		},
		{
			name:           "regular config yaml with jobs key",
			timeoutMinutes: 5,
			input: `# Config with jobs key but not a GitHub workflow
config:
  jobs:
    task1:
      runs-on: machine1`,
			expected: `# Config with jobs key but not a GitHub workflow
config:
  jobs:
    task1:
      runs-on: machine1`,
			wantChanged: false,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := Fixer{TimeoutMinutes: tt.timeoutMinutes}
			got, changed, err := f.Fix(context.Background(), tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			assert.Equal(t, tt.wantChanged, changed)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestGetJobPropertyIndent(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		jobLine  int
		expected string
		wantErr  bool
	}{
		{
			name: "standard 2 space indent",
			lines: []string{
				"jobs:",
				"  test:",
				"    runs-on: ubuntu-latest",
			},
			jobLine:  2,
			expected: "    ",
		},
		{
			name: "4 space indent",
			lines: []string{
				"jobs:",
				"    test:",
				"        runs-on: ubuntu-latest",
			},
			jobLine:  2,
			expected: "        ",
		},
		{
			name: "no properties found",
			lines: []string{
				"jobs:",
				"  test:",
				"    # just comments",
				"		 # another comment",
			},
			jobLine: 2,
			wantErr: true,
		},
		{
			name: "out of bounds line",
			lines: []string{
				"jobs:",
				"  test:",
			},
			jobLine: 5,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getJobPropertyIndent(tt.lines, tt.jobLine)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestFixer_Fix_MultipleJobs(t *testing.T) {
	input := `jobs:
  has-timeout:
    timeout-minutes: 10
    runs-on: ubuntu-latest
  needs-timeout:
    runs-on: ubuntu-latest
  reusable:
    uses: owner/repo/.github/workflows/workflow.yml@main`

	expected := `jobs:
  has-timeout:
    timeout-minutes: 10
    runs-on: ubuntu-latest
  needs-timeout:
    timeout-minutes: 5
    runs-on: ubuntu-latest
  reusable:
    uses: owner/repo/.github/workflows/workflow.yml@main`

	f := Fixer{TimeoutMinutes: 5}
	got, changed, err := f.Fix(context.Background(), input)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, expected, got)
}

// Generative AI is not good to edit large table driven tests, so we will add individual tests for specific cases
func TestFixer_Fix_FlowStyle(t *testing.T) {
	input := `jobs:
  flow-job: { runs-on: ubuntu-latest, steps: [ { run: echo test } ] }`

	f := Fixer{TimeoutMinutes: 5}
	got, changed, err := f.Fix(context.Background(), input)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrFlowStyleNotSupported))
	assert.False(t, changed)
	assert.Equal(t, input, got)
}

func TestFixer_Fix_CompactJobSyntax(t *testing.T) {
	// Job definition without space after colon
	input := `jobs:
  compact:runs-on: ubuntu-latest`

	f := Fixer{TimeoutMinutes: 5}
	_, _, err := f.Fix(context.Background(), input)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrCompactJobSyntaxNotSupported))
}

func TestFixer_Fix_EmptyJobs(t *testing.T) {
	// Empty jobs should be skipped
	input := `jobs:
  empty-job:

  empty-with-comment:
    # This job is empty

  has-content:
    runs-on: ubuntu-latest`

	expected := `jobs:
  empty-job:

  empty-with-comment:
    # This job is empty

  has-content:
    timeout-minutes: 5
    runs-on: ubuntu-latest`

	f := Fixer{TimeoutMinutes: 5}
	got, changed, err := f.Fix(context.Background(), input)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, expected, got)
}

func TestFixer_Fix_TimeoutWithExpression(t *testing.T) {
	// Jobs with timeout expressions should be skipped
	input := `jobs:
  has-expression:
    timeout-minutes: ${{ inputs.timeout }}
    runs-on: ubuntu-latest
  has-number:
    timeout-minutes: 10
    runs-on: ubuntu-latest
  needs-timeout:
    runs-on: ubuntu-latest`

	expected := `jobs:
  has-expression:
    timeout-minutes: ${{ inputs.timeout }}
    runs-on: ubuntu-latest
  has-number:
    timeout-minutes: 10
    runs-on: ubuntu-latest
  needs-timeout:
    timeout-minutes: 5
    runs-on: ubuntu-latest`

	f := Fixer{TimeoutMinutes: 5}
	got, changed, err := f.Fix(context.Background(), input)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, expected, got)
}

func TestFixer_Fix_SpecialJobNames(t *testing.T) {
	// Jobs with special characters in names
	input := `jobs:
  "quoted job name":
    runs-on: ubuntu-latest
  'single-quoted':
    runs-on: ubuntu-latest
  job-with-dashes:
    runs-on: ubuntu-latest
  job_with_underscores:
    runs-on: ubuntu-latest`

	f := Fixer{TimeoutMinutes: 5}
	got, changed, err := f.Fix(context.Background(), input)
	require.NoError(t, err)
	assert.True(t, changed)
	// All jobs should get timeout-minutes
	assert.Equal(t, 4, strings.Count(got, "timeout-minutes: 5"))
}

func TestFixer_Fix_JobsWithoutRunsOn(t *testing.T) {
	// Workflow with jobs that don't have runs-on key
	input := `name: Test Workflow
on: push
jobs:
  job-with-runs-on:
    runs-on: ubuntu-latest
    steps:
      - run: echo hello
  job-without-runs-on:
    steps:
      - run: echo world
  container-job:
    container:
      image: node:14.16
    steps:
      - run: npm test`

	expected := `name: Test Workflow
on: push
jobs:
  job-with-runs-on:
    timeout-minutes: 5
    runs-on: ubuntu-latest
    steps:
      - run: echo hello
  job-without-runs-on:
    timeout-minutes: 5
    steps:
      - run: echo world
  container-job:
    timeout-minutes: 5
    container:
      image: node:14.16
    steps:
      - run: npm test`

	f := Fixer{TimeoutMinutes: 5}
	got, changed, err := f.Fix(context.Background(), input)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, expected, got)
}

func TestFixer_Fix_NonWorkflowFile(t *testing.T) {
	// Workflow with jobs that don't have runs-on key
	input := `name: Test Workflow
replace:
  on: push
jobs:
  job-with-runs-on:
    steps:
      - run: echo hello`

	f := Fixer{TimeoutMinutes: 5}
	_, changed, err := f.Fix(context.Background(), input)
	require.NoError(t, err)
	assert.False(t, changed)
}
