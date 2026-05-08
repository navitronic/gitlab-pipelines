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
	_, err := c.FetchPipelinesByUser(context.Background(), 42, 1, time.Now().Add(-24*time.Hour))
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

func TestFetchPipelinesByUser_NoUserID(t *testing.T) {
	dir := t.TempDir()
	response := `[{"id":1,"status":"running"}]`
	script := fakeGlabScript(t, dir, response)

	c := &Client{BinaryPath: script}
	pipelines, err := c.FetchPipelinesByUser(context.Background(), 42, 0, time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pipelines) != 1 {
		t.Fatalf("expected 1 pipeline, got %d", len(pipelines))
	}
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
