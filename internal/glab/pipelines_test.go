package glab

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestFetchPipelineJobs(t *testing.T) {
	dir := t.TempDir()
	response := `[{"id":100,"name":"build","stage":"build","status":"success"}]`
	script := fakeGlabScript(t, dir, response)

	c := &Client{BinaryPath: script}
	jobs, err := c.FetchPipelineJobs(context.Background(), 42, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Name != "build" {
		t.Errorf("expected job name 'build', got %q", jobs[0].Name)
	}
}

func TestFetchPipelines_ParseError(t *testing.T) {
	dir := t.TempDir()
	script := fakeGlabScript(t, dir, "not json")

	c := &Client{BinaryPath: script}
	_, err := c.FetchPipelinesByUser(context.Background(), 42, "testuser", time.Now().Add(-24*time.Hour))
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestFetchPipelineJobs_EmptyResponse(t *testing.T) {
	dir := t.TempDir()
	script := fakeGlabScript(t, dir, "[]")

	c := &Client{BinaryPath: script}
	jobs, err := c.FetchPipelineJobs(context.Background(), 42, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("expected 0 jobs, got %d", len(jobs))
	}
}

func TestFetchPipeline(t *testing.T) {
	dir := t.TempDir()
	response := `{"id":99,"sha":"abc123","ref":"main","status":"success","web_url":"https://gitlab.com/p/99"}`
	script := fakeGlabScript(t, dir, response)

	c := &Client{BinaryPath: script}
	p, err := c.FetchPipeline(context.Background(), 42, 99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.ID != 99 {
		t.Errorf("ID = %d, want 99", p.ID)
	}
	if p.Status != "success" {
		t.Errorf("Status = %q, want \"success\"", p.Status)
	}
}

func TestFetchPipeline_ParseError(t *testing.T) {
	dir := t.TempDir()
	script := fakeGlabScript(t, dir, "not json")

	c := &Client{BinaryPath: script}
	_, err := c.FetchPipeline(context.Background(), 42, 99)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestFetchPipeline_RunError(t *testing.T) {
	c := &Client{BinaryPath: "/nonexistent/glab"}
	_, err := c.FetchPipeline(context.Background(), 42, 99)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFetchPipelineJobs_RunError(t *testing.T) {
	c := &Client{BinaryPath: "/nonexistent/glab"}
	_, err := c.FetchPipelineJobs(context.Background(), 42, 99)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFetchPipelineJobs_ParseError(t *testing.T) {
	dir := t.TempDir()
	script := fakeGlabScript(t, dir, "not json")

	c := &Client{BinaryPath: script}
	_, err := c.FetchPipelineJobs(context.Background(), 42, 1)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestFetchProjectJobs(t *testing.T) {
	dir := t.TempDir()
	argsPath := filepath.Join(dir, "args")
	now := time.Now()
	response := fmt.Sprintf(`[{"id":1,"name":"build","status":"success","created_at":%q}]`, now.Format(time.RFC3339))
	script := fakeGlabScriptWithArgs(t, dir, argsPath, response)

	c := &Client{BinaryPath: script}
	cutoff := now.Add(-1 * time.Hour)
	jobs, err := c.FetchProjectJobs(context.Background(), 42, cutoff, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("reading args: %v", err)
	}
	want := "api projects/42/jobs?order_by=id&sort=desc&per_page=25&page=1"
	if string(args) != want {
		t.Errorf("args = %q, want %q", string(args), want)
	}
}

func TestFetchProjectJobs_StopsAtCutoff(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("paged fake script is unix-only")
	}
	dir := t.TempDir()
	argsPath := filepath.Join(dir, "args")
	now := time.Now()

	// Page 1 is a full page (matches per_page) with a mix of jobs after and
	// before the cutoff, so fetching should stop mid-page without a page 2 call.
	page1 := fmt.Sprintf(
		`[{"id":3,"name":"c","status":"success","created_at":%q},{"id":2,"name":"b","status":"success","created_at":%q},{"id":1,"name":"a","status":"success","created_at":%q}]`,
		now.Format(time.RFC3339),
		now.Add(-2*time.Hour).Format(time.RFC3339),
		now.Add(-3*time.Hour).Format(time.RFC3339),
	)
	script := fakeGlabScriptPagedWithArgs(t, dir, argsPath, map[int]string{1: page1})

	c := &Client{BinaryPath: script}
	cutoff := now.Add(-1 * time.Hour)
	jobs, err := c.FetchProjectJobs(context.Background(), 42, cutoff, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job (only 'c' is after cutoff), got %d", len(jobs))
	}
	if jobs[0].Name != "c" {
		t.Errorf("jobs[0].Name = %q, want \"c\"", jobs[0].Name)
	}

	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("reading args: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(args), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected only 1 page fetched, got %d calls: %v", len(lines), lines)
	}
}

func TestFetchProjectJobs_LimitCap(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	response := fmt.Sprintf(
		`[{"id":2,"name":"b","status":"success","created_at":%q},{"id":1,"name":"a","status":"success","created_at":%q}]`,
		now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	script := fakeGlabScript(t, dir, response)

	c := &Client{BinaryPath: script}
	jobs, err := c.FetchProjectJobs(context.Background(), 42, now.Add(-1*time.Hour), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job (capped by limit), got %d", len(jobs))
	}
}

func TestFetchProjectJobs_RunError(t *testing.T) {
	c := &Client{BinaryPath: "/nonexistent/glab"}
	_, err := c.FetchProjectJobs(context.Background(), 42, time.Now(), 25)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFetchProjectJobs_ParseError(t *testing.T) {
	dir := t.TempDir()
	script := fakeGlabScript(t, dir, "not json")

	c := &Client{BinaryPath: script}
	_, err := c.FetchProjectJobs(context.Background(), 42, time.Now(), 25)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestFetchProjectJobsSince(t *testing.T) {
	dir := t.TempDir()
	response := `[{"id":5,"name":"new-job","status":"success"}]`
	script := fakeGlabScript(t, dir, response)

	c := &Client{BinaryPath: script}
	jobs, err := c.FetchProjectJobsSince(context.Background(), 42, 3, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].ID != 5 {
		t.Errorf("jobs[0].ID = %d, want 5", jobs[0].ID)
	}
}

func TestFetchProjectJobsSince_StopsAtBoundary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("paged fake script is unix-only")
	}
	dir := t.TempDir()
	argsPath := filepath.Join(dir, "args")

	// A full page (matches per_page) with a mix of IDs above and below the
	// boundary, so fetching should stop mid-page without a page 2 call.
	page1 := `[{"id":5,"name":"c","status":"success"},{"id":4,"name":"b","status":"success"},{"id":3,"name":"a","status":"success"}]`
	script := fakeGlabScriptPagedWithArgs(t, dir, argsPath, map[int]string{1: page1})

	c := &Client{BinaryPath: script}
	jobs, err := c.FetchProjectJobsSince(context.Background(), 42, 3, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs (IDs 5 and 4 are after the boundary), got %d", len(jobs))
	}

	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("reading args: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(args), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected only 1 page fetched, got %d calls: %v", len(lines), lines)
	}
}

func TestFetchProjectJobsSince_RunError(t *testing.T) {
	c := &Client{BinaryPath: "/nonexistent/glab"}
	_, err := c.FetchProjectJobsSince(context.Background(), 42, 0, 25)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFetchProjectJobsSince_ParseError(t *testing.T) {
	dir := t.TempDir()
	script := fakeGlabScript(t, dir, "not json")

	c := &Client{BinaryPath: script}
	_, err := c.FetchProjectJobsSince(context.Background(), 42, 0, 25)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestFetchJob(t *testing.T) {
	dir := t.TempDir()
	argsPath := filepath.Join(dir, "args")
	response := `{"id":7,"name":"deploy","status":"running","web_url":"https://gitlab.com/j/7"}`
	script := fakeGlabScriptWithArgs(t, dir, argsPath, response)

	c := &Client{BinaryPath: script}
	job, err := c.FetchJob(context.Background(), 42, 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if job.ID != 7 {
		t.Errorf("ID = %d, want 7", job.ID)
	}
	if job.Status != "running" {
		t.Errorf("Status = %q, want \"running\"", job.Status)
	}
	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("reading args: %v", err)
	}
	want := "api projects/42/jobs/7"
	if string(args) != want {
		t.Errorf("args = %q, want %q", string(args), want)
	}
}

func TestFetchJob_RunError(t *testing.T) {
	c := &Client{BinaryPath: "/nonexistent/glab"}
	_, err := c.FetchJob(context.Background(), 42, 7)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFetchJob_ParseError(t *testing.T) {
	dir := t.TempDir()
	script := fakeGlabScript(t, dir, "not json")

	c := &Client{BinaryPath: script}
	_, err := c.FetchJob(context.Background(), 42, 7)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func fakeGlabScriptPagedWithArgs(t *testing.T, dir, argsPath string, pageResponses map[int]string) string {
	t.Helper()

	script := filepath.Join(dir, "glab")
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	b.WriteString("printf '%s\\n' \"$*\" >> '" + argsPath + "'\n")
	b.WriteString("case \"$2\" in\n")
	for page, resp := range pageResponses {
		b.WriteString(fmt.Sprintf("*page=%d) echo '%s' ;;\n", page, resp))
	}
	b.WriteString("*) echo '[]' ;;\nesac\n")
	if err := os.WriteFile(script, []byte(b.String()), 0o755); err != nil {
		t.Fatalf("writing fake glab script: %v", err)
	}
	return script
}

func TestFetchMergeRequestByBranch(t *testing.T) {
	dir := t.TempDir()
	response := `[{"iid":5,"web_url":"https://gitlab.com/mr/5","state":"merged","title":"Fix bug"}]`
	script := fakeGlabScript(t, dir, response)

	c := &Client{BinaryPath: script}
	mr, err := c.FetchMergeRequestByBranch(context.Background(), 42, "feature-branch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mr.IID != 5 {
		t.Errorf("IID = %d, want 5", mr.IID)
	}
	if mr.WebURL != "https://gitlab.com/mr/5" {
		t.Errorf("WebURL = %q", mr.WebURL)
	}
}

func TestFetchMergeRequestByBranch_Empty(t *testing.T) {
	dir := t.TempDir()
	script := fakeGlabScript(t, dir, "[]")

	c := &Client{BinaryPath: script}
	mr, err := c.FetchMergeRequestByBranch(context.Background(), 42, "no-mr-branch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mr.IID != 0 {
		t.Errorf("expected zero MR, got IID=%d", mr.IID)
	}
}

func TestFetchMergeRequestByBranch_ParseError(t *testing.T) {
	dir := t.TempDir()
	script := fakeGlabScript(t, dir, "not json")

	c := &Client{BinaryPath: script}
	_, err := c.FetchMergeRequestByBranch(context.Background(), 42, "branch")
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestFetchMergeRequestByBranch_RunError(t *testing.T) {
	c := &Client{BinaryPath: "/nonexistent/glab"}
	_, err := c.FetchMergeRequestByBranch(context.Background(), 42, "branch")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFetchUserMergeRequests(t *testing.T) {
	dir := t.TempDir()
	response := `[{"iid":1,"project_id":42,"web_url":"https://gitlab.com/mr/1","state":"opened","title":"Feature"}]`
	script := fakeGlabScript(t, dir, response)

	c := &Client{BinaryPath: script}
	mrs, err := c.FetchUserMergeRequests(context.Background(), time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mrs) != 2 {
		t.Fatalf("expected 2 MRs (1 opened + 1 merged from same script), got %d", len(mrs))
	}
	if mrs[0].ProjectID != 42 {
		t.Errorf("ProjectID = %d, want 42", mrs[0].ProjectID)
	}
}

func TestFetchUserMergeRequests_RunError(t *testing.T) {
	c := &Client{BinaryPath: "/nonexistent/glab"}
	_, err := c.FetchUserMergeRequests(context.Background(), time.Now().Add(-24*time.Hour))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFetchUserMergeRequests_ParseError(t *testing.T) {
	dir := t.TempDir()
	script := fakeGlabScript(t, dir, "not json")

	c := &Client{BinaryPath: script}
	_, err := c.FetchUserMergeRequests(context.Background(), time.Now().Add(-24*time.Hour))
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestFetchPipelines(t *testing.T) {
	dir := t.TempDir()
	argsPath := filepath.Join(dir, "args")
	script := fakeGlabScriptWithArgs(t, dir, argsPath, `[{"id":1,"status":"running"}]`)

	c := &Client{BinaryPath: script}
	pipelines, err := c.FetchPipelines(context.Background(), 42, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pipelines) != 1 {
		t.Fatalf("expected 1 pipeline, got %d", len(pipelines))
	}
	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("reading args: %v", err)
	}
	want := "api projects/42/pipelines?order_by=id&sort=desc&per_page=25&page=1"
	if string(args) != want {
		t.Errorf("args = %q, want %q", string(args), want)
	}
}

func TestFetchPipelinesByUser_NoUsername(t *testing.T) {
	dir := t.TempDir()
	response := `[{"id":1,"status":"running"}]`
	script := fakeGlabScript(t, dir, response)

	c := &Client{BinaryPath: script}
	pipelines, err := c.FetchPipelinesByUser(context.Background(), 42, "", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pipelines) != 1 {
		t.Fatalf("expected 1 pipeline, got %d", len(pipelines))
	}
}

func fakeGlabScriptWithArgs(t *testing.T, dir string, argsPath string, response string) string {
	t.Helper()

	var script string
	if runtime.GOOS == "windows" {
		script = filepath.Join(dir, "glab.bat")
		os.WriteFile(script, []byte("@echo off\necho %* > "+argsPath+"\necho "+response+"\n"), 0o755)
	} else {
		script = filepath.Join(dir, "glab")
		os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s' \"$*\" > '"+argsPath+"'\necho '"+response+"'\n"), 0o755)
	}
	return script
}

func fakeGlabScript(t *testing.T, dir string, response string) string {
	t.Helper()

	respBytes, _ := json.Marshal(response)
	_ = respBytes

	var script string
	if runtime.GOOS == "windows" {
		script = filepath.Join(dir, "glab.bat")
		os.WriteFile(script, []byte("@echo off\necho "+response+"\n"), 0o755)
	} else {
		script = filepath.Join(dir, "glab")
		os.WriteFile(script, []byte("#!/bin/sh\necho '"+response+"'\n"), 0o755)
	}
	return script
}
