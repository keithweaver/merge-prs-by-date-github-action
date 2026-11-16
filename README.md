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
        uses: keithweaver/merge-prs-by-date-github-action@v1.0.5
        with:
          repo_owner: ${{ github.repository_owner }}
          repo: ${{ github.event.repository.name }}
          github_access_token: ${{ secrets.G_ACCESS_TOKEN }} # Secrets cannot start with GITHUB_
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

      - uses: keithweaver/merge-prs-by-date-github-action@v1.0.5
        with:
          repo_owner: myorg
          repo: myrepo
          github_access_token: ${{ secrets.G_ACCESS_TOKEN }} # Secrets cannot start with GITHUB_
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
# cd into repo (merge-prs-by-date-github-action)
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

## Merge strategy, timezone, and stacked PRs

Optional inputs:

| Input | Default | Behavior |
| --- | --- | --- |
| `merge_method` | `merge` | `merge`, `squash`, or `rebase`; use `squash` for squash-only repositories. |
| `timezone` | `UTC` | IANA timezone used for dates, for example `America/Toronto`. |

The action processes all due, non-draft PRs oldest first. It uses the repository's
default branch and requires successful status checks and completed check runs
before merging. Pending, failing, conflicting, or not-yet-evaluated PRs wait for
the next scheduled run. The merge request includes the inspected head SHA so a
concurrent push cannot merge untested code.

A stacked PR is never merged into its parent branch. Once the exact PR backing
its base branch has merged into the default branch, the action retargets the child
to the default branch and waits for a later run before merging it. No series
manifest, hardcoded branch names, force pushes, or repository-specific workflow
is needed. Keep your workflow on a recurring schedule so retargeted PRs can finish
their checks before the next attempt.

The GitHub token needs repository read access (including checks and commit
statuses), pull-request write access for retargeting, and contents write access
for merges. The action never runs code from a PR branch.

### Separate check-reading token

Fine-grained PATs cannot access the check-runs endpoint. Set `checks_token` to
`${{ github.token }}` and grant that workflow token `checks: read` and
`statuses: read`. Keep `github_access_token` set to your PAT for merges and PR
retargeting so those writes can still trigger downstream workflows. If
`checks_token` is omitted, the action preserves its single-token behavior.

Each run logs skipped PRs and a scanned/merged/retargeted/skipped/error summary.
