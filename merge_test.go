package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func reply(value any) *http.Response {
	data, _ := json.Marshal(value)
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(data)))}
}
func boolPointer(value bool) *bool { return &value }
func readyPR(number int, title string) PullRequest {
	return PullRequest{Number: number, Title: title, State: "open", Head: Branch{Sha: fmt.Sprintf("head-%d", number), Repo: Repository{FullName: "owner/repo"}}, Base: Branch{Ref: "main", Repo: Repository{FullName: "owner/repo"}}, Mergeable: boolPointer(true), MergeableState: "clean"}
}

type fixture struct {
	prs        []PullRequest
	current    map[int]PullRequest
	parents    []PullRequest
	status     string
	conclusion string
	checkCount int
	runStatus  string
	writes     []string
	merges     []MergeRequest
	statusHTTP int
}

func (f *fixture) client(t *testing.T) *GitHubClient {
	t.Helper()
	c := NewGitHubClient("test-token", "owner", "repo")
	c.mergeMethod = "squash"
	c.client.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatal("missing authentication")
		}
		path := strings.TrimPrefix(r.URL.Path, "/repos/owner/repo")
		if r.Method != "GET" {
			f.writes = append(f.writes, r.Method+" "+path)
		}
		switch {
		case path == "":
			return reply(map[string]string{"default_branch": "main"}), nil
		case path == "/pulls" && r.URL.Query().Get("state") == "open":
			return reply(f.prs), nil
		case path == "/pulls" && r.URL.Query().Get("state") == "closed":
			return reply(f.parents), nil
		case strings.HasSuffix(path, "/merge"):
			var m MergeRequest
			if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
				t.Fatal(err)
			}
			f.merges = append(f.merges, m)
			return reply(MergeResponse{Merged: true}), nil
		case strings.HasPrefix(path, "/pulls/"):
			var number int
			fmt.Sscanf(path, "/pulls/%d", &number)
			if r.Method == "PATCH" {
				var body map[string]string
				json.NewDecoder(r.Body).Decode(&body)
				if body["base"] != "main" {
					t.Fatal("retargeted to wrong branch")
				}
				return reply(map[string]string{}), nil
			}
			pr, ok := f.current[number]
			if !ok {
				t.Fatalf("unexpected PR read %d", number)
			}
			return reply(pr), nil
		case strings.HasSuffix(path, "/status"):
			if f.statusHTTP != 0 {
				r := reply(map[string]string{})
				r.StatusCode = f.statusHTTP
				return r, nil
			}
			return reply(map[string]any{"state": f.status, "total_count": 1}), nil
		case strings.HasSuffix(path, "/check-runs"):
			runs := []map[string]string{}
			for i := 0; i < f.checkCount; i++ {
				runs = append(runs, map[string]string{"status": f.runStatus, "conclusion": f.conclusion})
			}
			return reply(map[string]any{"check_runs": runs}), nil
		}
		t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		return nil, nil
	})
	return c
}
func (f *fixture) run(t *testing.T) error {
	return Run(f.client(t), time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC), func(string, ...any) {})
}
func successful(prs ...PullRequest) *fixture {
	current := map[int]PullRequest{}
	for _, p := range prs {
		current[p.Number] = p
	}
	return &fixture{prs: prs, current: current, status: "success", conclusion: "success", runStatus: "completed", checkCount: 1}
}

func TestDuePRsMergeOldestFirstWithSquashAndHeadLease(t *testing.T) {
	f := successful(readyPR(2, "[Nov 15 2026] second"), readyPR(1, "[Nov 14 2026] first"))
	if err := f.run(t); err != nil {
		t.Fatal(err)
	}
	if len(f.merges) != 2 || f.merges[0].Sha != "head-1" || f.merges[1].Sha != "head-2" {
		t.Fatalf("unexpected merges %+v", f.merges)
	}
	for _, m := range f.merges {
		if m.MergeMethod != "squash" {
			t.Fatal("wrong strategy")
		}
	}
}
func TestDraftFutureAndUndatedPRsAreSkipped(t *testing.T) {
	draft := readyPR(1, "[Nov 14 2026] draft")
	draft.Draft = true
	f := successful(draft, readyPR(2, "[Nov 16 2026] future"), readyPR(3, "No date"))
	if err := f.run(t); err != nil {
		t.Fatal(err)
	}
	if len(f.writes) != 0 {
		t.Fatal(f.writes)
	}
}
func TestFreshDraftOrChangedFutureTitlePreventsMerge(t *testing.T) {
	for _, kind := range []string{"draft", "future"} {
		t.Run(kind, func(t *testing.T) {
			p := readyPR(1, "[Nov 14 2026] post")
			f := successful(p)
			if kind == "draft" {
				p.Draft = true
			} else {
				p.Title = "[Nov 16 2026] moved"
			}
			f.current[1] = p
			if err := f.run(t); err != nil {
				t.Fatal(err)
			}
			if len(f.writes) != 0 {
				t.Fatal(f.writes)
			}
		})
	}
}
func TestChecksMustFinishSuccessfully(t *testing.T) {
	for _, kind := range []string{"status-failure", "status-pending", "check-failure", "check-pending"} {
		t.Run(kind, func(t *testing.T) {
			f := successful(readyPR(1, "[Nov 14 2026] post"))
			switch kind {
			case "status-failure":
				f.status = "failure"
			case "status-pending":
				f.status = "pending"
			case "check-failure":
				f.conclusion = "failure"
			case "check-pending":
				f.runStatus = "in_progress"
			}
			if err := f.run(t); err != nil {
				t.Fatal(err)
			}
			if len(f.writes) != 0 {
				t.Fatal(f.writes)
			}
		})
	}
}
func TestUnmergedStackParentIsNeverMergedInto(t *testing.T) {
	p := readyPR(2, "[Nov 15 2026] child")
	p.Base.Ref = "previous"
	p.Base.Sha = "parent-head"
	f := successful(p)
	if err := f.run(t); err != nil {
		t.Fatal(err)
	}
	if len(f.writes) != 0 {
		t.Fatal(f.writes)
	}
}
func TestMergedParentOnlyRetargetsChildForNextRun(t *testing.T) {
	p := readyPR(2, "[Nov 15 2026] child")
	p.Base.Ref = "previous"
	p.Base.Sha = "parent-head"
	parent := readyPR(1, "[Nov 14 2026] parent")
	now := time.Now()
	parent.MergedAt = &now
	parent.Head.Sha = "parent-head"
	f := successful(p)
	f.parents = []PullRequest{parent}
	if err := f.run(t); err != nil {
		t.Fatal(err)
	}
	if len(f.writes) != 1 || f.writes[0] != "PATCH /pulls/2" || len(f.merges) != 0 {
		t.Fatal(f.writes)
	}
}
func TestStaleMergedParentDoesNotRetargetReusedBranch(t *testing.T) {
	p := readyPR(2, "[Nov 15 2026] child")
	p.Base.Ref = "previous"
	p.Base.Sha = "new-parent-head"
	parent := readyPR(1, "[Nov 14 2026] parent")
	now := time.Now()
	parent.MergedAt = &now
	parent.Head.Sha = "old-parent-head"
	f := successful(p)
	f.parents = []PullRequest{parent}
	if err := f.run(t); err != nil {
		t.Fatal(err)
	}
	if len(f.writes) != 0 {
		t.Fatal(f.writes)
	}
}
func TestGitHubCheckErrorsPreventMerge(t *testing.T) {
	f := successful(readyPR(1, "[Nov 14 2026] post"))
	f.statusHTTP = 403
	if err := f.run(t); err == nil {
		t.Fatal("expected API error")
	}
	if len(f.writes) != 0 {
		t.Fatal(f.writes)
	}
}
func TestMergeMethodValidation(t *testing.T) {
	for _, v := range []string{"merge", "squash", "rebase", ""} {
		if _, err := mergeMethod(v); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := mergeMethod("invalid"); err == nil {
		t.Fatal("invalid strategy accepted")
	}
}
func TestDatesUseConfiguredTimezoneAndSupportedTitleFormats(t *testing.T) {
	old := time.Local
	defer func() { time.Local = old }()
	location, err := time.LoadLocation("America/Toronto")
	if err != nil {
		t.Fatal(err)
	}
	time.Local = location
	for _, title := range []string{"[Nov 15 2026] post", "November 15 2026: post", "Nov 15 2026 / post", "[2026-11-15] post"} {
		got := ParseDateFromTitle(title)
		expected := time.Date(2026, 11, 15, 0, 0, 0, 0, location)
		if !got.Equal(expected) {
			t.Fatalf("%s: %v != %v", title, got, expected)
		}
		if !got.After(time.Date(2026, 11, 15, 3, 0, 0, 0, time.UTC)) {
			t.Fatal("published before Toronto midnight")
		}
	}
	if !ParseDateFromTitle("not dated").IsZero() {
		t.Fatal("accepted undated title")
	}
}
