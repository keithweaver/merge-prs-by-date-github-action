package main

import (
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
	// Validate required inputs
	repoOwner := os.Getenv("REPO_OWNER")
	repo := os.Getenv("REPO")
	githubToken := os.Getenv("GITHUB_ACCESS_TOKEN")

	if repoOwner == "" {
		log.Fatal("Error: REPO_OWNER is required")
	}
	if repo == "" {
		log.Fatal("Error: REPO is required")
	}
	if githubToken == "" {
		log.Fatal("Error: GITHUB_ACCESS_TOKEN is required")
	}

	fmt.Printf("Starting PR merge automation for %s/%s\n", repoOwner, repo)

	client := NewGitHubClient(githubToken, repoOwner, repo)

	// Fetch all open PRs (paginated)
	allPRs, err := client.GetAllOpenPullRequests()
	if err != nil {
		log.Fatalf("Failed to fetch pull requests: %v", err)
	}

	fmt.Printf("Found %d open pull requests\n", len(allPRs))

	now := time.Now()
	mergedCount := 0
	skippedCount := 0
	errorCount := 0

	for _, pr := range allPRs {
		fmt.Printf("\nProcessing PR #%d: %s\n", pr.Number, pr.Title)

		// Parse date from title
		prDate := ParseDateFromTitle(pr.Title)
		if prDate.IsZero() {
			fmt.Printf("  No valid date found in title, skipping\n")
			skippedCount++
			continue
		}

		fmt.Printf("  Found date: %s\n", prDate.Format("Jan 2"))

		// Check if date is in the past
		if prDate.After(now) {
			fmt.Printf("  Date is in the future, skipping\n")
			skippedCount++
			continue
		}

		fmt.Printf("  Date is in the past, attempting to merge...\n")

		// Attempt to merge the PR
		merged, err := client.MergePullRequest(pr.Number, pr.Title)
		if err != nil {
			fmt.Printf("  Failed to merge: %v\n", err)
			skippedCount++
			errorCount++
			continue
		}

		if merged {
			fmt.Printf("  Successfully merged PR #%d\n", pr.Number)
			mergedCount++
		} else {
			fmt.Printf("  PR #%d was not merged (may have conflicts or checks)\n", pr.Number)
			skippedCount++
		}
	}

	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Total PRs processed: %d\n", len(allPRs))
	fmt.Printf("PRs merged: %d\n", mergedCount)
	fmt.Printf("PRs skipped: %d\n", skippedCount)

	if errorCount > 0 {
		os.Exit(1)
	}
}
