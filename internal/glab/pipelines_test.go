package glab

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
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
	_, err := c.FetchPipelinesByUser(context.Background(), 42, 1)
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
