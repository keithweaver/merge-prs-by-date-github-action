package main

import (
	"fmt"
	"os"
	"sort"
	"time"
)

func mergeMethod(value string) (string, error) {
	if value == "" {
		value = "merge"
	}
	switch value {
	case "merge", "squash", "rebase":
		return value, nil
	}
	return "", fmt.Errorf("invalid merge_method %q", value)
}

// Run processes all due PRs, independently of any series or branch naming scheme.
func Run(client *GitHubClient, now time.Time, logf func(string, ...any)) error {
	target, err := client.DefaultBranch()
	if err != nil {
		return err
	}
	prs, err := client.GetAllOpenPullRequests()
	if err != nil {
		return err
	}
	sort.SliceStable(prs, func(i, j int) bool {
		a, b := ParseDateFromTitle(prs[i].Title), ParseDateFromTitle(prs[j].Title)
		if a.Equal(b) {
			return prs[i].Number < prs[j].Number
		}
		return a.Before(b)
	})
	logf("Scanning %s/%s: %d open PRs, target=%s, timezone=%s, now=%s", client.repoOwner, client.repo, len(prs), target, now.Location(), now.Format(time.RFC3339))
	mergedCount, retargetedCount, errorCount := 0, 0, 0
	defer func() {
		logf("Summary: scanned=%d merged=%d retargeted=%d skipped=%d errors=%d", len(prs), mergedCount, retargetedCount, len(prs)-mergedCount-retargetedCount-errorCount, errorCount)
	}()
	var firstError error
	for _, candidate := range prs {
		date := ParseDateFromTitle(candidate.Title)
		if candidate.Draft {
			logf("#%d: skipped: draft", candidate.Number)
			continue
		}
		if date.IsZero() {
			logf("#%d: skipped: no recognized date in title", candidate.Number)
			continue
		}
		if date.After(now) {
			logf("#%d: skipped: date %s is in the future", candidate.Number, date.Format("2006-01-02"))
			continue
		}
		pr, err := client.PullRequest(candidate.Number)
		if err != nil {
			firstError = err
			errorCount++
			logf("#%d: %v", candidate.Number, err)
			continue
		}
		date = ParseDateFromTitle(pr.Title)
		if pr.Draft || pr.State != "open" || date.IsZero() || date.After(now) {
			logf("#%d: skipped: PR state or date changed (state=%s draft=%v)", pr.Number, pr.State, pr.Draft)
			continue
		}
		if pr.Base.Ref != target {
			ready, err := client.ParentMerged(pr, target)
			if err != nil {
				firstError = err
				errorCount++
				logf("#%d: %v", pr.Number, err)
				continue
			}
			if !ready {
				logf("#%d: waiting for its base PR to merge", pr.Number)
				continue
			}
			if err = client.Retarget(pr.Number, target); err != nil {
				firstError = err
				errorCount++
				logf("#%d: %v", pr.Number, err)
				continue
			}
			retargetedCount++
			logf("#%d: retargeted to %s; wait for checks on the next scheduled run", pr.Number, target)
			continue
		}
		if pr.Mergeable == nil || !*pr.Mergeable || pr.MergeableState != "clean" {
			logf("#%d: not mergeable yet", pr.Number)
			continue
		}
		passed, err := client.ChecksPassed(pr.Head.Sha)
		if err != nil {
			firstError = err
			errorCount++
			logf("#%d: %v", pr.Number, err)
			continue
		}
		if !passed {
			logf("#%d: waiting for successful checks", pr.Number)
			continue
		}
		merged, err := client.MergePullRequestAtSHA(pr.Number, pr.Title, pr.Head.Sha)
		if err != nil {
			firstError = err
			errorCount++
			logf("#%d: %v", pr.Number, err)
			continue
		}
		if !merged {
			firstError = fmt.Errorf("GitHub did not merge #%d", pr.Number)
			errorCount++
			logf("%v", firstError)
			continue
		}
		mergedCount++
		logf("#%d: merged=%v", pr.Number, merged)
	}
	return firstError
}

func configure() (*time.Location, string, error) {
	method, err := mergeMethod(os.Getenv("MERGE_METHOD"))
	if err != nil {
		return nil, "", err
	}
	zone := os.Getenv("TIMEZONE")
	if zone == "" {
		zone = "UTC"
	}
	location, err := time.LoadLocation(zone)
	if err != nil {
		return nil, "", fmt.Errorf("invalid timezone: %w", err)
	}
	return location, method, nil
}
