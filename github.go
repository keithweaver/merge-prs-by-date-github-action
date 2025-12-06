package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	githubAPIBaseURL = "https://api.github.com"
	perPage          = 100
)

// GitHubClient handles GitHub API requests
type GitHubClient struct {
	token     string
	repoOwner string
	repo      string
	client    *http.Client
}

// NewGitHubClient creates a new GitHub API client
func NewGitHubClient(token, repoOwner, repo string) *GitHubClient {
	return &GitHubClient{
		token:     token,
		repoOwner: repoOwner,
		repo:      repo,
		client:    &http.Client{},
	}
}

// GetAllOpenPullRequests fetches all open pull requests with pagination
func (c *GitHubClient) GetAllOpenPullRequests() ([]PullRequest, error) {
	var allPRs []PullRequest
	page := 1

	for {
		prs, err := c.getPullRequestsPage(page)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch page %d: %w", page, err)
		}

		if len(prs) == 0 {
			break
		}

		allPRs = append(allPRs, prs...)

		// If we got fewer results than perPage, we've reached the last page
		if len(prs) < perPage {
			break
		}

		page++
	}

	return allPRs, nil
}

// getPullRequestsPage fetches a single page of pull requests
func (c *GitHubClient) getPullRequestsPage(page int) ([]PullRequest, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls?state=open&per_page=%d&page=%d",
		githubAPIBaseURL, c.repoOwner, c.repo, perPage, page)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(body))
	}

	var prs []PullRequest
	if err := json.NewDecoder(resp.Body).Decode(&prs); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return prs, nil
}

// MergePullRequest merges a pull request
func (c *GitHubClient) MergePullRequest(prNumber int, prTitle string) (bool, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/merge",
		githubAPIBaseURL, c.repoOwner, c.repo, prNumber)

	mergeReq := MergeRequest{
		CommitTitle:   fmt.Sprintf("Auto-merge: %s", prTitle),
		CommitMessage: "Automatically merged by date-based PR merger",
		MergeMethod:   "merge",
	}

	jsonData, err := json.Marshal(mergeReq)
	if err != nil {
		return false, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Success status codes are 200-299
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(body))
	}

	var mergeResp MergeResponse
	if err := json.Unmarshal(body, &mergeResp); err != nil {
		return false, fmt.Errorf("failed to decode response: %w", err)
	}

	return mergeResp.Merged, nil
}
