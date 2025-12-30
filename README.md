# gha-fix

gha-fix automates security and maintenance fixes in GitHub Actions workflows. It provides commands to address common issues in workflow files.

## Features

- **Pin GitHub Actions**: Converts version references to specific commit SHAs for improved security
- **Add Timeouts**: Adds `timeout-minutes` to GitHub Actions jobs to prevent workflows from running for too long

## Installation

### Using Go

```bash
go install github.com/Finatext/gha-fix@latest
```

## Usage

### pin

Pin GitHub Actions used in workflow files (.yml or .yaml) to specific commit SHAs.

This command scans GitHub Actions in workflow files and replaces references like 'owner/repo@v1' with specific commit SHAs like 'owner/repo@8843d7f53bd34e3b78f2acee556ba5d53feae7c4'.

```bash
gha-fix pin [file1 file2 ...] [flags]
```

If no files are specified, all workflow files (.yml or .yaml) in the current directory and subdirectories will be processed.

#### GitHub Token Configuration
`GITHUB_TOKEN` is required to fetch tags and commit SHAs from GitHub. Can be provided via environment variable or other ways.

#### GitHub API Server (GHES support)

By default, `gha-fix` uses the GitHub.com API (`https://api.github.com/`). To use GitHub Enterprise Server (GHES) or any other deployment, set the **full API base URL**.

Supported configuration (highest priority first):

1. CLI flag: `gha-fix pin --api-server <FULL_API_BASE_URL>`
2. Config file key: `pin.api-server`
3. Environment variable: `GITHUB_API_URL`
4. Default: `https://api.github.com/`

Example (GHES):

```bash
export GITHUB_TOKEN=...
export GITHUB_API_URL="https://github.enterprise.company.com/api/v3/"
gha-fix pin
```

Or with config file (`gha-fix.yaml`):

```yaml
pin:
  api-server: "https://github.enterprise.company.com/api/v3/"
```

Note: `api-server` must be the **full API base URL** for your deployment. `gha-fix` will not assume `/api/v3`.

#### Strict SHA Pinning (--strict-pinning-202508)

The `--strict-pinning-202508` option implements support for GitHub's SHA pinning enforcement policy announced in August 2025. When enabled, this option modifies the behavior of ignore-owners:

- **Actions, composite actions** (e.g., `my-org/repo@v1`, `my-org/repo/path/to/action@v4`) will be pinned to SHAs even if their owner is specified in `--ignore-owners` to follow SHA pinning enforcement policy
- **Reusable workflows** (e.g., `org/repo/.github/workflows/build.yml@main`) will still respect the `--ignore-owners` setting

This differentiation allows organizations to comply with GitHub's security policies for composite actions while maintaining flexibility for reusable workflows. The tool distinguishes between composite actions and reusable workflows based on whether the action path contains a file extension.

Reference: [GitHub Actions policy now supports blocking and SHA pinning actions](https://github.blog/changelog/2025-08-15-github-actions-policy-now-supports-blocking-and-sha-pinning-actions/)

#### Example

```bash
# Process a specific workflow file
gha-fix pin .github/workflows/deploy.yml

# Process all workflow files in the current directory and subdirectories
gha-fix pin

# Ignore specific owners
gha-fix pin --ignore-owners=actions,github

# Enable strict SHA pinning for composite actions (GitHub's SHA pinning enforcement policy)
gha-fix pin --strict-pinning-202508

# Use GHES API server explicitly
gha-fix pin --api-server "https://github.enterprise.company.com/api/v3/"

# Ignore specific directories when searching for workflow files (global option)
# This will skip any directory with these names, including in subdirectories (e.g., abc/def/node_modules/)
gha-fix --ignore-dirs=.git,node_modules,dist,out,vendor,.idea,.vscode pin
```

### timeout

Add `timeout-minutes` to GitHub Actions workflow jobs that don't have one defined.

This command scans GitHub Actions workflow files and adds a `timeout-minutes` parameter to jobs without it. Jobs using reusable workflows (with 'uses' field) are automatically skipped since they don't directly support setting timeouts.

```bash
gha-fix timeout [file1 file2 ...] [flags]
```

If no files are specified, all workflow files (.yml or .yaml) in the current directory and subdirectories will be processed.

#### Example

```bash
# Add default timeout (5 minutes) to all workflow files
gha-fix timeout

# Set custom timeout value for specific workflow file
gha-fix timeout .github/workflows/deploy.yml --timeout-value 10

# Process all workflow files with custom timeout value
gha-fix timeout -t 15

# Process all workflow files with custom timeout value and ignore specific directories
gha-fix --ignore-dirs=node_modules,dist timeout -t 15
```

## Acknowledgements

`gha-fix` adopts a text-based processing strategy for GitHub Actions workflow files, an approach inspired by [suzuki-shunsuke/pinact](https://github.com/suzuki-shunsuke/pinact).

In addition to this inspiration, `gha-fix` was developed to support new features and behavioral changes that better fit our use case. These include:

- Updating actions even when a branch name is specified, rather than failing.
- Exposing a Go interface that's easy to call from within our own tools.
- Scanning all directories by default — not just `.github` — to support reusable workflows placed elsewhere.

## Development
### Release
Create a Git tag and push it. The CI/CD pipeline will take care of the release process.

