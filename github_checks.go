package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

func (c *GitHubClient) api(method, path string, body, result any) error {
	return c.apiWithToken(method, path, c.token, body, result)
}

func (c *GitHubClient) apiWithToken(method, path, token string, body, result any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, githubAPIBaseURL+"/repos/"+c.repoOwner+"/"+c.repo+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GitHub %s %s returned HTTP %d", method, path, resp.StatusCode)
	}
	if result == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(result)
}

func (c *GitHubClient) DefaultBranch() (string, error) {
	var repo Repository
	if err := c.api("GET", "", nil, &repo); err != nil {
		return "", err
	}
	if repo.DefaultBranch == "" {
		return "", fmt.Errorf("repository has no default branch")
	}
	return repo.DefaultBranch, nil
}
func (c *GitHubClient) PullRequest(number int) (PullRequest, error) {
	var pr PullRequest
	err := c.api("GET", fmt.Sprintf("/pulls/%d", number), nil, &pr)
	return pr, err
}
func (c *GitHubClient) Retarget(number int, target string) error {
	return c.api("PATCH", fmt.Sprintf("/pulls/%d", number), map[string]string{"base": target}, nil)
}

// Only retarget a child after the exact PR backing its current base was merged.
func (c *GitHubClient) ParentMerged(pr PullRequest, target string) (bool, error) {
	if pr.Base.Repo.FullName != c.repoOwner+"/"+c.repo {
		return false, nil
	}
	for page := 1; ; page++ {
		query := url.Values{"state": {"closed"}, "head": {c.repoOwner + ":" + pr.Base.Ref}, "base": {target}, "per_page": {"100"}, "page": {fmt.Sprint(page)}}
		var parents []PullRequest
		if err := c.api("GET", "/pulls?"+query.Encode(), nil, &parents); err != nil {
			return false, err
		}
		for _, parent := range parents {
			if parent.MergedAt != nil && parent.Head.Sha == pr.Base.Sha && parent.Head.Repo.FullName == pr.Base.Repo.FullName {
				return true, nil
			}
		}
		if len(parents) < 100 {
			return false, nil
		}
	}
}
func (c *GitHubClient) ChecksPassed(sha string) (bool, error) {
	var statuses struct {
		State string `json:"state"`
		Total int    `json:"total_count"`
	}
	if err := c.apiWithToken("GET", "/commits/"+sha+"/status", c.checksToken, nil, &statuses); err != nil {
		return false, err
	}
	if statuses.Total > 0 && statuses.State != "success" {
		return false, nil
	}
	count := statuses.Total
	for page := 1; ; page++ {
		var checks struct {
			Runs []struct {
				Status     string `json:"status"`
				Conclusion string `json:"conclusion"`
			} `json:"check_runs"`
		}
		if err := c.apiWithToken("GET", fmt.Sprintf("/commits/%s/check-runs?filter=latest&per_page=100&page=%d", sha, page), c.checksToken, nil, &checks); err != nil {
			return false, err
		}
		count += len(checks.Runs)
		for _, run := range checks.Runs {
			if run.Status != "completed" || (run.Conclusion != "success" && run.Conclusion != "neutral" && run.Conclusion != "skipped") {
				return false, nil
			}
		}
		if len(checks.Runs) < 100 {
			break
		}
	}
	return count > 0, nil
}
