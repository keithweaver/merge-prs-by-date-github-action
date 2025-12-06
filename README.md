# Merge PRs by Date GitHub Action

A GitHub Action that automatically merges pull requests based on dates in their titles. This is useful for scheduling PRs to be merged on specific dates.

## How it Works

The action:

1. Fetches all open pull requests from your repository
2. Parses dates from PR titles using supported formats
3. Merges PRs whose dates are in the past

## Supported Date Formats

The action supports three date format patterns in PR titles:

### Bracket Format

```
[Dec 12] Add new feature
[December 12] Fix bug
[Jan 5 2024] Update documentation
```

### Colon Format

```
Dec 12: Add new feature
December 12: Fix bug
Jan 5 2024: Update documentation
```

### Slash Format

```
Dec 12 / Add new feature
December 12 / Fix bug
Jan 5 2024 / Update documentation
```

### Date Formats Supported

- `Dec 12`, `December 12` (uses current year)
- `Dec 12 2024`, `December 12 2024` (with year)
- `1/12`, `01/12` (numeric, uses current year)
- `1/12/2024`, `01/12/2024` (numeric with year)
- `2024-12-12` (ISO format)

## Usage

### Scheduled Workflow

Create a workflow file (e.g., [.github/workflows/scheduled-merge.yml](.github/workflows/scheduled-merge.yml)):

```yaml
name: Scheduled PR Merge

on:
  schedule:
    # Run every day at midnight UTC
    - cron: "0 0 * * *"
  workflow_dispatch: # Allow manual triggering

jobs:
  merge-prs:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Merge PRs by Date
        uses: keithweaver/merge-prs-by-date-github-action@v1.0.0
        with:
          repo_owner: ${{ github.repository_owner }}
          repo: ${{ github.event.repository.name }}
          github_access_token: ${{ secrets.GITHUB_TOKEN }}
```

### Manual Workflow

```yaml
name: Manual PR Merge

on:
  workflow_dispatch:

jobs:
  merge-prs:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: yourusername/merge-prs-by-date-github-action@v1.0.0
        with:
          repo_owner: myorg
          repo: myrepo
          github_access_token: ${{ secrets.GITHUB_TOKEN }}
```

## Inputs

### `repo_owner`

**Required** The owner of the repository (username or organization).

### `repo`

**Required** The repository name.

### `github_access_token`

**Required** GitHub access token with `repo` permissions. You can use `${{ secrets.GITHUB_TOKEN }}` for actions running in the same repository.

## Example

If you have a PR titled `[Dec 4] Deploy new feature` and today is December 5th, the action will automatically merge this PR when it runs.

## Permissions

The GitHub token needs the following permissions:

- `pull_requests: write` - to merge PRs
- `contents: read` - to read PR information

## Development

### Local Testing

Build and test locally:

```bash
cd /Users/keithweaver/go/src/temp/merge-prs-by-date-github-action
docker build -t pr-merger .
docker run -e INPUT_REPO_OWNER=owner -e INPUT_REPO=repo -e INPUT_GITHUB_ACCESS_TOKEN=token pr-merger
```

### Files

- [main.go](main.go) - Main orchestration and validation
- [github.go](github.go) - GitHub API client
- [github_models.go](github_models.go) - API response structs
- [date_parser.go](date_parser.go) - Date parsing logic
- [action.yml](action.yml) - Action metadata
- [Dockerfile](Dockerfile) - Multi-stage build configuration
- [entrypoint.sh](entrypoint.sh) - Entry point script

## License

MIT
