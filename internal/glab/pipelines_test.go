package glab

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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
